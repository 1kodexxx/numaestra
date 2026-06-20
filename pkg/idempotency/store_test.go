package idempotency

import (
	"context"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// memStore — in-memory реализация Storer для unit-тестов (без Redis).
type memStore struct {
	mu   sync.Mutex
	data map[string]Entry
	set  map[string]struct{} // занятые ключи (sentinel)
}

func newMemStore() *memStore {
	return &memStore{
		data: make(map[string]Entry),
		set:  make(map[string]struct{}),
	}
}

func (m *memStore) Get(_ context.Context, key string) (*Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, inProgress := m.set[key]; inProgress {
		e := Entry{StatusCode: 0}
		return &e, nil
	}
	e, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return &e, nil
}

func (m *memStore) Reserve(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.set[key]; exists {
		return ErrConflict
	}
	if _, exists := m.data[key]; exists {
		return ErrConflict
	}
	m.set[key] = struct{}{}
	return nil
}

func (m *memStore) Complete(_ context.Context, key string, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.set, key)
	m.data[key] = e
	return nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.set, key)
	delete(m.data, key)
	return nil
}

var _ Storer = (*memStore)(nil)

// --- тесты контракта Storer ---

func TestStorer_GetUnknownKey_ReturnsNotFound(t *testing.T) {
	s := newMemStore()
	_, err := s.Get(context.Background(), "missing")
	if err != ErrNotFound {
		t.Errorf("ожидали ErrNotFound, получили %v", err)
	}
}

func TestStorer_Reserve_ThenGetReturnsInProgress(t *testing.T) {
	s := newMemStore()
	if err := s.Reserve(context.Background(), "k1"); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	e, err := s.Get(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Get after Reserve: %v", err)
	}
	if e.StatusCode != 0 {
		t.Errorf("in-progress sentinel должен иметь StatusCode=0, получили %d", e.StatusCode)
	}
}

func TestStorer_Reserve_DuplicateReturnsConflict(t *testing.T) {
	s := newMemStore()
	_ = s.Reserve(context.Background(), "k2")
	if err := s.Reserve(context.Background(), "k2"); err != ErrConflict {
		t.Errorf("ожидали ErrConflict при повторном Reserve, получили %v", err)
	}
}

func TestStorer_Complete_StoresEntryAndClearsSentinel(t *testing.T) {
	s := newMemStore()
	_ = s.Reserve(context.Background(), "k3")

	entry := Entry{StatusCode: 201, Body: []byte(`{"id":"abc"}`)}
	if err := s.Complete(context.Background(), "k3", entry); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := s.Get(context.Background(), "k3")
	if err != nil {
		t.Fatalf("Get after Complete: %v", err)
	}
	if got.StatusCode != 201 {
		t.Errorf("ожидали StatusCode=201, получили %d", got.StatusCode)
	}
	if string(got.Body) != `{"id":"abc"}` {
		t.Errorf("ожидали тело %q, получили %q", `{"id":"abc"}`, string(got.Body))
	}
}

func TestStorer_Complete_BlocksNewReserve(t *testing.T) {
	s := newMemStore()
	_ = s.Reserve(context.Background(), "k4")
	_ = s.Complete(context.Background(), "k4", Entry{StatusCode: 200})

	if err := s.Reserve(context.Background(), "k4"); err != ErrConflict {
		t.Errorf("повторный Reserve после Complete должен вернуть ErrConflict, получили %v", err)
	}
}

func TestStorer_Delete_RemovesCompletedEntry(t *testing.T) {
	s := newMemStore()
	_ = s.Reserve(context.Background(), "k5")
	_ = s.Complete(context.Background(), "k5", Entry{StatusCode: 200})

	if err := s.Delete(context.Background(), "k5"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(context.Background(), "k5"); err != ErrNotFound {
		t.Errorf("после Delete ожидали ErrNotFound, получили %v", err)
	}
}

func TestStorer_Delete_ClearsInProgressSentinel(t *testing.T) {
	s := newMemStore()
	_ = s.Reserve(context.Background(), "k6")
	_ = s.Delete(context.Background(), "k6")

	// После Delete ключ свободен — новый Reserve должен пройти.
	if err := s.Reserve(context.Background(), "k6"); err != nil {
		t.Errorf("Reserve после Delete должен пройти, получили %v", err)
	}
}

// --- Redis Store (miniredis) ---

func newRedisStore(t *testing.T) *Store {
	t.Helper()
	mini := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewStore(rdb)
}

func TestRedisStore_GetUnknown_ReturnsNotFound(t *testing.T) {
	s := newRedisStore(t)
	_, err := s.Get(context.Background(), "missing")
	if err != ErrNotFound {
		t.Errorf("ожидали ErrNotFound, получили %v", err)
	}
}

func TestRedisStore_Reserve_ThenGetReturnsSentinel(t *testing.T) {
	s := newRedisStore(t)
	if err := s.Reserve(context.Background(), "k1"); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	e, err := s.Get(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Get после Reserve: %v", err)
	}
	if e.StatusCode != 0 {
		t.Errorf("sentinel должен иметь StatusCode=0, получили %d", e.StatusCode)
	}
}

func TestRedisStore_Reserve_DuplicateReturnsConflict(t *testing.T) {
	s := newRedisStore(t)
	_ = s.Reserve(context.Background(), "k2")
	if err := s.Reserve(context.Background(), "k2"); err != ErrConflict {
		t.Errorf("ожидали ErrConflict, получили %v", err)
	}
}

func TestRedisStore_Complete_StoresAndRetrievesEntry(t *testing.T) {
	s := newRedisStore(t)
	_ = s.Reserve(context.Background(), "k3")
	entry := Entry{StatusCode: 201, Body: []byte(`{"id":"xyz"}`)}
	if err := s.Complete(context.Background(), "k3", entry); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	got, err := s.Get(context.Background(), "k3")
	if err != nil {
		t.Fatalf("Get после Complete: %v", err)
	}
	if got.StatusCode != 201 {
		t.Errorf("StatusCode: got %d, want 201", got.StatusCode)
	}
	if string(got.Body) != `{"id":"xyz"}` {
		t.Errorf("Body: got %q", string(got.Body))
	}
}

func TestRedisStore_Delete_RemovesKey(t *testing.T) {
	s := newRedisStore(t)
	_ = s.Reserve(context.Background(), "k4")
	_ = s.Complete(context.Background(), "k4", Entry{StatusCode: 200})
	if err := s.Delete(context.Background(), "k4"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(context.Background(), "k4"); err != ErrNotFound {
		t.Errorf("после Delete ожидали ErrNotFound, получили %v", err)
	}
}

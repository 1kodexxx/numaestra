package apphttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/numaestra/numaestra/pkg/idempotency"
)

// fakeStore — in-memory реализация Storer для тестов (не требует Redis).
type fakeStore struct {
	mu      sync.Mutex
	entries map[string]*idempotency.Entry
	getErr  error
	resErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{entries: make(map[string]*idempotency.Entry)}
}

func (s *fakeStore) Get(_ context.Context, key string) (*idempotency.Entry, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if !ok {
		return nil, idempotency.ErrNotFound
	}
	return e, nil
}

func (s *fakeStore) Reserve(_ context.Context, key string) error {
	if s.resErr != nil {
		return s.resErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[key]; ok {
		return idempotency.ErrConflict
	}
	s.entries[key] = &idempotency.Entry{} // sentinel: StatusCode == 0
	return nil
}

func (s *fakeStore) Complete(_ context.Context, key string, e idempotency.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = &e
	return nil
}

func (s *fakeStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

var _ idempotency.Storer = (*fakeStore)(nil)

// handler201 — простой обработчик, возвращающий 201 с JSON-телом.
func handler201(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"id":"abc"}`)) //nolint:errcheck
}

func applyMiddleware(store idempotency.Storer, h http.HandlerFunc) http.Handler {
	return idempotencyMiddleware(store)(http.HandlerFunc(h))
}

func TestIdempotency_NoHeader_PassThrough(t *testing.T) {
	store := newFakeStore()
	h := applyMiddleware(store, handler201)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d", w.Code)
	}
	// Ключ не должен был быть сохранён.
	if len(store.entries) != 0 {
		t.Error("стор должен оставаться пустым при отсутствии заголовка")
	}
}

func TestIdempotency_FirstRequest_CachesResponse(t *testing.T) {
	store := newFakeStore()
	h := applyMiddleware(store, handler201)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Idempotency-Key", "key-1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("первый запрос: ожидали 201, получили %d", w.Code)
	}
	if w.Header().Get("X-Idempotent-Replayed") != "" {
		t.Error("первый запрос не должен иметь заголовок X-Idempotent-Replayed")
	}
	// Ответ должен быть закешированн.
	e, err := store.Get(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("ожидали запись в сторе: %v", err)
	}
	if e.StatusCode != http.StatusCreated {
		t.Errorf("ожидали StatusCode=201, получили %d", e.StatusCode)
	}
}

func TestIdempotency_SecondRequest_Replays(t *testing.T) {
	store := newFakeStore()
	h := applyMiddleware(store, handler201)

	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Idempotency-Key", "key-2")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if w.Code != http.StatusCreated {
			t.Fatalf("запрос %d: ожидали 201, получили %d", i+1, w.Code)
		}
		body, _ := io.ReadAll(w.Body)
		if !strings.Contains(string(body), "abc") {
			t.Errorf("запрос %d: ожидали тело с 'abc', получили %q", i+1, string(body))
		}
	}

	// Второй запрос должен иметь заголовок воспроизведения.
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Idempotency-Key", "key-2")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Header().Get("X-Idempotent-Replayed") != "true" {
		t.Error("повторный запрос должен иметь X-Idempotent-Replayed: true")
	}
}

func TestIdempotency_InProgress_Returns409(t *testing.T) {
	store := newFakeStore()
	// Помещаем sentinel вручную (StatusCode == 0).
	store.entries["key-3"] = &idempotency.Entry{}

	h := applyMiddleware(store, handler201)
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Idempotency-Key", "key-3")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("ожидали 409 Conflict, получили %d", w.Code)
	}
}

func TestIdempotency_NonSuccessResponse_ClearsKey(t *testing.T) {
	store := newFakeStore()
	handler400 := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad"}`)) //nolint:errcheck
	})
	h := applyMiddleware(store, handler400)

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Idempotency-Key", "key-4")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
	if _, err := store.Get(context.Background(), "key-4"); err != idempotency.ErrNotFound {
		t.Error("ключ должен быть удалён после не-2xx ответа")
	}
}

func TestIdempotency_RedisDown_FailOpen(t *testing.T) {
	store := newFakeStore()
	store.getErr = io.EOF // имитация недоступности Redis

	h := applyMiddleware(store, handler201)
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Idempotency-Key", "key-5")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	// Должен пройти как обычный запрос.
	if w.Code != http.StatusCreated {
		t.Fatalf("fail-open: ожидали 201, получили %d", w.Code)
	}
}

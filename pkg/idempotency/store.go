package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ttlInProgress = 5 * time.Minute
	ttlCompleted  = 24 * time.Hour
	keyPrefix     = "idempotency:"
)

var (
	ErrNotFound = errors.New("idempotency: ключ не найден")
	ErrConflict = errors.New("idempotency: запрос с этим ключом уже выполняется")
)

// Entry хранит закешированный успешный ответ.
// StatusCode == 0 означает, что запрос ещё в процессе выполнения (sentinel).
type Entry struct {
	StatusCode int    `json:"status_code"`
	Body       []byte `json:"body"`
}

// Storer — порт идемпотентности; позволяет подменить реализацию в тестах.
type Storer interface {
	Get(ctx context.Context, key string) (*Entry, error)
	Reserve(ctx context.Context, key string) error
	Complete(ctx context.Context, key string, e Entry) error
	Delete(ctx context.Context, key string) error
}

// Store — Redis-реализация Storer.
type Store struct {
	rdb *redis.Client
}

func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func (s *Store) Get(ctx context.Context, key string) (*Entry, error) {
	val, err := s.rdb.Get(ctx, keyPrefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(val, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Reserve атомарно занимает ключ (SET NX).
// Возвращает ErrConflict если ключ уже занят.
func (s *Store) Reserve(ctx context.Context, key string) error {
	ok, err := s.rdb.SetNX(ctx, keyPrefix+key, []byte("{}"), ttlInProgress).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrConflict
	}
	return nil
}

// Complete перезаписывает sentinel результатом и устанавливает TTL 24h.
func (s *Store) Complete(ctx context.Context, key string, e Entry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyPrefix+key, b, ttlCompleted).Err()
}

func (s *Store) Delete(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, keyPrefix+key).Err()
}

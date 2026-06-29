package s3

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

// ResilientClient оборачивает *Client слоями retry + circuit breaker.
//
// Retry: до 3 попыток с экспоненциальной выдержкой (1s → 2s).
// UploadFromURL идемпотентен (PUT по фиксированному ключу), поэтому retry безопасен.
//
// Circuit Breaker: 3 последовательных провала (включая исчерпанные ретраи)
// → размыкание на 60 секунд. S3-outage длится дольше Suno-сбоя, поэтому timeout шире.
type ResilientClient struct {
	inner   *Client
	breaker *circuitbreaker.Breaker
}

// NewResilientClient создаёт S3-клиент с retry (3 попытки, exp backoff) и
// circuit breaker (3 ошибки → 60s open).
func NewResilientClient(endpoint, region, bucket, accessKey, secretKey string) *ResilientClient {
	return &ResilientClient{
		inner:   New(endpoint, region, bucket, accessKey, secretKey),
		breaker: circuitbreaker.New("s3", 3, 60*time.Second),
	}
}

// WithPublicBaseURL переопределяет базу публичных ссылок на CDN-домен (см.
// Client.WithPublicBaseURL). Пустая строка оставляет дефолт. Чейнинг.
func (r *ResilientClient) WithPublicBaseURL(base string) *ResilientClient {
	r.inner.WithPublicBaseURL(base)
	return r
}

// WithPresign включает выдачу временных подписанных ссылок (см. Client.WithPresign).
// Чейнинг.
func (r *ResilientClient) WithPresign(enabled bool) *ResilientClient {
	r.inner.WithPresign(enabled)
	return r
}

// ResolvePlayURL подписывает ссылку на трек для клиента (см. Client.ResolvePlayURL).
// Подпись — локальная операция (без сети), поэтому retry/circuit breaker не нужны.
func (r *ResilientClient) ResolvePlayURL(ctx context.Context, storedURL string, ttl time.Duration) (string, error) {
	return r.inner.ResolvePlayURL(ctx, storedURL, ttl)
}

const maxRetries = 3

func (r *ResilientClient) UploadFromURL(ctx context.Context, sourceURL, key, contentType string) (string, error) {
	var publicURL string
	err := r.breaker.Do(func() error {
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				delay := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			var err error
			publicURL, err = r.inner.UploadFromURL(ctx, sourceURL, key, contentType)
			if err == nil {
				return nil
			}
			lastErr = err
		}
		return lastErr
	})
	return publicURL, err
}

// DeleteOrderTracks удаляет MP3-объекты заказа из S3.
func (r *ResilientClient) DeleteOrderTracks(ctx context.Context, orderID uuid.UUID) error {
	return r.breaker.Do(func() error {
		return r.inner.DeleteOrderTracks(ctx, orderID)
	})
}

// DeleteByURL удаляет объект хранилища по его публичной ссылке.
func (r *ResilientClient) DeleteByURL(ctx context.Context, publicURL string) error {
	return r.breaker.Do(func() error {
		return r.inner.DeleteByURL(ctx, publicURL)
	})
}

// Upload загружает готовые байты под фиксированным ключом (идемпотентно),
// поэтому retry безопасен — те же слои устойчивости, что и у UploadFromURL.
func (r *ResilientClient) Upload(ctx context.Context, key, contentType string, data []byte) (string, error) {
	var publicURL string
	err := r.breaker.Do(func() error {
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				delay := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			var err error
			publicURL, err = r.inner.Upload(ctx, key, contentType, data)
			if err == nil {
				return nil
			}
			lastErr = err
		}
		return lastErr
	})
	return publicURL, err
}

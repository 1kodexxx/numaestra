package s3

import (
	"context"
	"time"

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

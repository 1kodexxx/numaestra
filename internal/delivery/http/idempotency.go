package apphttp

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/numaestra/numaestra/pkg/idempotency"
)

// idempotencyMiddleware реализует семантику Idempotency-Key для POST-эндпоинтов.
//
// Поведение:
//   - Заголовок отсутствует → пропускаем без изменений.
//   - Ключ в кеше с StatusCode > 0 (завершён) → воспроизводим ответ + X-Idempotent-Replayed: true.
//   - Ключ в кеше с StatusCode == 0 (выполняется) → 409 Conflict.
//   - Ключ не найден → резервируем, выполняем, кешируем 2xx на 24h.
//   - Redis недоступен → fail open (пропускаем запрос как обычный).
func idempotencyMiddleware(store idempotency.Storer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			entry, err := store.Get(ctx, key)
			if err == nil {
				if entry.StatusCode > 0 {
					// Воспроизводим закешированный ответ.
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Idempotent-Replayed", "true")
					w.WriteHeader(entry.StatusCode)
					w.Write(entry.Body) //nolint:errcheck
					return
				}
				// StatusCode == 0: запрос ещё выполняется (sentinel).
				respondError(w, r, http.StatusConflict, "запрос с таким Idempotency-Key уже выполняется")
				return
			}

			if err != idempotency.ErrNotFound {
				// Redis недоступен — fail open.
				next.ServeHTTP(w, r)
				return
			}

			// Ключ не найден — атомарно резервируем.
			if rerr := store.Reserve(ctx, key); rerr != nil {
				if rerr == idempotency.ErrConflict {
					respondError(w, r, http.StatusConflict, "запрос с таким Idempotency-Key уже выполняется")
					return
				}
				// Redis недоступен — fail open.
				next.ServeHTTP(w, r)
				return
			}

			// Захватываем ответ.
			rec := &responseRecorder{ResponseWriter: w, buf: &bytes.Buffer{}}
			next.ServeHTTP(rec, r)

			// Если WriteHeader не вызывался явно — ответ 200.
			if rec.status == 0 {
				rec.status = http.StatusOK
			}

			if rec.status >= 200 && rec.status < 300 {
				_ = store.Complete(ctx, key, idempotency.Entry{
					StatusCode: rec.status,
					Body:       rec.buf.Bytes(),
				})
			} else {
				_ = store.Delete(ctx, key)
			}
		})
	}
}

// responseRecorder перехватывает статус и тело ответа для последующего кеширования.
type responseRecorder struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	rr.buf.Write(b)
	return rr.ResponseWriter.Write(b)
}

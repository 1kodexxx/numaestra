// Package metrics регистрирует Prometheus-метрики приложения и предоставляет
// HTTP-хендлер для эндпоинта /metrics.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// OrdersCreated — счётчик созданных заказов.
	OrdersCreated = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "numaestra",
		Name:      "orders_created_total",
		Help:      "Общее число созданных заказов.",
	})

	// OrdersCompleted — счётчик успешно завершённых заказов (треки готовы).
	OrdersCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "numaestra",
		Name:      "orders_completed_total",
		Help:      "Общее число заказов, по которым треки успешно сгенерированы.",
	})

	// OrdersFailed — счётчик заказов, упавших после всех ретраев.
	OrdersFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "numaestra",
		Name:      "orders_failed_total",
		Help:      "Общее число заказов, завершившихся ошибкой после исчерпания ретраев.",
	})

	// PaymentsReceived — счётчик успешно подтверждённых оплат.
	PaymentsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "numaestra",
		Name:      "payments_received_total",
		Help:      "Общее число успешных вебхуков оплаты от Robokassa.",
	})

	// SunoAPIErrors — счётчик ошибок при вызове Suno API.
	SunoAPIErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "numaestra",
		Name:      "suno_api_errors_total",
		Help:      "Общее число ошибок при вызове Suno API.",
	})

	// LLMErrors — счётчик ошибок при вызове LLM (OpenRouter/OpenAI).
	LLMErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "numaestra",
		Name:      "llm_errors_total",
		Help:      "Общее число ошибок при вызове LLM для генерации текста.",
	})

	// ActiveWorkerSlots — gauge числа занятых слотов Suno-аккаунтов.
	ActiveWorkerSlots = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "numaestra",
		Name:      "active_worker_slots",
		Help:      "Текущее число занятых слотов генерации (concurrent_tasks по всем аккаунтам).",
	})

	// HTTPRequestDuration — гистограмма времени обработки HTTP-запросов.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "numaestra",
		Name:      "http_request_duration_seconds",
		Help:      "Время обработки HTTP-запросов.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
)

// Handler возвращает стандартный promhttp.Handler для монтирования на /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}

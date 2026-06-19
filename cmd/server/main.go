package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/numaestra/numaestra/internal/config"
	apphttp "github.com/numaestra/numaestra/internal/delivery/http"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/repository/postgres"
	"github.com/numaestra/numaestra/internal/repository/queue"
	sunorepo "github.com/numaestra/numaestra/internal/repository/suno"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/internal/worker"
	"github.com/numaestra/numaestra/migrations"
	"github.com/numaestra/numaestra/pkg/banner"
	"github.com/numaestra/numaestra/pkg/health"
	"github.com/numaestra/numaestra/pkg/logger"
	"github.com/numaestra/numaestra/pkg/migrate"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/openai"
	"github.com/numaestra/numaestra/pkg/robokassa"
	"github.com/numaestra/numaestra/pkg/s3"
	"github.com/numaestra/numaestra/pkg/suno"
)

// Проверка на этапе компиляции, что S3-клиент реализует порт хранилища треков.
var _ domain.TrackStorage = (*s3.Client)(nil)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(rootCtx); err != nil {
		fmt.Fprintln(os.Stderr, "фатальная ошибка запуска приложения:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// 1. Конфигурация.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("загрузка конфигурации: %w", err)
	}

	banner.Print(os.Stdout, "0.1.0", cfg.Env)

	// 2. Логгер.
	log := logger.New(cfg.Env)
	log.Info("Запуск сервиса Numaestra", "env", cfg.Env, "http_port", cfg.HTTP.Port)

	// 3. Инфраструктурные зависимости.
	pgPool, err := pgxpool.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("инициализация пула соединений postgres: %w", err)
	}
	defer pgPool.Close()

	if err := pgPool.Ping(ctx); err != nil {
		return fmt.Errorf("проверка соединения с postgres: %w", err)
	}
	log.Info("соединение с postgres установлено")

	if err := migrate.Run(ctx, pgPool, migrations.FS, log); err != nil {
		return fmt.Errorf("применение миграций: %w", err)
	}
	log.Info("миграции применены")

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	}

	asynqClient := asynq.NewClient(redisOpt)
	defer func() {
		if cerr := asynqClient.Close(); cerr != nil {
			log.Error("ошибка закрытия клиента asynq", "error", cerr)
		}
	}()

	// 4. Dependency Injection.
	accountRepo := postgres.NewAccountRepository(pgPool)
	orderRepo := postgres.NewOrderRepository(pgPool)
	queuePublisher := queue.NewAsynqPublisher(asynqClient)

	sunoClient := suno.NewClient(cfg.Suno.APIURL, cfg.Suno.APIKey)
	musicProvider := sunorepo.NewProviderAdapter(sunoClient)

	llmClient := openai.NewClient(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey)

	s3Client := s3.New(cfg.S3.Endpoint, cfg.S3.Region, cfg.S3.Bucket, cfg.S3.AccessKey, cfg.S3.SecretKey)

	// Notifier: заглушка-логгер до подключения реального SMTP/SMS-провайдера.
	// Чтобы подключить реальный провайдер — реализуйте notify.Notifier
	// и передайте сюда вместо NewLogNotifier.
	notifier := notify.NewLogNotifier(log)

	// Прайс определяется сервером по тарифу: цена не принимается из запроса клиента.
	pricing := usecase.NewStaticPricing(cfg.Pricing.Plans, cfg.Pricing.DefaultPlan)

	orderUC := usecase.NewOrderUseCase(orderRepo, accountRepo, queuePublisher, musicProvider, s3Client, notifier, llmClient, pricing, log)

	// 5. Asynq Worker.
	processor := worker.NewOrderProcessor(orderUC, log)

	asynqServer := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"generation": 5,
				"polling":    5,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				if errors.Is(e, usecase.ErrGenerationNotReady) {
					return 15 * time.Second
				}
				return asynq.DefaultRetryDelayFunc(n, e, t)
			},
			// Терминальный обработчик: при исчерпании всех ретраев переводим заказ
			// в failed и освобождаем аккаунт, иначе он застрянет в Busy навсегда.
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
				retried, _ := asynq.GetRetryCount(ctx)
				maxRetry, _ := asynq.GetMaxRetry(ctx)
				if retried < maxRetry {
					return // ещё будут ретраи — ждём
				}
				processor.HandleDeadTask(ctx, t, err)
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TaskTypeGenerateTrack, processor.HandleGenerateTask)
	mux.HandleFunc(queue.TaskTypeCheckStatus, processor.HandleStatusCheckTask)

	go func() {
		log.Info("запуск Asynq worker-сервера")
		if err := asynqServer.Run(mux); err != nil {
			log.Error("ошибка работы Asynq worker", "error", err)
		}
	}()
	defer asynqServer.Stop()

	// 6. HTTP-хендлеры и роутер.
	rkClient := robokassa.New(cfg.Robokassa.MerchantLogin, cfg.Robokassa.Password1, cfg.Robokassa.Password2, cfg.Robokassa.IsTest)
	orderHandler := apphttp.NewOrderHandler(orderUC, log, rkClient)
	healthChecker := health.New(pgPool, redisOpt)
	router := newRouter(log, orderHandler, healthChecker, cfg.HTTP)

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 7. Запуск.
	serveErrCh := make(chan error, 1)
	go func() {
		log.Info("http-сервер слушает", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
			return
		}
		serveErrCh <- nil
	}()

	select {
	case err := <-serveErrCh:
		if err != nil {
			return fmt.Errorf("работа http-сервера завершилась с ошибкой: %w", err)
		}
	case <-ctx.Done():
		log.Info("получен сигнал завершения, начинаем graceful shutdown")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("ошибка при остановке http-сервера", "error", err)
	}

	log.Info("сервис Numaestra остановлен корректно")
	return nil
}

func newRouter(log *slog.Logger, orderHandler *apphttp.OrderHandler, checker *health.Checker, httpCfg config.HTTPConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(requestLoggerMiddleware(log))
	// Ограничиваем размер тела запроса, чтобы защититься от исчерпания памяти.
	r.Use(apphttp.MaxBodyBytes(httpCfg.MaxBodyBytes))
	// Список разрешённых Origin берётся из конфигурации (CORS_ALLOWED_ORIGINS).
	// Пустой список означает "*" — допустимо для dev, в проде стоит ограничить.
	r.Use(apphttp.CORS(apphttp.DefaultCORSOptions(httpCfg.CORSAllowedOrigins)))

	r.Get("/healthz", checker.Handler)

	// Rate limiting вынесен внутрь orderHandler.Routes(): создание заказа и
	// защищённые маршруты ограничиваются клиентским лимитером, а вебхук Robokassa —
	// отдельным независимым бакетом, чтобы клиентский трафик не вызывал у него 429.
	r.Mount("/api/v1/orders", orderHandler.Routes())

	return r
}

func requestLoggerMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// Оборачиваем ResponseWriter, чтобы зафиксировать статус ответа и размер.
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("http запрос обработан",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"request_id", chimiddleware.GetReqID(r.Context()),
				"duration", time.Since(start).String(),
			)
		})
	}
}

package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/redis/go-redis/v9"

	"github.com/numaestra/numaestra/internal/config"
	apphttp "github.com/numaestra/numaestra/internal/delivery/http"
	"github.com/numaestra/numaestra/pkg/encryption"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/repository/postgres"
	"github.com/numaestra/numaestra/internal/repository/queue"
	sunorepo "github.com/numaestra/numaestra/internal/repository/suno"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/internal/worker"
	"github.com/numaestra/numaestra/migrations"
	"github.com/numaestra/numaestra/pkg/banner"
	"github.com/numaestra/numaestra/pkg/health"
	"github.com/numaestra/numaestra/pkg/idempotency"
	"github.com/numaestra/numaestra/pkg/logger"
	pkgmetrics "github.com/numaestra/numaestra/pkg/metrics"
	"github.com/numaestra/numaestra/pkg/migrate"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/openai"
	"github.com/numaestra/numaestra/pkg/robokassa"
	"github.com/numaestra/numaestra/pkg/s3"
	"github.com/numaestra/numaestra/pkg/suno"
)

// runMode перечисляет допустимые значения APP_MODE.
type runMode string

const (
	modeAll    runMode = "all"
	modeAPI    runMode = "api"
	modeWorker runMode = "worker"
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
	log.Info("Запуск сервиса Numaestra", "env", cfg.Env, "mode", cfg.Mode, "http_port", cfg.HTTP.Port)

	// 3a. Шифр для поля encrypted_session (AES-256-GCM).
	sessionCipher, err := buildSessionCipher(cfg.SessionEncryptionKey, cfg.Env, log)
	if err != nil {
		return fmt.Errorf("инициализация шифра сессий: %w", err)
	}

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
	accountRepo := postgres.NewAccountRepository(pgPool, sessionCipher)
	orderRepo := postgres.NewOrderRepository(pgPool)
	txManager := postgres.NewTxManager(pgPool)
	queuePublisher := queue.NewAsynqPublisher(asynqClient)

	// === НОВЫЙ БЛОК ДЛЯ UI (Server-Driven UI) ===
	categoryRepo := postgres.NewCategoryRepository(pgPool)
	promptUC := usecase.NewPromptUseCase(categoryRepo)
	// ============================================

	sunoClient := suno.NewClientWithBreaker(cfg.Suno.APIURL, cfg.Suno.APIKey)
	musicProvider := sunorepo.NewProviderAdapter(sunoClient)

	llmClient := openai.NewClientWithBreaker(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey)

	s3Client := s3.New(cfg.S3.Endpoint, cfg.S3.Region, cfg.S3.Bucket, cfg.S3.AccessKey, cfg.S3.SecretKey)

	// Notifier: SMTP если SMTP_HOST задан, иначе заглушка-логгер.
	var notifier notify.Notifier
	if cfg.Notify.SMTPHost != "" {
		notifier = notify.NewSmtpNotifier(
			cfg.Notify.SMTPHost,
			cfg.Notify.SMTPPort,
			cfg.Notify.SMTPUser,
			cfg.Notify.SMTPPassword,
			cfg.Notify.FromAddress,
			cfg.Notify.FromName,
		)
		log.Info("SMTP-нотификатор активен", "host", cfg.Notify.SMTPHost, "port", cfg.Notify.SMTPPort)
	} else {
		notifier = notify.NewLogNotifier(log)
		log.Warn("SMTP_HOST не задан — уведомления только в лог (заглушка)")
	}

	// Прайс определяется сервером по тарифу: цена не принимается из запроса клиента.
	pricing := usecase.NewStaticPricing(cfg.Pricing.Plans, cfg.Pricing.DefaultPlan)

	orderUC := usecase.NewOrderUseCase(orderRepo, accountRepo, queuePublisher, musicProvider, s3Client, notifier, llmClient, promptUC, pricing, txManager, log)

	mode := runMode(cfg.Mode)

	// 5. Asynq Worker — запускается в режимах "all" и "worker".
	if mode == modeAll || mode == modeWorker {
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
				ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
					retried, _ := asynq.GetRetryCount(ctx)
					maxRetry, _ := asynq.GetMaxRetry(ctx)
					if retried < maxRetry {
						return
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

		// Фоновое восстановление застрявших заказов: каждые 5 минут ищем заказы,
		// брошенные в статусе processing после краша пода, и возвращаем их в очередь.
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := orderUC.RecoverStuckOrders(ctx); err != nil {
						log.Error("ошибка восстановления застрявших заказов", "err", err)
					}
				}
			}
		}()
	}

	// В режиме "worker" HTTP-сервер не нужен — ждём сигнала и выходим.
	if mode == modeWorker {
		log.Info("режим worker: HTTP-сервер не запущен")
		<-ctx.Done()
		log.Info("сервис Numaestra остановлен корректно")
		return nil
	}

	// 6. HTTP-хендлеры и роутер — режимы "all" и "api".
	rkClient := robokassa.New(cfg.Robokassa.MerchantLogin, cfg.Robokassa.Password1, cfg.Robokassa.Password2, cfg.Robokassa.IsTest)
	webhookAllowedNets, err := apphttp.ParseCIDRs(cfg.Robokassa.AllowedIPs)
	if err != nil {
		return fmt.Errorf("разбор ROBOKASSA_ALLOWED_IPS: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	defer rdb.Close() //nolint:errcheck

	if cfg.AdminToken == "" {
		log.Warn("ADMIN_TOKEN не задан — административный API будет отклонять все запросы")
	}

	orderHandler := apphttp.NewOrderHandler(orderUC, log, rkClient, webhookAllowedNets).
		WithIdempotency(idempotency.NewStore(rdb)).
		WithRedis(rdb)
	categoryHandler := apphttp.NewCategoryHandler(promptUC, log)
	adminHandler := apphttp.NewAdminHandler(usecase.NewAdminUseCase(orderRepo, accountRepo, robokassa.NewRefunderWithBreaker(rkClient), log), log)
	metricsNets, err := apphttp.ParseCIDRs(cfg.HTTP.MetricsAllowedIPs)
	if err != nil {
		return fmt.Errorf("разбор METRICS_ALLOWED_IPS: %w", err)
	}

	healthChecker := health.New(pgPool, redisOpt)
	router := newRouter(log, orderHandler, categoryHandler, adminHandler, healthChecker, cfg, metricsNets)

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

func newRouter(
	log *slog.Logger,
	orderHandler *apphttp.OrderHandler,
	categoryHandler *apphttp.CategoryHandler,
	adminHandler *apphttp.AdminHandler,
	checker *health.Checker,
	cfg *config.Config,
	metricsNets []*net.IPNet,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(requestLoggerMiddleware(log))
	r.Use(apphttp.MaxBodyBytes(cfg.HTTP.MaxBodyBytes))
	r.Use(apphttp.CORS(apphttp.DefaultCORSOptions(cfg.HTTP.CORSAllowedOrigins)))

	r.Get("/healthz", checker.Handler)

	r.Group(func(r chi.Router) {
		r.Use(apphttp.IPAllowlist(metricsNets))
		r.Handle("/metrics", pkgmetrics.Handler())
	})

	r.Mount("/api/v1/orders", orderHandler.Routes())
	r.Mount("/api/v1/categories", categoryHandler.Routes())

	r.Group(func(r chi.Router) {
		r.Use(apphttp.AdminAuth(cfg.AdminToken))
		r.Mount("/api/v1/admin", adminHandler.Routes())
	})

	return r
}

// buildSessionCipher парсит SESSION_ENCRYPTION_KEY и возвращает AES-256-GCM шифр.
// В dev-окружении с пустым ключом использует небезопасный дев-ключ с предупреждением.
// В не-dev окружениях пустой ключ — фатальная ошибка запуска.
func buildSessionCipher(hexKey, env string, log *slog.Logger) (encryption.Cipher, error) {
	var keyBytes []byte
	if hexKey == "" {
		if env != "dev" {
			return nil, errors.New("SESSION_ENCRYPTION_KEY обязателен в не-dev окружении (64 hex-символа = 32 байта)")
		}
		log.Warn("SESSION_ENCRYPTION_KEY не задан — используется небезопасный дев-ключ. Никогда не используйте в production!")
		// Детерминированный 32-байтный дев-ключ: 0x00..0x1f
		keyBytes = make([]byte, 32)
		for i := range keyBytes {
			keyBytes[i] = byte(i)
		}
	} else {
		var err error
		keyBytes, err = hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("SESSION_ENCRYPTION_KEY должен быть hex-строкой: %w", err)
		}
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("SESSION_ENCRYPTION_KEY должен быть 64 hex-символа (32 байта), получено %d байт", len(keyBytes))
		}
	}
	return encryption.New(keyBytes)
}

func requestLoggerMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			elapsed := time.Since(start)
			// chi.RouteContext().RoutePattern() возвращает шаблон маршрута ("/api/v1/orders/{id}"),
			// что исключает взрыв кардинальности в Prometheus от реальных UUID в путях.
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			pkgmetrics.HTTPRequestDuration.
				WithLabelValues(r.Method, routePattern, strconv.Itoa(ww.Status())).
				Observe(elapsed.Seconds())
			log.Info("http запрос обработан",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"request_id", chimiddleware.GetReqID(r.Context()),
				"duration", elapsed.String(),
			)
		})
	}
}

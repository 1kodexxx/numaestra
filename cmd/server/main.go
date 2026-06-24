package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
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
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/repository/postgres"
	"github.com/numaestra/numaestra/internal/repository/queue"
	sunorepo "github.com/numaestra/numaestra/internal/repository/suno"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/internal/worker"
	"github.com/numaestra/numaestra/migrations"
	"github.com/numaestra/numaestra/pkg/banner"
	"github.com/numaestra/numaestra/pkg/encryption"
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
	"github.com/numaestra/numaestra/web"
)

// runMode перечисляет допустимые значения APP_MODE.
type runMode string

const (
	modeAll    runMode = "all"
	modeAPI    runMode = "api"
	modeWorker runMode = "worker"
)

// Проверки на этапе компиляции: оба S3-клиента реализуют порт хранилища треков.
var _ domain.TrackStorage = (*s3.Client)(nil)
var _ domain.TrackStorage = (*s3.ResilientClient)(nil)

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

	// 3b. Секрет подписи cookie-сессий админки (/admin на фронтенде).
	adminSessionSecret, err := buildAdminSessionSecret(cfg.AdminSessionSecret, cfg.Env, log)
	if err != nil {
		return fmt.Errorf("инициализация секрета сессий админки: %w", err)
	}

	// 3. Инфраструктурные зависимости.
	pgPool, err := pgxpool.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("инициализация пула соединений postgres: %w", err)
	}
	defer pgPool.Close()

	if err := pingWithRetry(ctx, pgPool, log); err != nil {
		return fmt.Errorf("postgres недоступен после всех попыток: %w", err)
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
	// redisOpt передаётся издателю для Inspector.RunTask: при конфликте TaskID
	// (задача застряла в retry с backoff) издатель немедленно активирует её.
	queuePublisher := queue.NewAsynqPublisher(asynqClient, redisOpt)

	// === НОВЫЙ БЛОК ДЛЯ UI (Server-Driven UI) ===
	categoryRepo := postgres.NewCategoryRepository(pgPool)
	promptUC := usecase.NewPromptUseCase(categoryRepo)
	// ============================================

	exampleRepo := postgres.NewExampleRepository(pgPool)
	exampleUC := usecase.NewExampleUseCase(exampleRepo, log)
	statsUC := usecase.NewStatsUseCase(orderRepo, accountRepo, categoryRepo, exampleRepo, log)

	reviewRepo := postgres.NewReviewRepository(pgPool)
	reviewUC := usecase.NewReviewUseCase(reviewRepo, log)

	sunoClient := suno.NewClientWithBreaker(cfg.Suno.APIURL, cfg.Suno.APIKey, cfg.Suno.Model)
	musicProvider := sunorepo.NewProviderAdapter(sunoClient)

	llmClient := openai.NewClientWithBreaker(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey)

	s3Client := s3.NewResilientClient(cfg.S3.Endpoint, cfg.S3.Region, cfg.S3.Bucket, cfg.S3.AccessKey, cfg.S3.SecretKey)

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
			cfg.Notify.PublicAppURL,
		)
		log.Info("SMTP-нотификатор активен", "host", cfg.Notify.SMTPHost, "port", cfg.Notify.SMTPPort, "public_url", cfg.Notify.PublicAppURL)
	} else {
		notifier = notify.NewLogNotifier(log)
		log.Warn("SMTP_HOST не задан — уведомления только в лог (заглушка)")
	}

	orderUC := usecase.NewOrderUseCase(orderRepo, accountRepo, queuePublisher, musicProvider, s3Client, notifier, llmClient, promptUC, cfg.Pricing.PriceKopecks, txManager, log)

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
					if err := orderUC.RecoverOrphanedQueuedOrders(ctx); err != nil {
						log.Error("ошибка восстановления осиротевших queued-заказов", "err", err)
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

	if err := pingRedisWithRetry(ctx, rdb, log); err != nil {
		return fmt.Errorf("redis недоступен после всех попыток: %w", err)
	}
	log.Info("соединение с redis установлено")

	if cfg.AdminToken == "" {
		log.Warn("ADMIN_TOKEN не задан — административный API (Bearer-доступ для скриптов) будет отклонять все запросы")
	}
	if cfg.AdminLogin == "" || cfg.AdminPassword == "" {
		log.Warn("ADMIN_LOGIN/ADMIN_PASSWORD не заданы — вход в /admin на фронтенде будет отклонять все запросы")
	}

	orderHandler := apphttp.NewOrderHandler(orderUC, log, rkClient, webhookAllowedNets).
		WithIdempotency(idempotency.NewStore(rdb)).
		WithRedis(rdb)
	categoryHandler := apphttp.NewCategoryHandler(promptUC, log)
	exampleHandler := apphttp.NewExampleHandler(exampleUC, log)
	reviewHandler := apphttp.NewReviewHandler(reviewUC, log)
	adminHandler := apphttp.NewAdminHandler(usecase.NewAdminUseCase(orderRepo, accountRepo, categoryRepo, robokassa.NewRefunderWithBreaker(rkClient), promptUC, notifier, log).WithQueue(queuePublisher), log).
		WithExamples(exampleUC).
		WithReviews(reviewUC).
		WithStats(statsUC)
	if cfg.S3.AccessKey != "" && cfg.S3.SecretKey != "" {
		adminHandler = adminHandler.WithCoverUploader(s3Client)
	} else {
		log.Warn("S3-ключи не заданы — загрузка обложек категорий в админке будет недоступна")
	}
	adminAuthHandler := apphttp.NewAdminAuthHandler(cfg.AdminLogin, cfg.AdminPassword, adminSessionSecret, cfg.Env != "dev", log).WithRedis(rdb)
	metricsNets, err := apphttp.ParseCIDRs(cfg.HTTP.MetricsAllowedIPs)
	if err != nil {
		return fmt.Errorf("разбор METRICS_ALLOWED_IPS: %w", err)
	}

	spaFS, err := fs.Sub(web.FS, "out")
	if err != nil {
		return fmt.Errorf("SPA filesystem: %w", err)
	}
	imagesFS, err := fs.Sub(web.FS, "images")
	if err != nil {
		return fmt.Errorf("images filesystem: %w", err)
	}

	healthChecker := health.New(pgPool, redisOpt)
	seoHandler := apphttp.NewSeoHandler(promptUC, log)

	// SEO-инъектор: вшивает в index.html серверный title/meta/OG/JSON-LD + блок
	// текста под маршрут, чтобы краулеры видели контент без исполнения JS.
	indexHTML, err := fs.ReadFile(spaFS, "index.html")
	if err != nil {
		return fmt.Errorf("чтение index.html для SEO-инъекции: %w", err)
	}
	seoInjector := apphttp.NewSEOInjector(indexHTML, promptUC, cfg.Pricing.PriceKopecks, log).
		WithReviews(reviewUC)

	router := newRouter(log, orderHandler, categoryHandler, exampleHandler, reviewHandler, adminHandler, adminAuthHandler, seoHandler, seoInjector, healthChecker, cfg, adminSessionSecret, metricsNets, spaFS, imagesFS)

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
	exampleHandler *apphttp.ExampleHandler,
	reviewHandler *apphttp.ReviewHandler,
	adminHandler *apphttp.AdminHandler,
	adminAuthHandler *apphttp.AdminAuthHandler,
	seoHandler *apphttp.SeoHandler,
	seoInjector *apphttp.SEOInjector,
	checker *health.Checker,
	cfg *config.Config,
	adminSessionSecret []byte,
	metricsNets []*net.IPNet,
	spaFS fs.FS,
	imagesFS fs.FS,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	// RealIP помечен deprecated из-за спуфинга X-Forwarded-For/X-Real-IP, но в проде
	// приложение всегда стоит за Caddy, который выставляет X-Real-IP={remote_host}
	// (см. deploy/Caddyfile), затирая любой клиентский заголовок — спуфинг исключён.
	r.Use(chimiddleware.RealIP) //nolint:staticcheck // безопасно за доверенным прокси Caddy
	r.Use(chimiddleware.Recoverer)
	r.Use(requestLoggerMiddleware(log))
	r.Use(apphttp.MaxBodyBytes(cfg.HTTP.MaxBodyBytes))
	// CORS: дефолтный wildcard "*" допустим только в dev. В проде без явного
	// CORS_ALLOWED_ORIGINS заголовки CORS не выставляются вовсе — штатно SPA
	// раздаётся с того же origin, что и API, и cross-origin доступ не нужен.
	corsOpts := apphttp.DefaultCORSOptions(cfg.HTTP.CORSAllowedOrigins)
	if cfg.Env != "dev" && len(cfg.HTTP.CORSAllowedOrigins) == 0 {
		corsOpts.AllowedOrigins = nil
	}
	r.Use(apphttp.CORS(corsOpts))
	// Защитные заголовки на все ответы; HSTS — только вне dev (за TLS).
	r.Use(apphttp.SecurityHeaders(cfg.Env != "dev"))

	r.Get("/healthz", checker.Handler)

	r.Group(func(r chi.Router) {
		r.Use(apphttp.IPAllowlist(metricsNets))
		r.Handle("/metrics", pkgmetrics.Handler())
	})

	r.Mount("/api/v1/orders", orderHandler.Routes())
	r.Mount("/api/v1/categories", categoryHandler.Routes())
	r.Mount("/api/v1/examples", exampleHandler.Routes())
	r.Mount("/api/v1/reviews", reviewHandler.Routes())

	r.Route("/api/v1/admin", func(r chi.Router) {
		// /login и /logout — публичные, регистрируем напрямую чтобы избежать
		// двойного Mount("/", ...) на одном mux (chi запрещает).
		adminAuthHandler.Register(r)

		r.Group(func(r chi.Router) {
			r.Use(apphttp.AdminAuth(cfg.AdminToken, adminSessionSecret))
			r.Get("/me", adminAuthHandler.Me)
			r.Mount("/", adminHandler.Routes())
		})
	})

	// SEO: robots.txt и sitemap.xml (абсолютные URL по хосту запроса, категории из БД).
	r.Get("/robots.txt", seoHandler.Robots)
	r.Get("/sitemap.xml", seoHandler.Sitemap)

	// Статичные обложки категорий (SVG-заглушки, встроенные через go:embed).
	r.Mount("/images/", http.StripPrefix("/images/", http.FileServer(http.FS(imagesFS))))

	// React SPA — catch-all, должен быть последним.
	// API-маршруты выше имеют приоритет по специфичности.
	r.Mount("/", apphttp.NewSPAHandler(spaFS, seoInjector))

	return r
}

// buildSessionCipher парсит SESSION_ENCRYPTION_KEY и возвращает AES-256-GCM шифр.
// В dev-окружении с пустым ключом использует небезопасный дев-ключ с предупреждением.
// В не-dev окружениях пустой ключ — фатальная ошибка запуска.
func buildSessionCipher(hexKey, env string, log *slog.Logger) (encryption.Cipher, error) {
	keyBytes, err := decodeHexKeyWithDevFallback(hexKey, env, "SESSION_ENCRYPTION_KEY", 0x00, log)
	if err != nil {
		return nil, err
	}
	return encryption.New(keyBytes)
}

// buildAdminSessionSecret парсит ADMIN_SESSION_SECRET — ключ, которым подписываются
// cookie-сессии админки (см. pkg/adminsession). Та же схема дев-фоллбэка, что и у
// buildSessionCipher, но с другим дев-ключом (XOR-соль), чтобы в dev секреты разных
// подсистем не совпадали даже при одновременно пустых переменных окружения.
func buildAdminSessionSecret(hexKey, env string, log *slog.Logger) ([]byte, error) {
	return decodeHexKeyWithDevFallback(hexKey, env, "ADMIN_SESSION_SECRET", 0xff, log)
}

// decodeHexKeyWithDevFallback декодирует 64-символьный hex-ключ (32 байта).
// В dev-окружении с пустым ключом возвращает детерминированный небезопасный
// ключ (0^salt, 1^salt, 2^salt, ...) с предупреждением в логе.
// В не-dev окружениях пустой или некорректный ключ — фатальная ошибка запуска.
func decodeHexKeyWithDevFallback(hexKey, env, envVarName string, devSalt byte, log *slog.Logger) ([]byte, error) {
	if hexKey == "" {
		if env != "dev" {
			return nil, fmt.Errorf("%s обязателен в не-dev окружении (64 hex-символа = 32 байта)", envVarName)
		}
		log.Warn(envVarName+" не задан — используется небезопасный дев-ключ. Никогда не используйте в production!",
			"env_var", envVarName)
		keyBytes := make([]byte, 32)
		for i := range keyBytes {
			keyBytes[i] = byte(i) ^ devSalt
		}
		return keyBytes, nil
	}
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("%s должен быть hex-строкой: %w", envVarName, err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("%s должен быть 64 hex-символа (32 байта), получено %d байт", envVarName, len(keyBytes))
	}
	return keyBytes, nil
}

// pingRedisWithRetry пытается достучаться до Redis с экспоненциальной выдержкой.
// Попытки: 1s → 2s → 4s → 8s → 16s (итого ≤31s до первого запроса).
func pingRedisWithRetry(ctx context.Context, rdb *redis.Client, log *slog.Logger) error {
	const maxAttempts = 5
	delay := time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := rdb.Ping(ctx).Err(); err == nil {
			return nil
		} else if attempt == maxAttempts {
			return err
		} else {
			log.Warn("redis недоступен, повтор через...",
				"attempt", attempt, "max", maxAttempts, "delay", delay)
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
		delay *= 2
	}
	return nil // unreachable
}

// pingWithRetry пытается достучаться до Postgres с экспоненциальной выдержкой.
// Попытки: 1s → 2s → 4s → 8s → 16s (итого ≤31s до первого запроса).
// Нужен при старте в k8s, когда pod поднимается раньше StatefulSet postgres.
func pingWithRetry(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	const maxAttempts = 5
	delay := time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := pool.Ping(ctx); err == nil {
			return nil
		} else if attempt == maxAttempts {
			return err
		} else {
			log.Warn("postgres недоступен, повтор через...",
				"attempt", attempt, "max", maxAttempts, "delay", delay)
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
		delay *= 2
	}
	return nil // unreachable
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

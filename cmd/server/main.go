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
	"github.com/numaestra/numaestra/internal/repository/postgres"
	"github.com/numaestra/numaestra/internal/repository/queue"
	sunorepo "github.com/numaestra/numaestra/internal/repository/suno"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/internal/worker"
	"github.com/numaestra/numaestra/migrations"
	"github.com/numaestra/numaestra/pkg/banner"
	"github.com/numaestra/numaestra/pkg/logger"
	"github.com/numaestra/numaestra/pkg/migrate"
	"github.com/numaestra/numaestra/pkg/openai"
	"github.com/numaestra/numaestra/pkg/suno"
)

func main() {
	// Корневой контекст приложения, отменяемый при получении SIGINT/SIGTERM.
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

	// 3. Инфраструктурные зависимости (База данных и Очереди).
	pgPool, err := pgxpool.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("инициализация пула соединений postgres: %w", err)
	}
	defer pgPool.Close()

	if err := pgPool.Ping(ctx); err != nil {
		return fmt.Errorf("проверка соединения с postgres: %w", err)
	}
	log.Info("соединение с postgres установлено")

	// Применяем SQL-миграции при каждом старте.
	// Уже применённые файлы пропускаются — идемпотентно и безопасно.
	if err := migrate.Run(ctx, pgPool, migrations.FS, log); err != nil {
		return fmt.Errorf("применение миграций: %w", err)
	}
	log.Info("миграции применены")

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	defer func() {
		if cerr := asynqClient.Close(); cerr != nil {
			log.Error("ошибка закрытия клиента asynq", "error", cerr)
		}
	}()

	// 4. Сборка зависимостей (Dependency Injection).
	// Репозитории
	accountRepo := postgres.NewAccountRepository(pgPool)
	orderRepo := postgres.NewOrderRepository(pgPool)
	queuePublisher := queue.NewAsynqPublisher(asynqClient)

	// Боевой клиент Suno
	sunoClient := suno.NewClient(cfg.Suno.APIURL, cfg.Suno.APIKey)
	musicProvider := sunorepo.NewProviderAdapter(sunoClient)

	// Инициализируем LLM Клиент (OpenRouter / OpenAI)
	llmClient := openai.NewClient(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey)

	// Бизнес-логика (Use-Case)
	orderUC := usecase.NewOrderUseCase(orderRepo, accountRepo, queuePublisher, musicProvider, llmClient, log) // <-- ПЕРЕДАН llmClient

	// 5. Настройка и запуск Asynq Worker (Фоновые задачи)
	asynqServer := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"generation": 5,
				"polling":    5,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				if errors.Is(e, usecase.ErrGenerationNotReady) {
					return 15 * time.Second // Поллинг каждые 15 сек, если трек еще генерируется
				}
				return asynq.DefaultRetryDelayFunc(n, e, t)
			},
		},
	)

	processor := worker.NewOrderProcessor(orderUC, log)
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

	// 6. Инициализация HTTP-хендлеров и роутера.
	orderHandler := apphttp.NewOrderHandler(orderUC, log, cfg.Robokassa) // <-- ПЕРЕДАН ПАРОЛЬ РОБОКАССЫ
	router := newRouter(log, orderHandler)

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 7. Запуск сервера в отдельной горутине.
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

	// Graceful shutdown HTTP сервера
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("ошибка при остановке http-сервера", "error", err)
	}

	log.Info("сервис Numaestra остановлен корректно")
	return nil
}

// newRouter собирает HTTP-роутер с базовыми middleware и служебными эндпоинтами.
func newRouter(log *slog.Logger, orderHandler *apphttp.OrderHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(requestLoggerMiddleware(log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Монтируем маршруты бизнес-логики
	r.Mount("/api/v1/orders", orderHandler.Routes())

	return r
}

func requestLoggerMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Info("http запрос обработан",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(start).String(),
			)
		})
	}
}

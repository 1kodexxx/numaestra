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
	"github.com/numaestra/numaestra/internal/repository/queue"
	"github.com/numaestra/numaestra/pkg/logger"
)

func main() {
	// Корневой контекст приложения, отменяемый при получении SIGINT/SIGTERM.
	// Именно его отмена запускает каскадное graceful-завершение всех компонентов.
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(rootCtx); err != nil {
		fmt.Fprintln(os.Stderr, "фатальная ошибка запуска приложения:", err)
		os.Exit(1)
	}
}

// run содержит всю логику инициализации и работы приложения.
// Вынесена из main, чтобы все defer-ы (закрытие пула БД, клиента Asynq) корректно
// отработали перед завершением процесса.
func run(ctx context.Context) error {
	// 1. Конфигурация.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("загрузка конфигурации: %w", err)
	}

	// 2. Логгер.
	log := logger.New(cfg.Env)
	log.Info("запуск сервиса Numaestra", "env", cfg.Env, "http_port", cfg.HTTP.Port)

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

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	defer func() {
		if cerr := asynqClient.Close(); cerr != nil {
			log.Error("ошибка закрытия клиента asynq", "error", cerr)
		}
	}()

	// 4. Сборка зависимостей (Dependency Injection) через конструкторы.
	// Сейчас подключены только инфраструктурные адаптеры, не зависящие от бизнес-логики.
	// По мере реализации use-case'ов сюда добавляются:
	//   accountRepo := postgres.NewAccountRepository(pgPool)
	//   orderRepo := postgres.NewOrderRepository(pgPool)
	//   orderUseCase := usecase.NewOrderUseCase(orderRepo, accountRepo, queuePublisher, log)
	//   orderHandler := httphandler.NewOrderHandler(orderUseCase)
	queuePublisher := queue.NewAsynqPublisher(asynqClient)
	_ = queuePublisher // будет передан в use-case при его реализации

	// 5. HTTP-роутер и middleware.
	router := newRouter(log)

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 6. Запуск сервера в отдельной горутине; основная горутина ждёт сигнал или ошибку.
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

	// 7. Graceful shutdown: даём активным запросам время на завершение.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown http-сервера: %w", err)
	}

	log.Info("сервис Numaestra остановлен корректно")
	return nil
}

// newRouter собирает HTTP-роутер с базовыми middleware и служебными эндпоинтами.
// Бизнес-маршруты подключаются позже через router.Mount при реализации delivery-слоя.
func newRouter(log *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(requestLoggerMiddleware(log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// TODO: r.Mount("/api/v1/orders", orderHandler.Routes())
	// TODO: r.Mount("/api/v1/payments", paymentHandler.Routes()) // вебхук Robokassa ResultURL

	return r
}

// requestLoggerMiddleware - тонкая обёртка, логирующая каждый запрос через slog.
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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server хранит общие зависимости приложения (пул БД, сервисы)
// и реализует интерфейс api.ServerInterface.
type Server struct {
	pool *pgxpool.Pool
}

// CreateTripPosition implements [api.ServerInterface].
func (s *Server) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// ListTripPositions implements [api.ServerInterface].
func (s *Server) ListTripPositions(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// Health обрабатывает запрос GET /health (Liveness-проба).
// Отвечает 200 OK до тех пор, пока процесс запущен и способен отвечать
// В противном случае клиент не дождётся ответ
// БД не проверяет
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK) //200

	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

// Ready обрабатывает запрос GET /ready
// проверяет жива ли бд
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// Если пул не инициализирован или метод Ping вернул ошибку:
	if s.pool == nil || s.pool.Ping(ctx) != nil {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}
	w.WriteHeader(http.StatusOK) // 200
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

// CreateTrip обрабатывает запрос POST /api/v1/trips (Создание поездки).
// Заглушка (пока что)
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// GetTrip обрабатывает запрос GET /api/v1/trips/{tripId} (Получение поездки).
// Заглушка (пока что)
func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// FinishTrip обрабатывает запрос POST /api/v1/trips/{tripId}/finish (Завершение поездки).
// Заглушка (пока что)
func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func main() {
	// 1. Читаем конфигурацию из переменных окружения
	cfg, err := config.Load()
	if err != nil {
		// Если обязательные переменные (HTTP_ADDR, DATABASE_URL) не переданы - выходим
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Настраиваем глобальный логгер, чтобы он писал логи в формате JSON в stdout
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// 2. Инициализируем пул соединений с PostgreSQL (pgxpool)
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL) // парсим конфиг
	if err != nil {
		slog.Error("failed to parse database url", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = cfg.DatabaseMaxConns
	poolCfg.MinConns = cfg.DatabaseMinConns
	poolCfg.MaxConnLifetime = cfg.DatabaseMaxConnLifetime
	poolCfg.ConnConfig.ConnectTimeout = cfg.DatabaseConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg) // подключаем бдшку
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 3. Создаем роутер
	r := chi.NewRouter()
	// добавляет индификаторы
	r.Use(middleware.RequestID)
	// Перехватывает паники, отдаём 500
	r.Use(middleware.Recoverer)

	// экземпляр сервера с подключенным бд
	serverImpl := &Server{pool: pool}

	// Регистрируем маршруты OpenAPI в роутере chi
	api.HandlerFromMux(serverImpl, r)

	// http сервер
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,  // Время на вычитывание HTTP-заголовков
		ReadTimeout:       5 * time.Second,  // Время на вычитывание всего тела запроса
		WriteTimeout:      5 * time.Second,  // Время на запись ответа клиенту
		IdleTimeout:       30 * time.Second, // Время жизни неактивного keep-alive соединения
	}

	serverErrors := make(chan error, 1)

	// Запускаем слушание порта в фоновой горутине, чтобы не блокировать выполнение main
	go func() {
		slog.Info("starting HTTP server", "addr", cfg.HTTPAddr)
		// ListenAndServe блокирует поток
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 5. Ожидаем сигналы операционной системы для Graceful Shutdown

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors: // сервер упал
		slog.Error("server failed to start", "error", err)
		os.Exit(1)

	case sig := <-shutdownSignals: // shutdown

		slog.Info("shutdown signal received", "signal", sig.String())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed, forcing close", "error", err)
			_ = httpServer.Close()
		}
		slog.Info("server stopped gracefully")
	}
}

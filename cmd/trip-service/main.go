package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
	api "github.com/Godofdeath-dev/avitoLabs/internal/generated"
	"github.com/Godofdeath-dev/avitoLabs/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// хелпер для ошибок
func writeProblem(w http.ResponseWriter, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Problem{
		Status: int32(status),
		Code:   code,
		Title:  title,
		Detail: &detail,
	})
}

// Server хранит общие зависимости приложения (пул БД, сервисы)
// и реализует интерфейс api.ServerInterface.
type Server struct {
	pool      *pgxpool.Pool
	repo      *repository.TripRepository
	txManager repository.TxManager
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
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // 200
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

// Ready обрабатывает запрос GET /ready
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

// POST /api/v1/trips
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var body api.CreateTripJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid JSON", err.Error())
		return
	}

	// Бизнес-проверки, не покрытые OpenAPI: нулевая цена разрешена, пустые UUID отклоняем
	if body.Price < 0 {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid price", "Price must be zero or greater")
		return
	}
	if body.UserId == uuid.Nil || body.DriverId == uuid.Nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid UUID", "User ID and Driver ID cannot be empty (nil UUID)")
		return
	}

	newTrip := api.Trip{
		Id:         uuid.New(),
		UserId:     body.UserId,
		DriverId:   body.DriverId,
		Price:      body.Price,
		Status:     api.Active,
		StartPoint: body.StartPoint,
		EndPoint:   body.EndPoint,
		StartedAt:  time.Now().UTC(),
	}

	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		if err := s.repo.InsertTrip(ctx, newTrip); err != nil {
			return err
		}
		return s.repo.AddStatusHistory(ctx, newTrip.Id, nil, newTrip.Status, newTrip.StartedAt)
	})
	if err != nil {
		if errors.Is(err, repository.ErrDriverBusy) {
			writeProblem(w, http.StatusConflict, "driver_busy", "Conflict", "Driver already has an active trip")
			return
		}
		slog.Error("failed to create trip", "error", err)
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to save trip")
		return
	}

	// Установка обязательного заголовка Location перед отправкой 201
	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", newTrip.Id.String()))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(newTrip)
}

// GET /api/v1/trips/{tripId}
func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	trip, err := s.repo.GetTripByID(r.Context(), tripId)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "trip_not_found", "Not Found", "Trip not found")
			return
		}
		slog.Error("failed to get trip", "error", err)
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to get trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(trip)
}

// POST /api/v1/trips/{tripId}/finish
func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	now := time.Now().UTC()
	var trip *api.Trip

	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		var err error
		trip, err = s.repo.UpdateTripCompleted(ctx, tripId, now)
		if err != nil {
			return err
		}
		fromStatus := string(api.Active)
		return s.repo.AddStatusHistory(ctx, tripId, &fromStatus, api.Completed, now)
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "trip_not_found", "Not Found", "Trip not found")
			return
		}
		if errors.Is(err, repository.ErrAlreadyFinished) {
			writeProblem(w, http.StatusConflict, "trip_completed", "Conflict", "Trip is already completed") // Исправлено на trip_completed
			return
		}
		slog.Error("failed to finish trip", "error", err)
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to finish trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(trip)
}

func main() {
	// 1. Читаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 2. БД
	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to parse database url", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = cfg.DatabaseMaxConns
	poolCfg.MinConns = cfg.DatabaseMinConns
	poolCfg.MaxConnLifetime = cfg.DatabaseMaxConnLifetime
	poolCfg.ConnConfig.ConnectTimeout = cfg.DatabaseConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		slog.Error("failed to create database pool:", "error", err)
		os.Exit(1)
	}

	poolCtx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	defer cancel()
	if err := pool.Ping(poolCtx); err != nil {
		slog.Error("failed to ping database:", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to postgres successfully")

	// 3. Создаем роутер и репозитории
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Передаем таймаут базы данных в репозиторий и TxManager
	repo := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := repository.NewTxManager(pool, cfg.DatabaseQueryTimeout)

	serverImpl := &Server{
		pool:      pool,
		repo:      repo,
		txManager: txManager,
	}

	// 4. Подключаем OpenAPI валидатор
	swagger, err := api.GetSwagger()
	if err != nil {
		slog.Error("failed to load swagger spec", "error", err)
		os.Exit(1)
	}
	swagger.Servers = nil // Очищаем список серверов для корректной валидации

	validatorOptions := &oapimiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			writeProblem(w, statusCode, "invalid_request", "Invalid Request", message)
		},
	}
	r.Use(oapimiddleware.OapiRequestValidatorWithOptions(swagger, validatorOptions))

	// 5. Регистрируем маршруты с обработчиком ошибок в параметрах пути (ErrorHandlerFunc)
	handlerOptions := api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid parameter", err.Error())
		},
	}
	api.HandlerWithOptions(serverImpl, handlerOptions)

	// 6. запускаем сервер
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		slog.Info("starting HTTP server", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 7. Graceful Shutdown
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		slog.Error("server failed to start", "error", err)
		os.Exit(1)

	case sig := <-shutdownSignals:
		slog.Info("shutdown signal received", "signal", sig.String())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		done := make(chan struct{})

		go func() {
			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				slog.Error("server shutdown failed", "error", err)
			}
			if pool != nil {
				pool.Close()
				slog.Info("database connection pool closed")
			}
			close(done)
		}()

		select {
		case <-done:
			slog.Info("graceful shutdown completed")
		case <-shutdownCtx.Done():
			slog.Error("shutdown timeout exceeded, forcing exit")
			os.Exit(1)
		}
	}
}

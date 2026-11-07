package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/health"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/middleware"
	createhttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/create"
	finishhttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/finish"
	gethttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/get"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/telemetry"
	triprepo "github.com/Godofdeath-dev/avitoLabs/internal/repository/trip"
	createuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/create"
	finishuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/finish"
	getuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/get"
)

const serviceName = "trip-service"

type Container struct {
	httpServer   *http.Server
	pool         *pgxpool.Pool
	otelShutdown func(context.Context) error
	logger       *slog.Logger
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	logger := telemetry.NewLogger(cfg.LogLevel, serviceName, nil)

	otelShutdown, err := telemetry.SetupOTel(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("setup otel: %w", err)
	}

	metrics, err := telemetry.NewMetrics()
	if err != nil {
		_ = otelShutdown(ctx)
		return nil, fmt.Errorf("create metrics: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		_ = otelShutdown(ctx)
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		_ = otelShutdown(ctx)
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	txManager := transaction.NewManager(pool)
	repo := triprepo.New(pool, metrics)

	createUsecase := createuc.New(repo, txManager, metrics, logger)
	getUsecase := getuc.New(repo)
	finishUsecase := finishuc.New(repo, txManager, metrics, logger)

	createHandler := createhttp.New(createUsecase, logger)
	getHandler := gethttp.New(getUsecase, logger)
	finishHandler := finishhttp.New(finishUsecase, logger)
	healthHandler := health.New(pool)

	r := chi.NewRouter()
	// Порядок из п. 6.4: Recover -> Tracing -> Logging -> Metrics -> роутер
	r.Use(middleware.Recover(logger))
	r.Use(middleware.Tracing(serviceName))
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Metrics(metrics))

	r.Post("/api/v1/trips", createHandler.Handle)
	r.Get("/api/v1/trips/{tripId}", getHandler.Handle)
	r.Post("/api/v1/trips/{tripId}/finish", finishHandler.Handle)
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return &Container{
		httpServer:   httpServer,
		pool:         pool,
		otelShutdown: otelShutdown,
		logger:       logger,
	}, nil
}

func (c *Container) Logger() *slog.Logger {
	return c.logger
}

func (c *Container) Start() <-chan error {
	errCh := make(chan error, 1)
	go func() {
		c.logger.Info("starting HTTP server", slog.String("addr", c.httpServer.Addr))
		if err := c.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	return errCh
}

func (c *Container) Shutdown(ctx context.Context) error {
	var err error
	if shutdownErr := c.httpServer.Shutdown(ctx); shutdownErr != nil {
		err = errors.Join(err, shutdownErr)
		_ = c.httpServer.Close()
	}
	if c.pool != nil {
		c.pool.Close()
	}
	if c.otelShutdown != nil {
		err = errors.Join(err, c.otelShutdown(ctx))
	}
	return err
}

package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
	pushgateway "github.com/Godofdeath-dev/avitoLabs/internal/gateway/push"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/health"
	"github.com/Godofdeath-dev/avitoLabs/internal/http/middleware"
	createhttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/create"
	finishhttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/finish"
	gethttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/get"
	listpositionshttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/positions/list"
	savepositionhttp "github.com/Godofdeath-dev/avitoLabs/internal/http/trip/positions/save"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/postgres/transaction"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/telemetry"
	triprepo "github.com/Godofdeath-dev/avitoLabs/internal/repository/trip"
	createuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/create"
	finishuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/finish"
	getuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/get"
	listpositionsuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/positions/list"
	savepositionuc "github.com/Godofdeath-dev/avitoLabs/internal/usecase/trip/positions/save"
	"github.com/Godofdeath-dev/avitoLabs/internal/worker/positions"
)

type Container struct {
	httpServer            *http.Server
	pool                  *pgxpool.Pool
	otelShutdown          func(context.Context) error
	logger                *slog.Logger
	positions             *positions.Pool
	push                  *pushgateway.Gateway
	unregisterPoolMetrics func() error
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	serviceName := cfg.ServiceName
	logger := telemetry.NewLogger(cfg.LogLevel, serviceName, nil)

	otelShutdown, err := telemetry.SetupOTel(ctx, serviceName)
	if err != nil {
		_ = otelShutdown(ctx)
		return nil, fmt.Errorf("setup otel: %w", err)
	}

	metrics, err := telemetry.NewMetrics()
	if err != nil {
		_ = otelShutdown(ctx)
		return nil, fmt.Errorf("create metrics: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg, metrics)
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

	txManager := transaction.NewManager(pool, cfg.DatabaseQueryTimeout)
	repo := triprepo.New(pool, cfg.DatabaseQueryTimeout)

	positionMetrics, err := telemetry.NewPositionMetrics()
	if err != nil {
		pool.Close()
		_ = otelShutdown(ctx)
		return nil, err
	}
	tracer := otel.Tracer(serviceName)
	push, err := pushgateway.New(cfg.PushHTTPURL, cfg.PushRequestTimeout, tracer)
	if err != nil {
		pool.Close()
		_ = otelShutdown(ctx)
		return nil, err
	}
	positionPool, err := positions.New(repo, push, positionMetrics, clockwork.NewRealClock(), positions.Config{Interval: cfg.PositionCheckInterval, StaleAfter: cfg.PositionStaleAfter, Workers: cfg.PositionWorkers, QueueSize: cfg.PositionQueueSize, RequestTimeout: cfg.PushRequestTimeout}, logger, tracer)
	if err != nil {
		push.Close()
		pool.Close()
		_ = otelShutdown(ctx)
		return nil, err
	}
	unregister, err := positionMetrics.BindPool(positionPool.Snapshot)
	if err != nil {
		push.Close()
		pool.Close()
		_ = otelShutdown(ctx)
		return nil, err
	}
	savePositionHandler := savepositionhttp.New(savepositionuc.New(repo, txManager, positionPool, positionMetrics, clockwork.NewRealClock()), logger)
	listPositionsHandler := listpositionshttp.New(listpositionsuc.New(repo), logger)

	createUsecase := createuc.New(repo, txManager, metrics, logger)
	getUsecase := getuc.New(repo)
	finishUsecase := finishuc.New(repo, txManager, metrics, logger)

	createHandler := createhttp.New(createUsecase, logger)
	getHandler := gethttp.New(getUsecase, logger)
	finishHandler := finishhttp.New(finishUsecase, logger)
	healthHandler := health.New(pool, cfg.DatabaseQueryTimeout)

	r := chi.NewRouter()
	// Порядок из п. 6.4: Recover -> Tracing -> Logging -> Metrics -> роутер
	r.Use(middleware.Recover(logger))
	r.Use(middleware.Tracing(serviceName))
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Metrics(metrics))

	r.Post("/api/v1/trips", createHandler.Handle)
	r.Get("/api/v1/trips/{tripId}", getHandler.Handle)
	r.Post("/api/v1/trips/{tripId}/finish", finishHandler.Handle)
	r.Post("/api/v1/trips/{tripId}/positions", savePositionHandler.Handle)
	r.Get("/api/v1/trips/{tripId}/positions", listPositionsHandler.Handle)
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           middleware.ResolveRoute(r)(r),
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	return &Container{
		httpServer:   httpServer,
		pool:         pool,
		otelShutdown: otelShutdown,
		logger:       logger,
		positions:    positionPool, push: push, unregisterPoolMetrics: unregister,
	}, nil
}

func (c *Container) Logger() *slog.Logger {
	return c.logger
}

func (c *Container) Start(ctx context.Context) <-chan error {
	c.positions.Start(ctx)
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
	if stopErr := c.positions.Stop(ctx); stopErr != nil {
		err = errors.Join(err, stopErr)
	}
	c.push.Close()
	if unregisterErr := c.unregisterPoolMetrics(); unregisterErr != nil {
		err = errors.Join(err, unregisterErr)
	}
	if c.pool != nil {
		c.pool.Close()
	}
	if c.otelShutdown != nil {
		err = errors.Join(err, c.otelShutdown(ctx))
	}
	return err
}

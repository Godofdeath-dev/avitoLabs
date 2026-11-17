package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
	"github.com/Godofdeath-dev/avitoLabs/internal/container"
	"github.com/Godofdeath-dev/avitoLabs/internal/infra/telemetry"
)

func main() { os.Exit(run()) }

func run() int {
	slog.SetDefault(telemetry.NewLogger(os.Getenv("LOG_LEVEL"), "trip-service", os.Stdout))
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		return 1
	}
	ctx := context.Background()
	c, err := container.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize container", "error", err)
		return 1
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	exitCode := 0
	select {
	case err := <-c.Start(ctx):
		if err != nil {
			c.Logger().Error("server failed", "error", err)
			exitCode = 1
		}
	case sig := <-signals:
		c.Logger().Info("shutdown signal received", "signal", sig.String())
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.ShutdownTimeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Shutdown(shutdownCtx) }()
	select {
	case err := <-done:
		if err != nil {
			c.Logger().Error("graceful shutdown failed", "error", err)
			return 1
		}
	case <-shutdownCtx.Done():
		c.Logger().Error("shutdown timeout exceeded; forcing exit", "error", shutdownCtx.Err())
		return 1
	}
	c.Logger().Info("server stopped")
	return exitCode
}

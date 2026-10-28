package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Godofdeath-dev/avitoLabs/internal/config"
	"github.com/Godofdeath-dev/avitoLabs/internal/container"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx := context.Background()
	c, err := container.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize container", slog.String("error", err.Error()))
		os.Exit(1)
	}

	serverErrors := c.Start()

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil {
			c.Logger().Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	case sig := <-shutdownSignals:
		c.Logger().Info("shutdown signal received", slog.String("signal", sig.String()))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := c.Shutdown(shutdownCtx); err != nil {
			c.Logger().Error("graceful shutdown failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		c.Logger().Info("server stopped gracefully")
	}
}

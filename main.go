// Package main is the entry point for alertmanager-webhook-relay.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"alertmanager-webhook-relay/internal/config"
	"alertmanager-webhook-relay/internal/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New(cfg.SlogLevel())
	slog.SetDefault(logger)

	logger.Info("starting alertmanager-webhook-relay", "config", cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // safe: os.Exit is called in main(), not in run()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	// TODO: использовать cfg.ShutdownTimeout для graceful shutdown HTTP-сервера
	logger.Info("shutdown complete")

	return nil
}

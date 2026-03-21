// Package main is the entry point for alertmanager-webhook-relay.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"alertmanager-webhook-relay/internal/config"
	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/server"
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

	srv := server.New(
		server.Config{
			Port:            cfg.Port,
			ShutdownTimeout: cfg.ShutdownTimeout,
		},
		logger,
	)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		return err
	}

	logger.Info("shutdown complete")

	return nil
}

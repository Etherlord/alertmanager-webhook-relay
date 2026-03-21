// Package main is the entry point for alertmanager-webhook-relay.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/config"
	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/server"
	"alertmanager-webhook-relay/internal/storage/sqlite"
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

	// Initialize SQLite store.
	store, err := sqlite.New(cfg.DatabaseDSN)
	if err != nil {
		return err
	}
	logger.Info("database initialized", "driver", cfg.DatabaseDriver, "dsn", cfg.DatabaseDSN)

	// Build alert service and handler.
	alertSvc := alerts.NewService(store, logger, cfg.MaxAlertsPerPayload)
	alertHandler := alerts.HandleWebhook(logger, alertSvc, int64(cfg.MaxPayloadSize))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // safe: os.Exit is called in main(), not in run()

	srv := server.New(
		server.Config{
			Port:            cfg.Port,
			ShutdownTimeout: cfg.ShutdownTimeout,
		},
		logger,
		alertHandler,
		store, // SQLite store implements server.Checker
	)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		store.Close()
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		store.Close()
		return err
	}

	store.Close()
	logger.Info("shutdown complete")

	return nil
}

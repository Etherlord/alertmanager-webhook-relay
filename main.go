// Package main is the entry point for alertmanager-webhook-relay.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/config"
	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/notify"
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

	// Initialize store based on configured driver.
	var store alerts.Store
	var storeChecker server.Checker

	switch cfg.DatabaseDriver {
	case "sqlite":
		s, err := sqlite.New(cfg.DatabaseDSN, logger)
		if err != nil {
			return err
		}
		store = s
		storeChecker = s
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.DatabaseDriver)
	}

	logger.Info("database initialized", "driver", cfg.DatabaseDriver, "dsn", cfg.DatabaseDSN)

	// Build alert service and handler.
	alertSvc := alerts.NewService(store, logger, cfg.MaxAlertsPerPayload)
	alertHandler := alerts.HandleWebhook(logger, alertSvc, int64(cfg.MaxPayloadSize))

	// Build notification dispatcher.
	// Empty channels list — concrete channels (Pachca, Email) added in future milestones.
	var channels []notify.Channel
	dispatcher := notify.NewDispatcher(store, channels, notify.DispatcherConfig{
		PollInterval: cfg.NotifyPollInterval,
		BatchSize:    cfg.NotifyBatchSize,
		QueueSize:    cfg.NotifyQueueSize,
		SendTimeout:  cfg.NotifySendTimeout,
	}, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // safe: os.Exit is called in main(), not in run()

	srv := server.New(
		server.Config{
			Port:            cfg.Port,
			ShutdownTimeout: cfg.ShutdownTimeout,
		},
		logger,
		alertHandler,
		storeChecker,
		dispatcher,
	)

	// Start dispatcher in background goroutine.
	dispatcherCtx, dispatcherCancel := context.WithCancel(ctx)
	dispatcherDone := make(chan error, 1)
	go func() {
		dispatcherDone <- dispatcher.Run(dispatcherCtx)
	}()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		dispatcherCancel()
		<-dispatcherDone
		if closeErr := store.Close(); closeErr != nil {
			logger.Error("failed to close store", "error", closeErr)
		}
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	// Shutdown sequence: srv.Shutdown → dispatcher stops → store.Close
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		dispatcherCancel()
		<-dispatcherDone
		if closeErr := store.Close(); closeErr != nil {
			logger.Error("failed to close store", "error", closeErr)
		}
		return err
	}

	logger.Info("stopping dispatcher")
	dispatcherCancel()
	if err := <-dispatcherDone; err != nil {
		logger.Error("dispatcher error", "error", err)
	}
	logger.Info("dispatcher stopped")

	if err := store.Close(); err != nil {
		logger.Error("failed to close store", "error", err)
		return err
	}

	logger.Info("shutdown complete")

	return nil
}

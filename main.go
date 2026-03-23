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
	"alertmanager-webhook-relay/internal/channel/email"
	"alertmanager-webhook-relay/internal/channel/pachca"
	"alertmanager-webhook-relay/internal/config"
	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/notify"
	"alertmanager-webhook-relay/internal/server"
	"alertmanager-webhook-relay/internal/storage/sqlite"
	"alertmanager-webhook-relay/internal/template"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // safe: os.Exit is called in main(), not in run()

	// Build notification channels.
	var channels []notify.Channel

	if cfg.Pachca.Enabled {
		channels = append(channels, pachca.NewChannel(cfg.Pachca.BaseURL, cfg.Pachca.Token, cfg.Pachca.ChatID, logger))
		logger.Info("pachca channel enabled", "chat_id", cfg.Pachca.ChatID)
	} else {
		logger.Info("pachca channel disabled")
	}

	if cfg.Email.Enabled {
		emailSender := email.NewSender(
			cfg.Email.SMTPHost, cfg.Email.SMTPPort,
			cfg.Email.From, cfg.Email.Username, cfg.Email.Password,
			cfg.Email.TLSMode, logger,
		)

		var formatter email.BodyFormatter
		if cfg.Template.Dir != "" {
			engine, err := template.NewEngine(cfg.Template.Dir, email.DefaultFuncMap(), logger)
			if err != nil {
				return fmt.Errorf("template engine: %w", err)
			}
			formatter = email.NewTemplateFormatter(engine, "default.html.tmpl", logger)
			logger.Info("template engine initialized", "dir", cfg.Template.Dir)

			if cfg.Template.ReloadEnabled {
				watcher := template.NewWatcher(engine, logger)
				go func() {
					if err := watcher.Watch(ctx); err != nil {
						logger.Error("template watcher error", "error", err)
					}
				}()
				logger.Info("template watcher started", "dir", cfg.Template.Dir)
			}
		}

		channels = append(channels, email.NewChannel(emailSender, cfg.Email.To, cfg.Email.SubjectPrefix, logger, formatter))
		logger.Info("email channel enabled", "smtp_host", cfg.Email.SMTPHost, "to", cfg.Email.To)
	} else {
		logger.Info("email channel disabled")
	}
	dispatcher := notify.NewDispatcher(store, channels, notify.DispatcherConfig{
		PollInterval: cfg.NotifyPollInterval,
		BatchSize:    cfg.NotifyBatchSize,
		QueueSize:    cfg.NotifyQueueSize,
		SendTimeout:  cfg.NotifySendTimeout,
	}, logger)

	srv := server.New(
		server.Config{
			Port:            cfg.Port,
			ShutdownTimeout: cfg.ShutdownTimeout,
			PreStopDelay:    cfg.PreStopDelay,
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

	cleanup := func() {
		dispatcherCancel()
		if err := <-dispatcherDone; err != nil {
			logger.Error("dispatcher error", "error", err)
		}
		if err := store.Close(); err != nil {
			logger.Error("failed to close store", "error", err)
		}
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		cleanup()
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, starting graceful shutdown")
	}

	// Shutdown sequence: srv.Shutdown → dispatcher drains → store.Close (with WAL checkpoint)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	logger.Info("step 1/3: shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
		cleanup()
		return err
	}

	logger.Info("step 2/3: stopping dispatcher, draining in-flight notifications")
	dispatcherCancel()
	if err := <-dispatcherDone; err != nil {
		logger.Error("dispatcher error", "error", err)
	}

	logger.Info("step 3/3: closing store (WAL checkpoint)")
	if err := store.Close(); err != nil {
		logger.Error("failed to close store", "error", err)
	}

	logger.Info("shutdown complete")

	return nil
}

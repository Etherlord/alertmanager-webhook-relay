package server_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/server"
	"alertmanager-webhook-relay/internal/storage/sqlite"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestNew_CreatesServer(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger, nil)
	assert.NotNil(t, srv)
}

func TestServer_HealthzRoute(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	<-srv.Ready()

	addr := srv.Addr()
	require.NotEmpty(t, addr)

	resp := httpGet(t, "http://"+addr+"/healthz")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestServer_ReadyzRoute(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	<-srv.Ready()

	resp := httpGet(t, "http://"+srv.Addr()+"/readyz")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_ReadyzWithFailedChecker(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	failChecker := &stubChecker{name: "db", err: errors.New("down")}
	srv := server.New(cfg, logger, nil, failChecker)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	<-srv.Ready()

	resp := httpGet(t, "http://"+srv.Addr()+"/readyz")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestServer_MiddlewareChainApplied(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	<-srv.Ready()

	// Запрос на несуществующий маршрут должен вернуть 404 (а не panic).
	resp := httpGet(t, "http://"+srv.Addr()+"/nonexistent")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_AlertWebhookRoute(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	// Use file-based temp DB so goose and sqlite.New share the same database.
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	goose.SetLogger(goose.NopLogger())
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(db, "../../migrations/sqlite"))
	require.NoError(t, db.Close())

	store, err := sqlite.New(dbPath, logger)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	svc := alerts.NewService(store, logger, 100)
	handler := alerts.HandleWebhook(logger, svc, 1<<20)
	srv := server.New(cfg, logger, handler, store)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	<-srv.Ready()

	payload := `{
		"receiver": "webhook",
		"status": "firing",
		"alerts": [{
			"status": "firing",
			"labels": {"alertname": "TestAlert", "severity": "critical"},
			"annotations": {"summary": "test"},
			"startsAt": "2026-03-16T08:00:00Z",
			"endsAt": "0001-01-01T00:00:00Z",
			"generatorURL": "http://example.com",
			"fingerprint": "abc123"
		}],
		"groupLabels": {"alertname": "TestAlert"},
		"commonLabels": {"alertname": "TestAlert"},
		"commonAnnotations": {},
		"externalURL": "http://alertmanager:9093",
		"version": "4",
		"groupKey": "test-key",
		"truncatedAlerts": 0
	}`

	resp := httpPost(t, "http://"+srv.Addr()+"/api/v1/alerts", []byte(payload))
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestServer_GracefulShutdown(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	<-srv.Ready()

	// Проверяем, что сервер работает.
	resp := httpGet(t, "http://"+srv.Addr()+"/healthz")
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shutdown.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err := srv.Shutdown(shutdownCtx)
	assert.NoError(t, err)

	// Start должен завершиться без ошибки (http.ErrServerClosed — штатное завершение).
	select {
	case startErr := <-errCh:
		assert.NoError(t, startErr)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}

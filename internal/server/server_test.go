package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_CreatesServer(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger)
	assert.NotNil(t, srv)
}

func TestServer_HealthzRoute(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Даём серверу время запуститься.
	time.Sleep(50 * time.Millisecond)

	addr := srv.Addr()
	require.NotEmpty(t, addr)

	resp, err := http.Get("http://" + addr + "/healthz")
	require.NoError(t, err)
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

	srv := server.New(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get("http://" + srv.Addr() + "/readyz")
	require.NoError(t, err)
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
	srv := server.New(cfg, logger, failChecker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get("http://" + srv.Addr() + "/readyz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestServer_MiddlewareChainApplied(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Запрос на несуществующий маршрут должен вернуть 404 (а не panic).
	resp, err := http.Get("http://" + srv.Addr() + "/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_GracefulShutdown(t *testing.T) {
	logger := logging.NewWithWriter(slogLevelDebug, io.Discard)
	cfg := server.Config{
		Port:            0,
		ShutdownTimeout: 5 * time.Second,
	}

	srv := server.New(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Проверяем, что сервер работает.
	resp, err := http.Get("http://" + srv.Addr() + "/healthz")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shutdown.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = srv.Shutdown(shutdownCtx)
	assert.NoError(t, err)

	// Start должен завершиться без ошибки (http.ErrServerClosed — штатное завершение).
	select {
	case startErr := <-errCh:
		assert.NoError(t, startErr)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server to stop")
	}
}

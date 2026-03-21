package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setDefaults isolates tests from the real environment by setting
// all optional ENV variables to empty strings.
func setDefaults(t *testing.T) {
	t.Helper()
	t.Setenv("PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
}

func TestLoad_Defaults(t *testing.T) {
	setDefaults(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout)

	t.Logf("defaults: port=%d, log_level=%s, shutdown_timeout=%s", cfg.Port, cfg.LogLevel, cfg.ShutdownTimeout)
}

func TestLoad_CustomValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)

	t.Logf("custom: port=%d, log_level=%s, shutdown_timeout=%s", cfg.Port, cfg.LogLevel, cfg.ShutdownTimeout)
}

func TestLoad_InvalidPort(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "65536"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("PORT", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("PORT=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"trace", "trace"},
		{"fatal", "fatal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("LOG_LEVEL", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("LOG_LEVEL=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidShutdownTimeout(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"invalid format", "not-a-duration"},
		{"negative", "-5s"},
		{"zero", "0s"},
		{"too large", "10m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("SHUTDOWN_TIMEOUT", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("SHUTDOWN_TIMEOUT=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_ValidEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		port            string
		shutdownTimeout string
		expectedPort    int
		expectedTimeout time.Duration
	}{
		{"min port", "1", "15s", 1, 15 * time.Second},
		{"max port", "65535", "15s", 65535, 15 * time.Second},
		{"min timeout", "", "1s", 8080, 1 * time.Second},
		{"max timeout", "", "5m", 8080, 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			if tt.port != "" {
				t.Setenv("PORT", tt.port)
			}
			t.Setenv("SHUTDOWN_TIMEOUT", tt.shutdownTimeout)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPort, cfg.Port)
			assert.Equal(t, tt.expectedTimeout, cfg.ShutdownTimeout)

			t.Logf("edge case %q: port=%d, timeout=%s", tt.name, cfg.Port, cfg.ShutdownTimeout)
		})
	}
}

func TestLoad_Normalization(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "uppercase LOG_LEVEL normalized to lowercase",
			env:  "INFO",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "info", cfg.LogLevel)
			},
		},
		{
			name: "whitespace LOG_LEVEL trimmed",
			env:  " debug ",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("LOG_LEVEL", tt.env)

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)

			t.Logf("LOG_LEVEL=%q → normalized=%q", tt.env, cfg.LogLevel)
		})
	}
}

func TestConfig_SlogLevel(t *testing.T) {
	tests := []struct {
		logLevel string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.logLevel, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("LOG_LEVEL", tt.logLevel)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.SlogLevel())

			t.Logf("LogLevel=%q → slog.Level=%v", tt.logLevel, cfg.SlogLevel())
		})
	}
}

func TestConfig_LogValue(t *testing.T) {
	setDefaults(t)
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")

	cfg, err := Load()
	require.NoError(t, err)

	// Check LogValue returns a valid slog.Value via String()
	logValue := cfg.LogValue()
	str := logValue.String()
	t.Logf("LogValue().String() = %s", str)

	assert.Contains(t, str, "9090")
	assert.Contains(t, str, "warn")
	assert.Contains(t, str, "30s")

	// Check through a real slog.Logger writing to a buffer
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("test", "config", cfg)

	output := buf.String()
	t.Logf("slog output: %s", output)

	assert.Contains(t, output, "9090")
	assert.Contains(t, output, "warn")
	assert.Contains(t, output, "30s")
}

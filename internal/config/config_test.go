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
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_DSN", "")
	t.Setenv("MAX_PAYLOAD_SIZE", "")
	t.Setenv("MAX_ALERTS_PER_PAYLOAD", "")
}

func TestLoad_Defaults(t *testing.T) {
	setDefaults(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, "sqlite", cfg.DatabaseDriver)
	assert.Equal(t, "data/alerts.db", cfg.DatabaseDSN)
	assert.Equal(t, 1048576, cfg.MaxPayloadSize)
	assert.Equal(t, 100, cfg.MaxAlertsPerPayload)

	t.Logf("defaults: port=%d, log_level=%s, shutdown_timeout=%s, db_driver=%s, db_dsn=%s, max_payload=%d, max_alerts=%d",
		cfg.Port, cfg.LogLevel, cfg.ShutdownTimeout,
		cfg.DatabaseDriver, cfg.DatabaseDSN, cfg.MaxPayloadSize, cfg.MaxAlertsPerPayload)
}

func TestLoad_CustomValues(t *testing.T) {
	setDefaults(t)
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", "/tmp/test.db")
	t.Setenv("MAX_PAYLOAD_SIZE", "2097152")
	t.Setenv("MAX_ALERTS_PER_PAYLOAD", "50")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, "sqlite", cfg.DatabaseDriver)
	assert.Equal(t, "/tmp/test.db", cfg.DatabaseDSN)
	assert.Equal(t, 2097152, cfg.MaxPayloadSize)
	assert.Equal(t, 50, cfg.MaxAlertsPerPayload)

	t.Logf("custom: port=%d, db_driver=%s, db_dsn=%s, max_payload=%d, max_alerts=%d",
		cfg.Port, cfg.DatabaseDriver, cfg.DatabaseDSN, cfg.MaxPayloadSize, cfg.MaxAlertsPerPayload)
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

func TestLoad_InvalidDatabaseDriver(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"postgres not supported yet", "postgres"},
		{"mysql", "mysql"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("DATABASE_DRIVER", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("DATABASE_DRIVER=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidDatabaseDSN(t *testing.T) {
	setDefaults(t)
	// Whitespace-only is normalized to empty, which should fail validation.
	t.Setenv("DATABASE_DSN", "   ")

	cfg, err := Load()
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ErrInvalidConfig)

	t.Logf("DATABASE_DSN='   ' → error: %v", err)
}

func TestLoad_InvalidMaxPayloadSize(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"too small", "1023"},
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "10485761"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("MAX_PAYLOAD_SIZE", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("MAX_PAYLOAD_SIZE=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidMaxAlertsPerPayload(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "1001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("MAX_ALERTS_PER_PAYLOAD", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("MAX_ALERTS_PER_PAYLOAD=%q → error: %v", tt.value, err)
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

func TestLoad_ValidEdgeCases_Database(t *testing.T) {
	tests := []struct {
		name        string
		envKey      string
		envValue    string
		checkField  string
		expectedInt int
	}{
		{"min MAX_PAYLOAD_SIZE", "MAX_PAYLOAD_SIZE", "1024", "MaxPayloadSize", 1024},
		{"max MAX_PAYLOAD_SIZE", "MAX_PAYLOAD_SIZE", "10485760", "MaxPayloadSize", 10485760},
		{"min MAX_ALERTS_PER_PAYLOAD", "MAX_ALERTS_PER_PAYLOAD", "1", "MaxAlertsPerPayload", 1},
		{"max MAX_ALERTS_PER_PAYLOAD", "MAX_ALERTS_PER_PAYLOAD", "1000", "MaxAlertsPerPayload", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv(tt.envKey, tt.envValue)

			cfg, err := Load()
			require.NoError(t, err)

			switch tt.checkField {
			case "MaxPayloadSize":
				assert.Equal(t, tt.expectedInt, cfg.MaxPayloadSize)
			case "MaxAlertsPerPayload":
				assert.Equal(t, tt.expectedInt, cfg.MaxAlertsPerPayload)
			}

			t.Logf("edge case %q: %s=%d", tt.name, tt.envKey, tt.expectedInt)
		})
	}
}

func TestLoad_Normalization(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		env    string
		check  func(t *testing.T, cfg *Config)
	}{
		{
			name:   "uppercase LOG_LEVEL normalized to lowercase",
			envKey: "LOG_LEVEL",
			env:    "INFO",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "info", cfg.LogLevel)
			},
		},
		{
			name:   "whitespace LOG_LEVEL trimmed",
			envKey: "LOG_LEVEL",
			env:    " debug ",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
		{
			name:   "uppercase DATABASE_DRIVER normalized to lowercase",
			envKey: "DATABASE_DRIVER",
			env:    "SQLITE",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "sqlite", cfg.DatabaseDriver)
			},
		},
		{
			name:   "whitespace DATABASE_DSN trimmed",
			envKey: "DATABASE_DSN",
			env:    " /tmp/test.db ",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "/tmp/test.db", cfg.DatabaseDSN)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv(tt.envKey, tt.env)

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)

			t.Logf("%s=%q → normalized", tt.envKey, tt.env)
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
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", "/tmp/test.db")
	t.Setenv("MAX_PAYLOAD_SIZE", "2097152")
	t.Setenv("MAX_ALERTS_PER_PAYLOAD", "50")

	cfg, err := Load()
	require.NoError(t, err)

	// Check LogValue returns a valid slog.Value via String()
	logValue := cfg.LogValue()
	str := logValue.String()
	t.Logf("LogValue().String() = %s", str)

	assert.Contains(t, str, "9090")
	assert.Contains(t, str, "warn")
	assert.Contains(t, str, "30s")
	assert.Contains(t, str, "sqlite")
	assert.Contains(t, str, "/tmp/test.db")
	assert.Contains(t, str, "2097152")
	assert.Contains(t, str, "50")

	// Check through a real slog.Logger writing to a buffer
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("test", "config", cfg)

	output := buf.String()
	t.Logf("slog output: %s", output)

	assert.Contains(t, output, "9090")
	assert.Contains(t, output, "warn")
	assert.Contains(t, output, "sqlite")
}

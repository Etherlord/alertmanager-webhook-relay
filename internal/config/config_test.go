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
	t.Setenv("NOTIFY_POLL_INTERVAL", "")
	t.Setenv("NOTIFY_BATCH_SIZE", "")
	t.Setenv("NOTIFY_QUEUE_SIZE", "")
	t.Setenv("NOTIFY_SEND_TIMEOUT", "")
	t.Setenv("PACHCA_TOKEN", "")
	t.Setenv("PACHCA_BASE_URL", "")
	t.Setenv("PACHCA_CHAT_ID", "")
	t.Setenv("EMAIL_SMTP_HOST", "")
	t.Setenv("EMAIL_SMTP_PORT", "")
	t.Setenv("EMAIL_FROM", "")
	t.Setenv("EMAIL_TO", "")
	t.Setenv("EMAIL_USERNAME", "")
	t.Setenv("EMAIL_PASSWORD", "")
	t.Setenv("EMAIL_TLS_MODE", "")
	t.Setenv("EMAIL_SUBJECT_PREFIX", "")
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
	assert.Equal(t, 5*time.Second, cfg.NotifyPollInterval)
	assert.Equal(t, 50, cfg.NotifyBatchSize)
	assert.Equal(t, 100, cfg.NotifyQueueSize)
	assert.Equal(t, 30*time.Second, cfg.NotifySendTimeout)

	// Email defaults
	assert.False(t, cfg.Email.Enabled)
	assert.Empty(t, cfg.Email.SMTPHost)
	assert.Equal(t, 587, cfg.Email.SMTPPort)
	assert.Empty(t, cfg.Email.From)
	assert.Nil(t, cfg.Email.To)
	assert.Empty(t, cfg.Email.Username)
	assert.Empty(t, cfg.Email.Password)
	assert.Equal(t, "starttls", cfg.Email.TLSMode)
	assert.Equal(t, "[Alert]", cfg.Email.SubjectPrefix)

	t.Logf("defaults: port=%d, log_level=%s, shutdown_timeout=%s, db_driver=%s, db_dsn=%s, max_payload=%d, max_alerts=%d",
		cfg.Port, cfg.LogLevel, cfg.ShutdownTimeout,
		cfg.DatabaseDriver, cfg.DatabaseDSN, cfg.MaxPayloadSize, cfg.MaxAlertsPerPayload)
	t.Logf("notify defaults: poll_interval=%s, batch_size=%d, queue_size=%d, send_timeout=%s",
		cfg.NotifyPollInterval, cfg.NotifyBatchSize, cfg.NotifyQueueSize, cfg.NotifySendTimeout)
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
	tests := []struct {
		name        string
		value       string
		errContains string
	}{
		{"whitespace only", "   ", "не может быть пустым"},
		{"too long", strings.Repeat("a", 2049), "превышает максимум"},
		{"control chars tab", "data\t.db", "содержит управляющие символы"},
		{"shell expansion", "$(whoami).db", "опасную последовательность"},
		{"template injection", "data{{.Foo}}.db", "опасную последовательность"},
		{"backtick injection", "data`id`.db", "опасную последовательность"},
		{"CRLF injection", "data\r\n.db", "содержит управляющие символы"},
		{"URL-encoded null", "data%00.db", "опасную последовательность"},
		{"URL-encoded CRLF", "data%0d%0a.db", "опасную последовательность"},
		{"ANSI escape", "data\x1b[31m.db", "содержит управляющие символы"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("DATABASE_DSN", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}

			t.Logf("DATABASE_DSN=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_ValidDatabaseDSN(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"simple path", "data/alerts.db"},
		{"absolute path", "/var/lib/app/data.db"},
		{"postgres DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable"},
		{"max length", strings.Repeat("a", 2048)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("DATABASE_DSN", tt.value)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.value, cfg.DatabaseDSN)

			t.Logf("DATABASE_DSN=%q → ok", tt.value)
		})
	}
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
	assert.Contains(t, str, "[REDACTED]")
	assert.NotContains(t, str, "/tmp/test.db")
	assert.Contains(t, str, "2097152")
	assert.Contains(t, str, "50")
	assert.Contains(t, str, "5s")           // notify_poll_interval default
	assert.Contains(t, str, "notify_batch") // notify_batch_size
	assert.Contains(t, str, "notify_queue") // notify_queue_size
	assert.Contains(t, str, "30s")          // notify_send_timeout default

	// Check through a real slog.Logger writing to a buffer
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("test", "config", cfg)

	output := buf.String()
	t.Logf("slog output: %s", output)

	assert.Contains(t, output, "9090")
	assert.Contains(t, output, "warn")
	assert.Contains(t, output, "sqlite")
	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "/tmp/test.db")
}

func TestLoad_NotifyCustomValues(t *testing.T) {
	setDefaults(t)
	t.Setenv("NOTIFY_POLL_INTERVAL", "10s")
	t.Setenv("NOTIFY_BATCH_SIZE", "200")
	t.Setenv("NOTIFY_QUEUE_SIZE", "500")
	t.Setenv("NOTIFY_SEND_TIMEOUT", "60s")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 10*time.Second, cfg.NotifyPollInterval)
	assert.Equal(t, 200, cfg.NotifyBatchSize)
	assert.Equal(t, 500, cfg.NotifyQueueSize)
	assert.Equal(t, 60*time.Second, cfg.NotifySendTimeout)
}

func TestLoad_InvalidNotifyPollInterval(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"invalid format", "not-a-duration"},
		{"zero", "0s"},
		{"negative", "-1s"},
		{"below min", "500ms"},
		{"above max", "61s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("NOTIFY_POLL_INTERVAL", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("NOTIFY_POLL_INTERVAL=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidNotifyBatchSize(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "501"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("NOTIFY_BATCH_SIZE", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("NOTIFY_BATCH_SIZE=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidNotifyQueueSize(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"below min", "9"},
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "10001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("NOTIFY_QUEUE_SIZE", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("NOTIFY_QUEUE_SIZE=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidNotifySendTimeout(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"invalid format", "not-a-duration"},
		{"zero", "0s"},
		{"negative", "-1s"},
		{"below min", "4s"},
		{"above max", "121s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("NOTIFY_SEND_TIMEOUT", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("NOTIFY_SEND_TIMEOUT=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_NotifyEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		check    func(t *testing.T, cfg *Config)
	}{
		{"min poll interval", "NOTIFY_POLL_INTERVAL", "1s", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, time.Second, cfg.NotifyPollInterval)
		}},
		{"max poll interval", "NOTIFY_POLL_INTERVAL", "1m", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, time.Minute, cfg.NotifyPollInterval)
		}},
		{"min batch size", "NOTIFY_BATCH_SIZE", "1", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 1, cfg.NotifyBatchSize)
		}},
		{"max batch size", "NOTIFY_BATCH_SIZE", "100", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 100, cfg.NotifyBatchSize)
		}},
		{"min queue size", "NOTIFY_QUEUE_SIZE", "50", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 50, cfg.NotifyQueueSize)
		}},
		{"max queue size", "NOTIFY_QUEUE_SIZE", "10000", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 10000, cfg.NotifyQueueSize)
		}},
		{"min send timeout", "NOTIFY_SEND_TIMEOUT", "5s", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 5*time.Second, cfg.NotifySendTimeout)
		}},
		{"max send timeout", "NOTIFY_SEND_TIMEOUT", "2m", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 2*time.Minute, cfg.NotifySendTimeout)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv(tt.envKey, tt.envValue)

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)

			t.Logf("edge case %q: %s=%s", tt.name, tt.envKey, tt.envValue)
		})
	}
}

func TestLoad_NotifyCrossFieldEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		batch string
		queue string
		check func(t *testing.T, cfg *Config)
	}{
		{"max batch size with matching queue", "500", "500", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 500, cfg.NotifyBatchSize)
			assert.Equal(t, 500, cfg.NotifyQueueSize)
		}},
		{"min queue size with matching batch", "1", "10", func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 1, cfg.NotifyBatchSize)
			assert.Equal(t, 10, cfg.NotifyQueueSize)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("NOTIFY_BATCH_SIZE", tt.batch)
			t.Setenv("NOTIFY_QUEUE_SIZE", tt.queue)

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}

func TestLoad_NotifyQueueSizeLessThanBatchSize(t *testing.T) {
	setDefaults(t)
	t.Setenv("NOTIFY_BATCH_SIZE", "100")
	t.Setenv("NOTIFY_QUEUE_SIZE", "50")

	cfg, err := Load()
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ErrInvalidConfig)
	assert.Contains(t, err.Error(), "NOTIFY_QUEUE_SIZE")
	assert.Contains(t, err.Error(), "NOTIFY_BATCH_SIZE")

	t.Logf("NOTIFY_QUEUE_SIZE=50 < NOTIFY_BATCH_SIZE=100 → error: %v", err)
}

func TestLoad_NotifyQueueSizeEqualsBatchSize(t *testing.T) {
	setDefaults(t)
	t.Setenv("NOTIFY_BATCH_SIZE", "100")
	t.Setenv("NOTIFY_QUEUE_SIZE", "100")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.NotifyQueueSize)
	assert.Equal(t, 100, cfg.NotifyBatchSize)

	t.Logf("NOTIFY_QUEUE_SIZE == NOTIFY_BATCH_SIZE == 100 → ok")
}

func TestContainsControlChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"clean string", "/tmp/test.db", false},
		{"empty string", "", false},
		{"with path separators", "host=localhost dbname=test", false},
		{"printable special chars", "postgres://user:p@ss!#$/db", false},
		{"null byte", "data\x00.db", true},
		{"tab", "data\t.db", true},
		{"newline", "data\n.db", true},
		{"carriage return", "data\r.db", true},
		{"bell", "data\a.db", true},
		{"backspace", "data\b.db", true},
		{"escape", "data\x1b.db", true},
		{"DEL (0x7F)", "data\x7f.db", true},
		{"control char at start", "\x01data.db", true},
		{"control char at end", "data.db\x02", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsControlChars(tt.input)
			assert.Equal(t, tt.expected, result)

			t.Logf("containsControlChars(%q) = %v", tt.input, result)
		})
	}
}

func TestContainsDangerousSequence(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectFound bool
		expectDesc  string
	}{
		{"clean DSN", "data/alerts.db", false, ""},
		{"postgres DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable", false, ""},
		{"sqlite path", "/var/lib/app/data.db", false, ""},

		// CRLF injection
		{"CRLF", "data\r\n.db", true, "CRLF"},
		{"lone CR", "data\r.db", true, "CR"},

		// Null byte
		{"null byte", "data\x00.db", true, "null byte"},

		// ANSI escape
		{"ANSI escape", "data\x1b[31m.db", true, "ANSI escape"},

		// Shell expansion
		{"shell backtick", "$(whoami).db", true, "shell expansion"},
		{"shell dollar paren", "data`id`.db", true, "shell expansion"},

		// Template injection
		{"Go template", "data{{.Foo}}.db", true, "template injection"},

		// URL-encoded variants
		{"URL-encoded null", "data%00.db", true, "URL-encoded dangerous sequence"},
		{"URL-encoded CRLF", "data%0d%0a.db", true, "URL-encoded dangerous sequence"},
		{"URL-encoded newline", "data%0a.db", true, "URL-encoded dangerous sequence"},
		{"URL-encoded CR", "data%0d.db", true, "URL-encoded dangerous sequence"},

		// Normal percent signs should not trigger
		{"percent in password", "postgres://user:100%25safe@host/db", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, desc := containsDangerousSequence(tt.input)
			assert.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				assert.Contains(t, desc, tt.expectDesc)
			}

			t.Logf("containsDangerousSequence(%q) = (%v, %q)", tt.input, found, desc)
		})
	}
}

// --- Pachca configuration tests ---

func TestLoad_PachcaDefaults(t *testing.T) {
	setDefaults(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.Pachca.Enabled)
	assert.Equal(t, "https://api.pachca.com", cfg.Pachca.BaseURL)
	assert.Empty(t, cfg.Pachca.Token)
	assert.Equal(t, 0, cfg.Pachca.ChatID)

	t.Logf("pachca defaults: enabled=%v, base_url=%s", cfg.Pachca.Enabled, cfg.Pachca.BaseURL)
}

func TestLoad_PachcaImplicitEnable(t *testing.T) {
	setDefaults(t)
	t.Setenv("PACHCA_TOKEN", "test-token-value")
	t.Setenv("PACHCA_CHAT_ID", "12345")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.Pachca.Enabled)
	assert.Equal(t, "https://api.pachca.com", cfg.Pachca.BaseURL)
	assert.Equal(t, "test-token-value", cfg.Pachca.Token)
	assert.Equal(t, 12345, cfg.Pachca.ChatID)

	t.Logf("pachca enabled: token=***, chat_id=%d, base_url=%s",
		cfg.Pachca.ChatID, cfg.Pachca.BaseURL)
}

func TestLoad_PachcaCustomBaseURL(t *testing.T) {
	setDefaults(t)
	t.Setenv("PACHCA_TOKEN", "test-token-value")
	t.Setenv("PACHCA_CHAT_ID", "42")
	t.Setenv("PACHCA_BASE_URL", "https://custom.pachca.com")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.Pachca.Enabled)
	assert.Equal(t, "https://custom.pachca.com", cfg.Pachca.BaseURL)

	t.Logf("pachca custom base_url=%s", cfg.Pachca.BaseURL)
}

func TestLoad_PachcaDisabledNoValidation(t *testing.T) {
	setDefaults(t)
	// Token пустой → Enabled=false → ChatID=0 не вызывает ошибку
	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.Pachca.Enabled)
}

func TestLoad_InvalidPachcaChatID(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"float", "1.5"},
		{"above max", "1000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("PACHCA_TOKEN", "test-token-value")
			t.Setenv("PACHCA_CHAT_ID", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("PACHCA_CHAT_ID=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_PachcaCrossField_MissingChatID(t *testing.T) {
	setDefaults(t)
	t.Setenv("PACHCA_TOKEN", "test-token-value")
	// PACHCA_CHAT_ID не задан → Enabled=true, но ChatID=0 → ошибка

	cfg, err := Load()
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ErrInvalidConfig)
	assert.Contains(t, err.Error(), "PACHCA_CHAT_ID")

	t.Logf("cross-field: token set, chat_id missing → error: %v", err)
}

func TestLoad_PachcaCrossField_MissingToken(t *testing.T) {
	setDefaults(t)
	// Token пустой → Enabled=false → ChatID не парсится и валидация пропускается
	t.Setenv("PACHCA_CHAT_ID", "123")

	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.Pachca.Enabled)
	// ChatID парсится, но без Token канал выключен
}

func TestLoad_InvalidPachcaBaseURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"no scheme", "api.pachca.com"},
		{"ftp scheme", "ftp://api.pachca.com"},
		{"empty host", "https://"},
		{"invalid url", "://bad"},
		{"control chars", "https://api.pachca\x01.com"},
		{"shell expansion", "https://api.pachca.com/$(whoami)"},
		{"template injection", "https://api.pachca.com/{{.Path}}"},
		{"too long", "https://" + strings.Repeat("a", 2041)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("PACHCA_TOKEN", "test-token-value")
			t.Setenv("PACHCA_CHAT_ID", "123")
			t.Setenv("PACHCA_BASE_URL", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("PACHCA_BASE_URL=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidPachcaToken(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		errContains string
	}{
		{"whitespace only", "   ", "минимум"},
		{"space inside", "token value", "printable ASCII"},
		{"control chars", "token\x01value", "printable ASCII"},
		{"tab char", "token\tvalue", "printable ASCII"},
		{"unicode cyrillic", "токен-значение-длинный", "printable ASCII"},
		{"unicode emoji", "token-\xf0\x9f\x94\x91-value", "printable ASCII"},
		{"shell expansion", "$(whoami)", "опасную последовательность"},
		{"template injection", "{{.Token}}", "опасную последовательность"},
		{"too long", strings.Repeat("a", 513), "превышает максимум"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("PACHCA_TOKEN", tt.value)
			t.Setenv("PACHCA_CHAT_ID", "123")

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}

			t.Logf("PACHCA_TOKEN=%q → error: %v", "[REDACTED]", err)
		})
	}
}

func TestLoad_PachcaNormalization(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		env    string
		check  func(t *testing.T, cfg *Config)
	}{
		{
			name:   "BaseURL whitespace trimmed",
			envKey: "PACHCA_BASE_URL",
			env:    " https://api.pachca.com ",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "https://api.pachca.com", cfg.Pachca.BaseURL)
			},
		},
		{
			name:   "BaseURL trailing slash trimmed",
			envKey: "PACHCA_BASE_URL",
			env:    "https://api.pachca.com/",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "https://api.pachca.com", cfg.Pachca.BaseURL)
			},
		},
		{
			name:   "BaseURL multiple trailing slashes trimmed",
			envKey: "PACHCA_BASE_URL",
			env:    "https://api.pachca.com///",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "https://api.pachca.com", cfg.Pachca.BaseURL)
			},
		},
		{
			name:   "Token whitespace trimmed",
			envKey: "PACHCA_TOKEN",
			env:    " test-token-value ",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "test-token-value", cfg.Pachca.Token)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("PACHCA_TOKEN", "test-token-value")
			t.Setenv("PACHCA_CHAT_ID", "123")
			t.Setenv(tt.envKey, tt.env)

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)

			t.Logf("%s=%q → normalized", tt.envKey, tt.env)
		})
	}
}

func TestConfig_LogValue_Pachca(t *testing.T) {
	setDefaults(t)
	t.Setenv("PACHCA_TOKEN", "super-secret-token")
	t.Setenv("PACHCA_CHAT_ID", "42")

	cfg, err := Load()
	require.NoError(t, err)

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("test", "config", cfg)

	output := buf.String()
	t.Logf("slog output: %s", output)

	// Token must be redacted
	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "super-secret-token")
	// Other Pachca fields visible
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "api.pachca.com")
}

func TestLoad_PachcaValidEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, cfg *Config)
	}{
		{"min chat_id", map[string]string{"PACHCA_CHAT_ID": "1"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 1, cfg.Pachca.ChatID)
		}},
		{"max chat_id", map[string]string{"PACHCA_CHAT_ID": "999999999"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 999999999, cfg.Pachca.ChatID)
		}},
		{"min token length", map[string]string{"PACHCA_TOKEN": "a"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, "a", cfg.Pachca.Token)
		}},
		{"max token length", map[string]string{"PACHCA_TOKEN": strings.Repeat("a", 512)}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Len(t, cfg.Pachca.Token, 512)
		}},
		{"token printable ASCII specials", map[string]string{"PACHCA_TOKEN": "!@#$%^&*()-_=+[]|;:',.<>?/~"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.True(t, cfg.Pachca.Enabled)
		}},
		{"base_url max length boundary", map[string]string{"PACHCA_BASE_URL": "https://" + strings.Repeat("a", 2040)}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Len(t, cfg.Pachca.BaseURL, 2048)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("PACHCA_TOKEN", "test-token-value")
			t.Setenv("PACHCA_CHAT_ID", "123")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)

			t.Logf("edge case %q", tt.name)
		})
	}
}

func TestLoad_PachcaBaseURLSchemes(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{"https valid", "https://api.pachca.com", false},
		{"http valid", "http://localhost:8080", false},
		{"ftp invalid", "ftp://files.pachca.com", true},
		{"ws invalid", "ws://api.pachca.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			t.Setenv("PACHCA_TOKEN", "test-token-value")
			t.Setenv("PACHCA_CHAT_ID", "123")
			t.Setenv("PACHCA_BASE_URL", tt.baseURL)

			cfg, err := Load()
			if tt.wantErr {
				assert.Nil(t, cfg)
				assert.ErrorIs(t, err, ErrInvalidConfig)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.baseURL, cfg.Pachca.BaseURL)
			}

			t.Logf("PACHCA_BASE_URL=%q → wantErr=%v, err=%v", tt.baseURL, tt.wantErr, err)
		})
	}
}

// --- Email configuration tests ---

// setEmailDefaults sets valid Email ENV variables for tests that need Email enabled.
func setEmailDefaults(t *testing.T) {
	t.Helper()
	t.Setenv("EMAIL_SMTP_HOST", "mail.example.com")
	t.Setenv("EMAIL_FROM", "alerts@example.com")
	t.Setenv("EMAIL_TO", "oncall@example.com")
}

func TestLoad_EmailDefaults(t *testing.T) {
	setDefaults(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.Email.Enabled)
	assert.Equal(t, 587, cfg.Email.SMTPPort)
	assert.Equal(t, "starttls", cfg.Email.TLSMode)
	assert.Equal(t, "[Alert]", cfg.Email.SubjectPrefix)

	t.Logf("email defaults: enabled=%v, port=%d, tls=%s, prefix=%s",
		cfg.Email.Enabled, cfg.Email.SMTPPort, cfg.Email.TLSMode, cfg.Email.SubjectPrefix)
}

func TestLoad_EmailImplicitEnable(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.Email.Enabled)
	assert.Equal(t, "mail.example.com", cfg.Email.SMTPHost)
	assert.Equal(t, 587, cfg.Email.SMTPPort)
	assert.Equal(t, "alerts@example.com", cfg.Email.From)
	assert.Equal(t, []string{"oncall@example.com"}, cfg.Email.To)
	assert.Equal(t, "starttls", cfg.Email.TLSMode)
	assert.Equal(t, "[Alert]", cfg.Email.SubjectPrefix)

	t.Logf("email enabled: host=%s, port=%d, from=%s, to=%v",
		cfg.Email.SMTPHost, cfg.Email.SMTPPort, cfg.Email.From, cfg.Email.To)
}

func TestLoad_EmailCustomValues(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	t.Setenv("EMAIL_SMTP_PORT", "465")
	t.Setenv("EMAIL_TO", "admin@example.com, devops@example.com")
	t.Setenv("EMAIL_USERNAME", "smtp-user")
	t.Setenv("EMAIL_PASSWORD", "smtp-pass")
	t.Setenv("EMAIL_TLS_MODE", "tls")
	t.Setenv("EMAIL_SUBJECT_PREFIX", "[PROD]")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.Email.Enabled)
	assert.Equal(t, 465, cfg.Email.SMTPPort)
	assert.Equal(t, []string{"admin@example.com", "devops@example.com"}, cfg.Email.To)
	assert.Equal(t, "smtp-user", cfg.Email.Username)
	assert.Equal(t, "smtp-pass", cfg.Email.Password)
	assert.Equal(t, "tls", cfg.Email.TLSMode)
	assert.Equal(t, "[PROD]", cfg.Email.SubjectPrefix)
}

func TestLoad_EmailDisabledNoValidation(t *testing.T) {
	setDefaults(t)
	// SMTPHost пустой → Enabled=false → невалидные FROM/TO не вызывают ошибку
	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.Email.Enabled)
}

func TestLoad_EmailToMultipleRecipients(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	t.Setenv("EMAIL_TO", "a@example.com, b@example.com, c@example.com")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"a@example.com", "b@example.com", "c@example.com"}, cfg.Email.To)
}

func TestLoad_EmailToSkipsEmpty(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	t.Setenv("EMAIL_TO", "a@example.com,,  , b@example.com,")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"a@example.com", "b@example.com"}, cfg.Email.To)
}

func TestLoad_InvalidEmailSMTPPort(t *testing.T) {
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
			setEmailDefaults(t)
			t.Setenv("EMAIL_SMTP_PORT", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("EMAIL_SMTP_PORT=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidEmailFrom(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"no at sign", "not-an-email"},
		{"missing domain", "user@"},
		{"missing local", "@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			t.Setenv("EMAIL_FROM", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("EMAIL_FROM=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidEmailTo(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty string", ""},
		{"only commas", ",,"},
		{"only spaces", "   "},
		{"invalid email", "not-an-email"},
		{"one valid one invalid", "a@example.com, invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			t.Setenv("EMAIL_TO", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("EMAIL_TO=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidEmailTLSMode(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"ssl", "ssl"},
		{"auto", "auto"},
		{"unknown", "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			t.Setenv("EMAIL_TLS_MODE", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)

			t.Logf("EMAIL_TLS_MODE=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidEmailPassword(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		errContains string
	}{
		{"space inside", "pass word", "printable ASCII"},
		{"control chars", "pass\x01word", "printable ASCII"},
		{"unicode cyrillic", "пароль-длинный-пароль", "printable ASCII"},
		{"too long", strings.Repeat("a", 513), "превышает максимум"},
		{"shell expansion", "$(whoami)", "опасную последовательность"},
		{"template injection", "{{.Pass}}", "опасную последовательность"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			t.Setenv("EMAIL_USERNAME", "user")
			t.Setenv("EMAIL_PASSWORD", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}

			t.Logf("EMAIL_PASSWORD=%q → error: %v", "[REDACTED]", err)
		})
	}
}

func TestLoad_InvalidEmailSMTPHost(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		errContains string
	}{
		{"too long", strings.Repeat("a", 254), "превышает максимум"},
		{"control chars", "mail\x01.example.com", "управляющие символы"},
		{"shell expansion", "mail$(whoami).example.com", "опасную последовательность"},
		{"template injection", "mail{{.Host}}.com", "опасную последовательность"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			t.Setenv("EMAIL_SMTP_HOST", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}

			t.Logf("EMAIL_SMTP_HOST=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_InvalidEmailSubjectPrefix(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		errContains string
	}{
		{"too long", strings.Repeat("A", 129), "превышает максимум"},
		{"control chars", "[Alert\x01]", "управляющие символы"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			t.Setenv("EMAIL_SUBJECT_PREFIX", tt.value)

			cfg, err := Load()
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, ErrInvalidConfig)
			if tt.errContains != "" {
				assert.Contains(t, err.Error(), tt.errContains)
			}

			t.Logf("EMAIL_SUBJECT_PREFIX=%q → error: %v", tt.value, err)
		})
	}
}

func TestLoad_EmailCrossField_UsernameWithoutPassword(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	t.Setenv("EMAIL_USERNAME", "smtp-user")
	// EMAIL_PASSWORD пустой

	cfg, err := Load()
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ErrInvalidConfig)
	assert.Contains(t, err.Error(), "EMAIL_USERNAME")
	assert.Contains(t, err.Error(), "EMAIL_PASSWORD")

	t.Logf("cross-field: username set, password missing → error: %v", err)
}

func TestLoad_EmailCrossField_PasswordWithoutUsername(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	t.Setenv("EMAIL_PASSWORD", "smtp-pass")
	// EMAIL_USERNAME пустой

	cfg, err := Load()
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ErrInvalidConfig)
	assert.Contains(t, err.Error(), "EMAIL_USERNAME")
	assert.Contains(t, err.Error(), "EMAIL_PASSWORD")

	t.Logf("cross-field: password set, username missing → error: %v", err)
}

func TestLoad_EmailCrossField_BothAuthFieldsSet(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	t.Setenv("EMAIL_USERNAME", "smtp-user")
	t.Setenv("EMAIL_PASSWORD", "smtp-pass")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "smtp-user", cfg.Email.Username)
	assert.Equal(t, "smtp-pass", cfg.Email.Password)
}

func TestLoad_EmailCrossField_BothAuthFieldsEmpty(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	// Оба пусты — ok

	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Email.Username)
	assert.Empty(t, cfg.Email.Password)
}

func TestLoad_EmailNormalization(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "SMTPHost whitespace trimmed",
			env:  map[string]string{"EMAIL_SMTP_HOST": " mail.example.com "},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "mail.example.com", cfg.Email.SMTPHost)
			},
		},
		{
			name: "From whitespace trimmed",
			env:  map[string]string{"EMAIL_FROM": " alerts@example.com "},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "alerts@example.com", cfg.Email.From)
			},
		},
		{
			name: "TLSMode uppercase normalized to lowercase",
			env:  map[string]string{"EMAIL_TLS_MODE": "STARTTLS"},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "starttls", cfg.Email.TLSMode)
			},
		},
		{
			name: "TLSMode mixed case normalized",
			env:  map[string]string{"EMAIL_TLS_MODE": " TLS "},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "tls", cfg.Email.TLSMode)
			},
		},
		{
			name: "SubjectPrefix whitespace trimmed",
			env:  map[string]string{"EMAIL_SUBJECT_PREFIX": " [PROD] "},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "[PROD]", cfg.Email.SubjectPrefix)
			},
		},
		{
			name: "To addresses whitespace trimmed",
			env:  map[string]string{"EMAIL_TO": " a@example.com , b@example.com "},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, []string{"a@example.com", "b@example.com"}, cfg.Email.To)
			},
		},
		{
			name: "Username whitespace trimmed",
			env:  map[string]string{"EMAIL_USERNAME": " smtp-user ", "EMAIL_PASSWORD": "smtp-pass"},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "smtp-user", cfg.Email.Username)
			},
		},
		{
			name: "Password whitespace trimmed",
			env:  map[string]string{"EMAIL_USERNAME": "smtp-user", "EMAIL_PASSWORD": " smtp-pass "},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "smtp-pass", cfg.Email.Password)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)

			t.Logf("email normalization: %s", tt.name)
		})
	}
}

func TestLoad_EmailValidEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, cfg *Config)
	}{
		{"min port", map[string]string{"EMAIL_SMTP_PORT": "1"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 1, cfg.Email.SMTPPort)
		}},
		{"max port", map[string]string{"EMAIL_SMTP_PORT": "65535"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, 65535, cfg.Email.SMTPPort)
		}},
		{"tls mode starttls", map[string]string{"EMAIL_TLS_MODE": "starttls"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, "starttls", cfg.Email.TLSMode)
		}},
		{"tls mode tls", map[string]string{"EMAIL_TLS_MODE": "tls"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, "tls", cfg.Email.TLSMode)
		}},
		{"tls mode none", map[string]string{"EMAIL_TLS_MODE": "none"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, "none", cfg.Email.TLSMode)
		}},
		{"max smtp host length", map[string]string{"EMAIL_SMTP_HOST": strings.Repeat("a", 253)}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Len(t, cfg.Email.SMTPHost, 253)
		}},
		{"max password length", map[string]string{"EMAIL_USERNAME": "user", "EMAIL_PASSWORD": strings.Repeat("a", 512)}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Len(t, cfg.Email.Password, 512)
		}},
		{"max subject prefix length", map[string]string{"EMAIL_SUBJECT_PREFIX": strings.Repeat("A", 128)}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Len(t, cfg.Email.SubjectPrefix, 128)
		}},
		{"password with printable ASCII specials", map[string]string{"EMAIL_USERNAME": "user", "EMAIL_PASSWORD": "!@#$%^&*()-_=+[]|;:',.<>?/~"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.True(t, cfg.Email.Enabled)
		}},
		{"email with display name in From", map[string]string{"EMAIL_FROM": "Alert System <alerts@example.com>"}, func(t *testing.T, cfg *Config) {
			t.Helper()
			assert.Equal(t, "Alert System <alerts@example.com>", cfg.Email.From)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			require.NoError(t, err)
			tt.check(t, cfg)

			t.Logf("email edge case: %s", tt.name)
		})
	}
}

func TestLoad_EmailTLSModes(t *testing.T) {
	tests := []struct {
		name    string
		tlsMode string
		wantErr bool
	}{
		{"starttls valid", "starttls", false},
		{"tls valid", "tls", false},
		{"none valid", "none", false},
		{"ssl invalid", "ssl", true},
		{"auto invalid", "auto", true},
		{"empty uses default", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(t)
			setEmailDefaults(t)
			if tt.tlsMode != "" {
				t.Setenv("EMAIL_TLS_MODE", tt.tlsMode)
			}

			cfg, err := Load()
			if tt.wantErr {
				assert.Nil(t, cfg)
				assert.ErrorIs(t, err, ErrInvalidConfig)
			} else {
				require.NoError(t, err)
				if tt.tlsMode != "" {
					assert.Equal(t, tt.tlsMode, cfg.Email.TLSMode)
				} else {
					assert.Equal(t, "starttls", cfg.Email.TLSMode)
				}
			}

			t.Logf("EMAIL_TLS_MODE=%q → wantErr=%v, err=%v", tt.tlsMode, tt.wantErr, err)
		})
	}
}

func TestLoad_EmailMissingFrom(t *testing.T) {
	setDefaults(t)
	t.Setenv("EMAIL_SMTP_HOST", "mail.example.com")
	t.Setenv("EMAIL_TO", "oncall@example.com")
	// EMAIL_FROM не задан → ошибка

	cfg, err := Load()
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ErrInvalidConfig)
	assert.Contains(t, err.Error(), "EMAIL_FROM")
}

func TestLoad_EmailMissingTo(t *testing.T) {
	setDefaults(t)
	t.Setenv("EMAIL_SMTP_HOST", "mail.example.com")
	t.Setenv("EMAIL_FROM", "alerts@example.com")
	// EMAIL_TO не задан → ошибка

	cfg, err := Load()
	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, ErrInvalidConfig)
	assert.Contains(t, err.Error(), "EMAIL_TO")
}

func TestConfig_LogValue_Email(t *testing.T) {
	setDefaults(t)
	setEmailDefaults(t)
	t.Setenv("EMAIL_USERNAME", "smtp-user")
	t.Setenv("EMAIL_PASSWORD", "super-secret-password")

	cfg, err := Load()
	require.NoError(t, err)

	// Direct LogValue check
	val := cfg.LogValue()
	got := val.String()
	assert.Contains(t, got, "[REDACTED]")
	assert.NotContains(t, got, "super-secret-password")
	assert.Contains(t, got, "mail.example.com")

	// Through real slog
	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("test", "config", cfg)

	output := buf.String()
	t.Logf("slog output: %s", output)

	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "super-secret-password")
	assert.Contains(t, output, "mail.example.com")
	assert.Contains(t, output, "alerts@example.com")
}

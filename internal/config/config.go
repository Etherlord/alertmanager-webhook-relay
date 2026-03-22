package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	minPort            = 1
	maxPort            = 65535
	maxShutdownTimeout = 5 * time.Minute

	minMaxPayloadSize = 1024
	maxMaxPayloadSize = 10485760 // 10 MB

	minMaxAlertsPerPayload = 1
	maxMaxAlertsPerPayload = 1000

	maxDSNLen = 2048

	minNotifyPollInterval = 1 * time.Second
	maxNotifyPollInterval = 60 * time.Second

	minNotifyBatchSize = 1
	maxNotifyBatchSize = 500

	minNotifyQueueSize = 10
	maxNotifyQueueSize = 10000

	minNotifySendTimeout = 5 * time.Second
	maxNotifySendTimeout = 120 * time.Second

	maxPachcaTokenLen = 512
)

var validLogLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

var validDatabaseDrivers = map[string]struct{}{
	"sqlite": {},
}

// ErrInvalidConfig is a sentinel error for configuration validation failures.
var ErrInvalidConfig = errors.New("invalid configuration")

// PachcaConfig holds configuration for the Pachca notification channel.
type PachcaConfig struct {
	Enabled bool
	BaseURL string
	Token   string
	ChatID  int
}

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Port            int
	LogLevel        string
	ShutdownTimeout time.Duration

	DatabaseDriver      string
	DatabaseDSN         string
	MaxPayloadSize      int
	MaxAlertsPerPayload int

	NotifyPollInterval time.Duration
	NotifyBatchSize    int
	NotifyQueueSize    int
	NotifySendTimeout  time.Duration

	Pachca PachcaConfig
}

// Load reads configuration from environment variables, applies defaults,
// normalizes values, and validates constraints. Returns nil and an error
// wrapping ErrInvalidConfig on validation failure.
//
// NB: debug-логи внутри Load() используют default logger, который ещё не
// настроен (chicken-and-egg). Они станут видны только если до вызова Load()
// установить slog default на уровень Debug.
func Load() (*Config, error) {
	slog.Debug("loading configuration from environment")

	cfg := &Config{
		Port:                8080,
		LogLevel:            "info",
		ShutdownTimeout:     15 * time.Second,
		DatabaseDriver:      "sqlite",
		DatabaseDSN:         "data/alerts.db",
		MaxPayloadSize:      1048576, // 1 MB
		MaxAlertsPerPayload: 100,

		NotifyPollInterval: 5 * time.Second,
		NotifyBatchSize:    50,
		NotifyQueueSize:    100,
		NotifySendTimeout:  30 * time.Second,

		Pachca: PachcaConfig{
			BaseURL: "https://api.pachca.com",
		},
	}

	if v := os.Getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение PORT=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.Port = port
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение SHUTDOWN_TIMEOUT=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.ShutdownTimeout = d
	}

	if v := os.Getenv("DATABASE_DRIVER"); v != "" {
		cfg.DatabaseDriver = v
	}

	if v := os.Getenv("DATABASE_DSN"); v != "" {
		cfg.DatabaseDSN = v
	}

	if v := os.Getenv("MAX_PAYLOAD_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение MAX_PAYLOAD_SIZE=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.MaxPayloadSize = n
	}

	if v := os.Getenv("MAX_ALERTS_PER_PAYLOAD"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение MAX_ALERTS_PER_PAYLOAD=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.MaxAlertsPerPayload = n
	}

	if v := os.Getenv("NOTIFY_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение NOTIFY_POLL_INTERVAL=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.NotifyPollInterval = d
	}

	if v := os.Getenv("NOTIFY_BATCH_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение NOTIFY_BATCH_SIZE=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.NotifyBatchSize = n
	}

	if v := os.Getenv("NOTIFY_QUEUE_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение NOTIFY_QUEUE_SIZE=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.NotifyQueueSize = n
	}

	if v := os.Getenv("NOTIFY_SEND_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение NOTIFY_SEND_TIMEOUT=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.NotifySendTimeout = d
	}

	// Pachca channel configuration.
	if v := os.Getenv("PACHCA_TOKEN"); v != "" {
		cfg.Pachca.Token = v
		cfg.Pachca.Enabled = true
	}

	if v := os.Getenv("PACHCA_BASE_URL"); v != "" {
		cfg.Pachca.BaseURL = v
	}

	if v := os.Getenv("PACHCA_CHAT_ID"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("невалидное значение PACHCA_CHAT_ID=%q (%s): %w", v, err.Error(), ErrInvalidConfig)
		}
		cfg.Pachca.ChatID = n
	}

	cfg.normalize()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	slog.Debug("configuration loaded", "config", cfg)

	return cfg, nil
}

// normalize applies safe transformations: trim whitespace, lowercase enums.
func (c *Config) normalize() {
	if trimmed := strings.ToLower(strings.TrimSpace(c.LogLevel)); trimmed != c.LogLevel {
		slog.Debug("нормализация: LOG_LEVEL", "до", c.LogLevel, "после", trimmed)
		c.LogLevel = trimmed
	}
	if trimmed := strings.ToLower(strings.TrimSpace(c.DatabaseDriver)); trimmed != c.DatabaseDriver {
		slog.Debug("нормализация: DATABASE_DRIVER", "до", c.DatabaseDriver, "после", trimmed)
		c.DatabaseDriver = trimmed
	}
	if trimmed := strings.TrimSpace(c.DatabaseDSN); trimmed != c.DatabaseDSN {
		slog.Debug("нормализация: DATABASE_DSN", "действие", "TrimSpace")
		c.DatabaseDSN = trimmed
	}
	if trimmed := strings.TrimRight(c.Pachca.BaseURL, "/"); trimmed != c.Pachca.BaseURL {
		slog.Debug("нормализация: PACHCA_BASE_URL", "до", c.Pachca.BaseURL, "после", trimmed)
		c.Pachca.BaseURL = trimmed
	}
	if trimmed := strings.TrimSpace(c.Pachca.Token); trimmed != c.Pachca.Token {
		slog.Debug("нормализация: PACHCA_TOKEN", "действие", "TrimSpace")
		c.Pachca.Token = trimmed
	}
}

// validate checks all configuration constraints. Returns the first error found.
func (c *Config) validate() error {
	// 1. Numeric ranges
	if c.Port < minPort || c.Port > maxPort {
		return fmt.Errorf("PORT=%d вне диапазона [%d, %d]: %w", c.Port, minPort, maxPort, ErrInvalidConfig)
	}

	// 2. Enum (LogLevel)
	if _, ok := validLogLevels[c.LogLevel]; !ok {
		return fmt.Errorf("LOG_LEVEL=%q должен быть debug/info/warn/error: %w", c.LogLevel, ErrInvalidConfig)
	}

	// 3. Enum (DatabaseDriver)
	if _, ok := validDatabaseDrivers[c.DatabaseDriver]; !ok {
		return fmt.Errorf("DATABASE_DRIVER=%q должен быть sqlite: %w", c.DatabaseDriver, ErrInvalidConfig)
	}

	// 4. Arbitrary string (DatabaseDSN): empty → length → control chars → dangerous sequences
	if c.DatabaseDSN == "" {
		return fmt.Errorf("DATABASE_DSN не может быть пустым: %w", ErrInvalidConfig)
	}
	if len(c.DatabaseDSN) > maxDSNLen {
		return fmt.Errorf("DATABASE_DSN длиной %d превышает максимум %d: %w",
			len(c.DatabaseDSN), maxDSNLen, ErrInvalidConfig)
	}
	if containsControlChars(c.DatabaseDSN) {
		return fmt.Errorf("DATABASE_DSN содержит управляющие символы: %w", ErrInvalidConfig)
	}
	if found, desc := containsDangerousSequence(c.DatabaseDSN); found {
		slog.Debug("DATABASE_DSN содержит опасную последовательность", "тип", desc)
		return fmt.Errorf("DATABASE_DSN содержит опасную последовательность (%s): %w", desc, ErrInvalidConfig)
	}

	// 5. Numeric range (MaxPayloadSize)
	if c.MaxPayloadSize < minMaxPayloadSize || c.MaxPayloadSize > maxMaxPayloadSize {
		return fmt.Errorf("MAX_PAYLOAD_SIZE=%d вне диапазона [%d, %d]: %w",
			c.MaxPayloadSize, minMaxPayloadSize, maxMaxPayloadSize, ErrInvalidConfig)
	}

	// 6. Numeric range (MaxAlertsPerPayload)
	if c.MaxAlertsPerPayload < minMaxAlertsPerPayload || c.MaxAlertsPerPayload > maxMaxAlertsPerPayload {
		return fmt.Errorf("MAX_ALERTS_PER_PAYLOAD=%d вне диапазона [%d, %d]: %w",
			c.MaxAlertsPerPayload, minMaxAlertsPerPayload, maxMaxAlertsPerPayload, ErrInvalidConfig)
	}

	// 7. Duration (ShutdownTimeout)
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT=%s должен быть положительным: %w", c.ShutdownTimeout, ErrInvalidConfig)
	}
	if c.ShutdownTimeout > maxShutdownTimeout {
		return fmt.Errorf("SHUTDOWN_TIMEOUT=%s превышает максимум %s: %w", c.ShutdownTimeout, maxShutdownTimeout, ErrInvalidConfig)
	}

	// 8. Duration (NotifyPollInterval)
	if c.NotifyPollInterval < minNotifyPollInterval || c.NotifyPollInterval > maxNotifyPollInterval {
		return fmt.Errorf("NOTIFY_POLL_INTERVAL=%s вне диапазона [%s, %s]: %w",
			c.NotifyPollInterval, minNotifyPollInterval, maxNotifyPollInterval, ErrInvalidConfig)
	}

	// 9. Numeric range (NotifyBatchSize)
	if c.NotifyBatchSize < minNotifyBatchSize || c.NotifyBatchSize > maxNotifyBatchSize {
		return fmt.Errorf("NOTIFY_BATCH_SIZE=%d вне диапазона [%d, %d]: %w",
			c.NotifyBatchSize, minNotifyBatchSize, maxNotifyBatchSize, ErrInvalidConfig)
	}

	// 10. Numeric range (NotifyQueueSize)
	if c.NotifyQueueSize < minNotifyQueueSize || c.NotifyQueueSize > maxNotifyQueueSize {
		return fmt.Errorf("NOTIFY_QUEUE_SIZE=%d вне диапазона [%d, %d]: %w",
			c.NotifyQueueSize, minNotifyQueueSize, maxNotifyQueueSize, ErrInvalidConfig)
	}

	// 11. Duration (NotifySendTimeout)
	if c.NotifySendTimeout < minNotifySendTimeout || c.NotifySendTimeout > maxNotifySendTimeout {
		return fmt.Errorf("NOTIFY_SEND_TIMEOUT=%s вне диапазона [%s, %s]: %w",
			c.NotifySendTimeout, minNotifySendTimeout, maxNotifySendTimeout, ErrInvalidConfig)
	}

	// 12. Cross-field: QueueSize >= BatchSize
	if c.NotifyQueueSize < c.NotifyBatchSize {
		return fmt.Errorf("NOTIFY_QUEUE_SIZE=%d не может быть меньше NOTIFY_BATCH_SIZE=%d: %w",
			c.NotifyQueueSize, c.NotifyBatchSize, ErrInvalidConfig)
	}

	// 13. Pachca channel (skip if disabled)
	if c.Pachca.Enabled {
		if err := c.validatePachca(); err != nil {
			return err
		}
	}

	return nil
}

// validatePachca checks all Pachca channel configuration constraints.
func (c *Config) validatePachca() error {
	// Token: length
	if len(c.Pachca.Token) > maxPachcaTokenLen {
		return fmt.Errorf("PACHCA_TOKEN длиной %d превышает максимум %d: %w",
			len(c.Pachca.Token), maxPachcaTokenLen, ErrInvalidConfig)
	}

	// Token: control chars
	if containsControlChars(c.Pachca.Token) {
		return fmt.Errorf("PACHCA_TOKEN содержит управляющие символы: %w", ErrInvalidConfig)
	}

	// Token: dangerous sequences
	if found, desc := containsDangerousSequence(c.Pachca.Token); found {
		return fmt.Errorf("PACHCA_TOKEN содержит опасную последовательность (%s): %w", desc, ErrInvalidConfig)
	}

	// ChatID: must be > 0
	if c.Pachca.ChatID <= 0 {
		return fmt.Errorf("PACHCA_CHAT_ID=%d должен быть больше 0: %w", c.Pachca.ChatID, ErrInvalidConfig)
	}

	// BaseURL: parse and validate scheme
	u, err := url.Parse(c.Pachca.BaseURL)
	if err != nil {
		return fmt.Errorf("PACHCA_BASE_URL=%q невалидный URL (%s): %w", c.Pachca.BaseURL, err.Error(), ErrInvalidConfig)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("PACHCA_BASE_URL=%q должен использовать схему http или https: %w", c.Pachca.BaseURL, ErrInvalidConfig)
	}
	if u.Host == "" {
		return fmt.Errorf("PACHCA_BASE_URL=%q должен содержать хост: %w", c.Pachca.BaseURL, ErrInvalidConfig)
	}

	return nil
}

// containsControlChars reports whether s contains ASCII control characters
// (bytes < 0x20 or 0x7F).
func containsControlChars(s string) bool {
	for i := range len(s) {
		b := s[i]
		if b < 0x20 || b == 0x7F {
			return true
		}
	}
	return false
}

// dangerousPatterns maps human-readable descriptions to patterns that indicate
// injection attempts in arbitrary string values like DSN.
var dangerousPatterns = []struct {
	description string
	pattern     string
}{
	{"CRLF", "\r\n"},
	{"CR", "\r"},
	{"null byte", "\x00"},
	{"ANSI escape", "\x1b["},
	{"shell expansion", "$("},
	{"shell expansion", "`"},
	{"template injection", "{{"},
}

// urlEncodedDangerous maps URL-encoded sequences to their descriptions.
var urlEncodedDangerous = []struct {
	description string
	pattern     string
}{
	{"URL-encoded dangerous sequence", "%00"},
	{"URL-encoded dangerous sequence", "%0d%0a"},
	{"URL-encoded dangerous sequence", "%0a"},
	{"URL-encoded dangerous sequence", "%0d"},
}

// containsDangerousSequence checks s for known dangerous byte sequences
// including raw and URL-encoded variants. Returns true and a human-readable
// description if found.
func containsDangerousSequence(s string) (found bool, description string) {
	for _, p := range dangerousPatterns {
		if strings.Contains(s, p.pattern) {
			return true, p.description
		}
	}
	lower := strings.ToLower(s)
	for _, p := range urlEncodedDangerous {
		if strings.Contains(lower, p.pattern) {
			return true, p.description
		}
	}
	return false, ""
}

// SlogLevel converts the LogLevel string to a slog.Level.
func (c *Config) SlogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

//nolint:gocritic // value receiver для slog.LogValuer
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("port", c.Port),
		slog.String("log_level", c.LogLevel),
		slog.String("shutdown_timeout", c.ShutdownTimeout.String()),
		slog.String("database_driver", c.DatabaseDriver),
		slog.String("database_dsn", "[REDACTED]"),
		slog.Int("max_payload_size", c.MaxPayloadSize),
		slog.Int("max_alerts_per_payload", c.MaxAlertsPerPayload),
		slog.String("notify_poll_interval", c.NotifyPollInterval.String()),
		slog.Int("notify_batch_size", c.NotifyBatchSize),
		slog.Int("notify_queue_size", c.NotifyQueueSize),
		slog.String("notify_send_timeout", c.NotifySendTimeout.String()),
		slog.Group("pachca",
			slog.Bool("enabled", c.Pachca.Enabled),
			slog.String("base_url", c.Pachca.BaseURL),
			slog.String("token", "[REDACTED]"),
			slog.Int("chat_id", c.Pachca.ChatID),
		),
	)
}

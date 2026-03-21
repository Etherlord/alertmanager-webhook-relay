package config

import (
	"errors"
	"fmt"
	"log/slog"
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

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Port            int
	LogLevel        string
	ShutdownTimeout time.Duration

	DatabaseDriver      string
	DatabaseDSN         string
	MaxPayloadSize      int
	MaxAlertsPerPayload int
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
		slog.Debug("нормализация: DATABASE_DSN", "до", c.DatabaseDSN, "после", trimmed)
		c.DatabaseDSN = trimmed
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

	// 4. Non-empty string (DatabaseDSN)
	if c.DatabaseDSN == "" {
		return fmt.Errorf("DATABASE_DSN не может быть пустым: %w", ErrInvalidConfig)
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

	return nil
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
		slog.String("database_dsn", c.DatabaseDSN),
		slog.Int("max_payload_size", c.MaxPayloadSize),
		slog.Int("max_alerts_per_payload", c.MaxAlertsPerPayload),
	)
}

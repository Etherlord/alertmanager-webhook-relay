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
)

var validLogLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

// ErrInvalidConfig is a sentinel error for configuration validation failures.
var ErrInvalidConfig = errors.New("invalid configuration")

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Port            int
	LogLevel        string
	ShutdownTimeout time.Duration
}

// Load reads configuration from environment variables, applies defaults,
// normalizes values, and validates constraints. Returns nil and an error
// wrapping ErrInvalidConfig on validation failure.
func Load() (*Config, error) {
	slog.Debug("loading configuration from environment")

	cfg := &Config{
		Port:            8080,
		LogLevel:        "info",
		ShutdownTimeout: 15 * time.Second,
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

	// 3. Duration (ShutdownTimeout)
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
	)
}

package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ReturnsNonNil(t *testing.T) {
	logger := New(slog.LevelInfo)
	assert.NotNil(t, logger)
	t.Log("New() returned non-nil logger")
}

func TestNew_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(slog.LevelInfo, &buf)

	logger.Info("test message")

	var entry map[string]any
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err, "output must be valid JSON")

	t.Logf("JSON output: %s", buf.String())
}

func TestNew_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(slog.LevelWarn, &buf)

	logger.Info("should be filtered")

	assert.Empty(t, buf.String(), "info message should be filtered at warn level")
	t.Log("info message correctly filtered at warn level")
}

func TestNew_IncludesLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(slog.LevelInfo, &buf)

	logger.Info("test")

	var entry map[string]any
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Contains(t, entry, "level")

	t.Logf("level field: %v", entry["level"])
}

func TestNew_IncludesTimestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(slog.LevelInfo, &buf)

	logger.Info("test")

	var entry map[string]any
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Contains(t, entry, "time")

	t.Logf("time field: %v", entry["time"])
}

func TestNew_IncludesMessage(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(slog.LevelInfo, &buf)

	logger.Info("hello world")

	var entry map[string]any
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Equal(t, "hello world", entry["msg"])

	t.Logf("msg field: %v", entry["msg"])
}

package xlog

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_withoutOptions(t *testing.T) {
	log, err := New()
	require.EqualError(t, err, "xlog: missing slog.Handler factory")
	require.Nil(t, log)
}

func TestNew_defaults(t *testing.T) {
	log, err := New(AsText())
	require.NoError(t, err)
	require.NotNil(t, log)

	_, ok := log.(*discard)
	assert.False(t, ok, "default logger should not be a discard logger")
}

func TestNew_errorOption(t *testing.T) {
	log, err := New(func(o *options) error {
		return errors.New("mock error")
	})
	require.EqualError(t, err, "mock error")
	require.Nil(t, log)
}

func TestNew_withDiscard(t *testing.T) {
	log, err := New(Discard())
	require.NoError(t, err)
	require.NotNil(t, log)

	// Should return a discard logger
	_, ok := log.(*discard)
	assert.True(t, ok, "should return a discard logger")
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelDebug),
		MockClock(time.Unix(1650000000, 0).UTC()),
	)
	require.NoError(t, err)

	log.Debug("debug message", String("key", "value"))

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "key=value")
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelInfo),
		MockClock(time.Unix(1650000000, 0).UTC()),
	)
	require.NoError(t, err)

	log.Info("info message", String("status", "ok"))

	output := buf.String()
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "status=ok")
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelWarn),
		MockClock(time.Unix(1650000000, 0).UTC()),
	)
	require.NoError(t, err)

	log.Warn("warning message", Int("count", 5))

	output := buf.String()
	assert.Contains(t, output, "WARN")
	assert.Contains(t, output, "warning message")
	assert.Contains(t, output, "count=5")
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelError),
		MockClock(time.Unix(1650000000, 0).UTC()),
	)
	require.NoError(t, err)

	log.Error("error message", Bool("critical", true))

	output := buf.String()
	assert.Contains(t, output, "ERROR")
	assert.Contains(t, output, "error message")
	assert.Contains(t, output, "critical=true")
}

func TestLogger_levelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelWarn), // Only WARN and above
	)
	require.NoError(t, err)

	log.Debug("should not appear")
	log.Info("should not appear")
	log.Warn("should appear")

	output := buf.String()
	assert.NotContains(t, output, "DEBUG")
	assert.NotContains(t, output, "INFO")
	assert.Contains(t, output, "WARN")
	assert.Contains(t, output, "should appear")
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelInfo),
		MockClock(time.Unix(1650000000, 0).UTC()),
	)
	require.NoError(t, err)

	// Create a child logger with additional attributes
	child := log.With(String("component", "test"), Int("pid", 123))

	child.Info("message from child")

	output := buf.String()
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "message from child")
	assert.Contains(t, output, "component=test")
	assert.Contains(t, output, "pid=123")
}

func TestLogger_With_multipleAttrs(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelInfo),
	)
	require.NoError(t, err)

	// Chain multiple With calls
	child := log.With(String("component", "test"))
	grandchild := child.With(String("subcomponent", "sub"))

	grandchild.Info("nested message")

	output := buf.String()
	assert.Contains(t, output, "component=test")
	assert.Contains(t, output, "subcomponent=sub")
	assert.Contains(t, output, "nested message")
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
		hasError bool
	}{
		{"dbg", slog.LevelDebug, false},
		{"debug", slog.LevelDebug, false},
		{"DEBUG", slog.LevelDebug, false},

		{"", slog.LevelInfo, false},
		{"info", slog.LevelInfo, false},
		{"INFO", slog.LevelInfo, false},

		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"WARN", slog.LevelWarn, false},

		{"err", slog.LevelError, false},
		{"error", slog.LevelError, false},
		{"fatal", slog.LevelError, false},
		{"ERROR", slog.LevelError, false},

		{"invalid", slog.LevelInfo, true},
		{"trace", slog.LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)

			if tt.hasError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unknown log level")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level)
			}
		})
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsJSON(),
		WriteTo(&buf),
		Leveled(slog.LevelInfo),
	)
	require.NoError(t, err)

	log.Info("json test", String("format", "json"))

	output := buf.String()
	// Should be valid JSON
	assert.Contains(t, output, `"level":"INFO"`)
	assert.Contains(t, output, `"msg":"json test"`)
	assert.Contains(t, output, `"format":"json"`)
}

func TestLogger_multipleAttributes(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelInfo),
	)
	require.NoError(t, err)

	log.Info("test",
		String("str", "value"),
		Int("num", 42),
		Bool("flag", true),
		Duration("elapsed", time.Second),
	)

	output := buf.String()
	assert.Contains(t, output, "str=value")
	assert.Contains(t, output, "num=42")
	assert.Contains(t, output, "flag=true")
	assert.Contains(t, output, "elapsed=1s")
}

func TestNewDiscard(t *testing.T) {
	log := NewDiscard()
	require.NotNil(t, log)

	// Verify it's actually a discard logger
	_, ok := log.(*discard)
	assert.True(t, ok)

	// Should be safe to call all methods (they do nothing)
	log.Debug("should not panic")
	log.Info("should not panic")
	log.Warn("should not panic")
	log.Error("should not panic")

	// With should return itself
	child := log.With(String("key", "value"))
	assert.Equal(t, log, child)
}

func TestLogger_emptyMessage(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelInfo),
	)
	require.NoError(t, err)

	log.Info("", String("key", "value"))

	output := buf.String()
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "key=value")
}

func TestLogger_noAttributes(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(
		AsText(),
		WriteTo(&buf),
		Leveled(slog.LevelInfo),
	)
	require.NoError(t, err)

	log.Info("simple message")

	output := buf.String()
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "simple message")
	// Should not have any extra attributes beyond the message
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 1, len(lines), "should be a single line")
}

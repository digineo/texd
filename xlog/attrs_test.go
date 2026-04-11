package xlog

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorValue(t *testing.T) {
	err := errors.New("test error")
	ev := ErrorValue{err}

	assert.Equal(t, slog.StringValue("test error"), ev.Value())
	assert.Equal(t, slog.StringValue("test error"), ev.LogValue())
}

func TestError(t *testing.T) {
	err := errors.New("something went wrong")
	attr := Error(err)

	assert.Equal(t, ErrorKey, attr.Key)
	assert.Equal(t, "error", attr.Key)
	assert.Equal(t, slog.AnyValue(ErrorValue{err}), attr.Value)

	ev := attr.Value.Any().(ErrorValue)
	assert.Equal(t, "something went wrong", ev.Error())
}

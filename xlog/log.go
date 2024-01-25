// Package xlog provides a very thin (and maybe leaky)
// wrapper aroud log/slog.
package xlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Fatal(msg string, args ...any)

	With(args ...any) Logger
}

type logger struct {
	l       *slog.Logger
	context []any
}

type LoggerType int

const (
	TypeText LoggerType = iota // maps to slog.TextHandler
	TypeJSON                   // maps to slog.JSONHandler
	TypeNop                    // discards log records
)

func New(typ LoggerType, w io.Writer, o *slog.HandlerOptions) (Logger, error) {
	var h slog.Handler
	switch typ {
	case TypeText:
		h = slog.NewTextHandler(w, o)
	case TypeJSON:
		h = slog.NewTextHandler(w, o)
	case TypeNop:
		return &nop{}, nil
	default:
		return nil, fmt.Errorf("unknown LoggerType: %#v", typ)
	}

	return &logger{
		l: slog.New(h),
	}, nil
}

// log creates a log record. It is called by Debug, Info, etc.
func (log *logger) log(level slog.Level, msg string, args ...any) {
	if !log.l.Enabled(context.Background(), level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip runtime.Callers, log, and our caller
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(log.context...)
	r.Add(args...)
	_ = log.l.Handler().Handle(context.Background(), r)
}

func (log *logger) Debug(msg string, args ...any) {
	log.log(slog.LevelDebug, msg, args...)
}

func (log *logger) Info(msg string, args ...any) {
	log.log(slog.LevelInfo, msg, args...)
}

func (log *logger) Warn(msg string, args ...any) {
	log.log(slog.LevelWarn, msg, args...)
}

func (log *logger) Error(msg string, args ...any) {
	log.log(slog.LevelError, msg, args...)
}

// Fatal is the same as Error, but quits the program via os.Exit(1).
func (log *logger) Fatal(msg string, args ...any) {
	log.log(slog.LevelError, msg, args...)
	os.Exit(1)
}

func (log *logger) With(args ...any) Logger {
	context := make([]any, 0, len(log.context)+len(args))
	context = append(context, log.context...)
	context = append(context, args...)

	return &logger{
		l:       log.l,
		context: context,
	}
}

func ParseLevel(s string) (l slog.Level, err error) {
	switch s {
	case "debug", "DEBUG":
		l = slog.LevelDebug
	case "info", "INFO", "": // make the zero value useful
		l = slog.LevelInfo
	case "warn", "WARN":
		l = slog.LevelWarn
	case "error", "ERROR",
		"fatal", "FATAL":
		l = slog.LevelError
	default:
		err = fmt.Errorf("unknown log level: %q", s)
	}
	return
}

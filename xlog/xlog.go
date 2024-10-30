// Package xlog provides a thin wrapper around [log/slog].
package xlog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/digineo/texd/internal"
)

// A Logger allows writing messages with various severities.
type Logger interface {
	// Debug writes log messages with DEBUG severity.
	Debug(msg string, a ...slog.Attr)
	// Info writes log messages with INFO severity.
	Info(msg string, a ...slog.Attr)
	// Warn writes log messages with WARN severity.
	Warn(msg string, a ...slog.Attr)
	// Error writes log messages with ERROR severity.
	Error(msg string, a ...slog.Attr)
	// Fatal writes log messages with ERROR severity, and then
	// exits the whole program.
	Fatal(msg string, a ...slog.Attr)
	// With returns a Logger that includes the given attributes
	// in each output operation.
	With(a ...slog.Attr) Logger
}

type logger struct {
	l *slog.Logger
}

// New creates a new logger instance. By default, log messages are
// written to stdout, and the log level is INFO.
func New(opt ...Option) (Logger, error) {
	opts := options{
		level:  slog.LevelInfo,
		output: os.Stdout,
	}

	for _, o := range opt {
		if err := o(&opts); err != nil {
			return nil, err
		}
	}

	// the discard logger doesn't require any further setup
	if opts.discard {
		return &discard{}, nil
	}

	// setup mock time
	h := opts.buildHandler(&opts)
	if opts.clock != nil {
		h = &mockTimeHandler{
			clock:   opts.clock,
			Handler: h,
		}
	}

	return &logger{l: slog.New(h)}, nil
}

// log creates a log record. It is called by Debug, Info, etc.
func (log *logger) log(level slog.Level, msg string, a ...slog.Attr) {
	if !log.l.Enabled(context.Background(), level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) //nolint:mnd // skip runtime.Callers, log, and our caller
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.AddAttrs(a...)
	_ = log.l.Handler().Handle(context.Background(), r)
}

func (log *logger) Debug(msg string, a ...slog.Attr) {
	log.log(slog.LevelDebug, msg, a...)
}

func (log *logger) Info(msg string, a ...slog.Attr) {
	log.log(slog.LevelInfo, msg, a...)
}

func (log *logger) Warn(msg string, a ...slog.Attr) {
	log.log(slog.LevelWarn, msg, a...)
}

func (log *logger) Error(msg string, a ...slog.Attr) {
	log.log(slog.LevelError, msg, a...)
}

// Fatal is the same as Error, but quits the program via os.Exit(1).
func (log *logger) Fatal(msg string, a ...slog.Attr) {
	log.log(slog.LevelError, msg, a...)
	os.Exit(1)
}

func (log *logger) With(a ...slog.Attr) Logger {
	return &logger{
		l: slog.New(log.l.Handler().WithAttrs(a)),
	}
}

// ParseLevel tries to convert a (case-insensitive) string into a
// slog.Level. Accepted values are "debug", "info", "warn", "warning",
// "error" and "fatal". Other input will result in err not being nil.
func ParseLevel(s string) (l slog.Level, err error) {
	switch strings.ToLower(s) {
	case "debug":
		l = slog.LevelDebug
	case "info", "": // make the zero value useful
		l = slog.LevelInfo
	case "warn", "warning":
		l = slog.LevelWarn
	case "error", "fatal":
		l = slog.LevelError
	default:
		err = fmt.Errorf("unknown log level: %q", s)
	}
	return
}

type mockTimeHandler struct {
	clock internal.Clock
	slog.Handler
}

func (h *mockTimeHandler) Handle(ctx context.Context, a slog.Record) error {
	a.Time = h.clock.Now()
	return h.Handler.Handle(ctx, a)
}

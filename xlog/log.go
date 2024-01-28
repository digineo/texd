// Package xlog provides a very thin wrapper aroud log/slog.
package xlog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// A Logger allows writing messages with various severities.
type Logger interface {
	// Debug writes log messages with DEBUG severity.
	Debug(msg string, args ...any)
	// Info writes log messages with INFO severity.
	Info(msg string, args ...any)
	// Warn writes log messages with WARN severity.
	Warn(msg string, args ...any)
	// Error writes log messages with ERROR severity.
	Error(msg string, args ...any)
	// Fatal writes log messages with ERROR severity, and then
	// exits the whole program.
	Fatal(msg string, args ...any)
	// With creates a child logger, and adds the given arguments
	// to each child message output
	With(args ...any) Logger
}

type logger struct {
	l *slog.Logger
	// context holds the arguments received from With().
	context []any
}

func New(opt ...Option) (Logger, error) {
	opts := options{
		handlerOpts: &slog.HandlerOptions{},
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
	if opts.clock != nil {
		repl := opts.handlerOpts.ReplaceAttr
		opts.handlerOpts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				a.Value = slog.TimeValue(opts.clock.Now())
			}
			if repl == nil {
				return a
			}
			return repl(groups, a)
		}
	}

	return &logger{
		l: slog.New(opts.buildHandler(&opts)),
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

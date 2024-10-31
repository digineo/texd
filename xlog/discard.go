package xlog

import (
	"context"
	"log/slog"
	"os"
)

type discard struct{}

// NewDiscard produces basically the same logger as
//
//	xlog.New(xlog.Discard(), otheroptions...)
//
// but without the option overhead.
func NewDiscard() Logger {
	return &discard{}
}

var (
	_ Logger       = (*discard)(nil)
	_ slog.Handler = (*discard)(nil)
)

func (*discard) Debug(string, ...slog.Attr) {}
func (*discard) Info(string, ...slog.Attr)  {}
func (*discard) Warn(string, ...slog.Attr)  {}
func (*discard) Error(string, ...slog.Attr) {}
func (*discard) Fatal(string, ...slog.Attr) { os.Exit(1) }
func (d *discard) With(...slog.Attr) Logger { return d }

func (*discard) Enabled(context.Context, slog.Level) bool   { return false }
func (*discard) Handle(context.Context, slog.Record) error  { return nil }
func (d *discard) WithAttrs(attrs []slog.Attr) slog.Handler { return d }
func (d *discard) WithGroup(name string) slog.Handler       { return d }

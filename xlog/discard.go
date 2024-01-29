package xlog

import (
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

func (*discard) Debug(string, ...slog.Attr) {}
func (*discard) Info(string, ...slog.Attr)  {}
func (*discard) Warn(string, ...slog.Attr)  {}
func (*discard) Error(string, ...slog.Attr) {}
func (*discard) Fatal(string, ...slog.Attr) { os.Exit(1) }
func (d *discard) With(...slog.Attr) Logger { return d }

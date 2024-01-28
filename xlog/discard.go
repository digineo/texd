package xlog

import "os"

type discard struct{}

// NewDiscard produces basically the same logger as
//
//	xlog.New(xlog.Discard(), otheroptions...)
//
// but without the option overhead.
func NewDiscard() Logger {
	return &discard{}
}

func (*discard) Debug(msg string, args ...any) {}
func (*discard) Info(msg string, args ...any)  {}
func (*discard) Warn(msg string, args ...any)  {}
func (*discard) Error(msg string, args ...any) {}
func (*discard) Fatal(msg string, args ...any) { os.Exit(1) }
func (d *discard) With(args ...any) Logger     { return d }

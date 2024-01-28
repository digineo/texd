package xlog

import (
	"bytes"
	"io"
	"log/slog"
	"time"

	"github.com/digineo/texd/internal"
)

type Option func(*options) error

type options struct {
	discard      bool
	output       io.Writer
	clock        internal.Clock
	buildHandler func(o *options) slog.Handler
	handlerOpts  *slog.HandlerOptions
}

func Leveled(l slog.Level) Option {
	return func(o *options) error {
		o.handlerOpts.Level = l
		return nil
	}
}

func LeveledString(s string) Option {
	return func(o *options) error {
		l, err := ParseLevel(s)
		if err != nil {
			return err
		}
		o.handlerOpts.Level = l
		return nil
	}
}

func WriteTo(w io.Writer) Option {
	return func(o *options) error {
		o.output = w
		return nil
	}
}

func CaptureOutput() (Option, *bytes.Buffer) {
	var b bytes.Buffer
	return func(o *options) error {
		o.output = &b
		return nil
	}, &b
}

func MockClock(t time.Time) Option {
	return func(o *options) error {
		o.clock = internal.MockClock(t)
		return nil
	}
}

func WithSource() Option {
	return func(o *options) error {
		o.handlerOpts.AddSource = true
		return nil
	}
}

func WithAttrReplacer(f func(groups []string, a slog.Attr) slog.Attr) Option {
	return func(o *options) error {
		o.handlerOpts.ReplaceAttr = f
		return nil
	}
}

func AsJSON() Option {
	return func(o *options) error {
		o.buildHandler = func(o *options) slog.Handler {
			return slog.NewJSONHandler(o.output, o.handlerOpts)
		}
		return nil
	}
}

func AsText() Option {
	return func(o *options) error {
		o.buildHandler = func(o *options) slog.Handler {
			return slog.NewTextHandler(o.output, o.handlerOpts)
		}
		return nil
	}
}

func Discard() Option {
	return func(o *options) error {
		o.discard = true
		return nil
	}
}

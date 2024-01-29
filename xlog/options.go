package xlog

import (
	"io"
	"log/slog"
	"time"

	"github.com/digineo/texd/internal"
)

// An Option represents a functional configuration option. These are
// used to configure new logger instances.
type Option func(*options) error

type options struct {
	discard      bool
	output       io.Writer
	clock        internal.Clock
	buildHandler func(o *options) slog.Handler
	handlerOpts  *slog.HandlerOptions
}

// Leveled sets the log level.
func Leveled(l slog.Level) Option {
	return func(o *options) error {
		o.handlerOpts.Level = l
		return nil
	}
}

// LeveledString interprets s (see ParseLevel) and sets the log level.
// If s is unknown, the error will be revealed with New().
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

// WriteTo sets the output.
func WriteTo(w io.Writer) Option {
	return func(o *options) error {
		o.output = w
		return nil
	}
}

// MockClock sets up a canned timestamp.
func MockClock(t time.Time) Option {
	return func(o *options) error {
		o.clock = internal.MockClock(t)
		return nil
	}
}

// WithSource enables source code positions in log messages.
func WithSource() Option {
	return func(o *options) error {
		o.handlerOpts.AddSource = true
		return nil
	}
}

// WithAttrReplacer configures an attribute replacer.
// See (slog.HandlerOptions).ReplaceAtrr for details.
func WithAttrReplacer(f func(groups []string, a slog.Attr) slog.Attr) Option {
	return func(o *options) error {
		o.handlerOpts.ReplaceAttr = f
		return nil
	}
}

// AsJSON configures a JSONHandler, i.e. log messages will be printed
// as JSON string.
func AsJSON() Option {
	return func(o *options) error {
		o.buildHandler = func(o *options) slog.Handler {
			return slog.NewJSONHandler(o.output, o.handlerOpts)
		}
		return nil
	}
}

// AsText configures a TextHandler, i.e. the message output is a
// simple list of key=value pairs with minimal quoting.
func AsText() Option {
	return func(o *options) error {
		o.buildHandler = func(o *options) slog.Handler {
			return slog.NewTextHandler(o.output, o.handlerOpts)
		}
		return nil
	}
}

// Discard mutes the logger. See also NewDiscard() for a simpler
// constructor.
func Discard() Option {
	return func(o *options) error {
		o.discard = true
		return nil
	}
}

package xlog

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/digineo/texd/internal"
	"github.com/mattn/go-isatty"
	"gitlab.com/greyxor/slogor"
)

// An Option represents a functional configuration option. These are
// used to configure new logger instances.
type Option func(*options) error

type options struct {
	output io.Writer
	color  bool
	clock  internal.Clock
	level  slog.Leveler
	source bool

	buildHandler func(o *options) slog.Handler
}

// Leveled sets the log level.
func Leveled(l slog.Level) Option {
	return func(o *options) error {
		o.level = l
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
		o.level = l
		return nil
	}
}

var ErrNilWriter = errors.New("invalid writer: nil")

// WriteTo sets the output.
func WriteTo(w io.Writer) Option {
	return func(o *options) error {
		if w == nil {
			return ErrNilWriter
		}

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
		o.source = true
		return nil
	}
}

// Color enables colorful log output. If the output writer set by
// [WriteTo] isn't a TTY, or messages are output [AsJSON], enabling
// colors won't have an effect.
func Color() Option {
	return func(o *options) error {
		o.color = true
		return nil
	}
}

// AsJSON configures a JSONHandler, i.e. log messages will be printed
// as JSON string.
func AsJSON() Option {
	return func(o *options) error {
		o.buildHandler = func(o *options) slog.Handler {
			return slog.NewJSONHandler(o.output, &slog.HandlerOptions{
				AddSource: o.source,
				Level:     o.level,
			})
		}
		return nil
	}
}

// AsText configures a TextHandler, i.e. the message output is a
// simple list of key=value pairs with minimal quoting.
func AsText() Option {
	return func(o *options) error {
		o.buildHandler = func(o *options) slog.Handler {
			opts := []slogor.OptionFn{
				slogor.SetLevel(o.level.Level()),
				slogor.SetTimeFormat("[15:04:05.000]"),
			}
			if o.source {
				opts = append(opts, slogor.ShowSource())
			}
			if f, isFile := o.output.(*os.File); !isFile || !isatty.IsTerminal(f.Fd()) {
				opts = append(opts, slogor.DisableColor())
			}
			return slogor.NewHandler(o.output, opts...)
		}
		return nil
	}
}

// Discard mutes the logger. See also NewDiscard() for a simpler
// constructor.
func Discard() Option {
	return func(o *options) error {
		o.buildHandler = func(o *options) slog.Handler {
			// the discard logger doesn't have any options
			return &discard{}
		}
		return nil
	}
}

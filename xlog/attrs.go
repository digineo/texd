package xlog

import "log/slog"

// Convenience slog.Attr generators. Allows cluttering imports (no need
// to import both log/slog and xlog).
var (
	String   = slog.String
	Int64    = slog.Int64
	Int      = slog.Int
	Uint64   = slog.Uint64
	Float64  = slog.Float64
	Bool     = slog.Bool
	Time     = slog.Time
	Group    = slog.Group
	Duration = slog.Duration
	Any      = slog.Any
)

// Key used to denote error values.
const ErrorKey = "error"

// ErrorValue holds an error value.
type ErrorValue struct{ error }

var _ slog.LogValuer = (*ErrorValue)(nil)

// Value extracts the error message.
func (err ErrorValue) Value() slog.Value {
	return slog.StringValue(err.Error())
}

// LogValue implements [slog.LogValuer].
func (err ErrorValue) LogValue() slog.Value {
	return err.Value()
}

// Error constructs a first-class error log attribute.
//
// Not to be confused with (xlog.Logger).Error() or (log/slog).Error(),
// which produce an error-level log message.
func Error(err error) slog.Attr {
	return Any(ErrorKey, ErrorValue{err})
}

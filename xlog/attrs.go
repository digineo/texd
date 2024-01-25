package xlog

import "log/slog"

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

const ErrorKey = "error"

type ErrorValue struct{ error }

func (err ErrorValue) Value() slog.Value {
	return slog.StringValue(err.Error())
}

func Error(err error) slog.Attr {
	return slog.Any(ErrorKey, ErrorValue{err})
}

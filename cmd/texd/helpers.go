package main

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/digineo/texd"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// printVersion prints version information to the given writer.
func printVersion(w io.Writer) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	puts := func(s string) { _, _ = fmt.Fprintln(w, s) }
	putsf := func(s string, args ...any) { _, _ = fmt.Fprintf(w, s, args...) }

	putsf("\nGo: %s, %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	const l = "  %-10s %-50s %s\n"
	puts("Dependencies:")
	putsf(l, "main", info.Main.Path, texd.Version())
	for _, i := range info.Deps {
		if r := i.Replace; r == nil {
			putsf(l, "dep", i.Path, i.Version)
		} else {
			putsf(l, "dep", r.Path, r.Version)
			putsf(l, "  replaces", i.Path, i.Version)
		}
	}
}

// setupLogger creates and configures a logger with the given level.
func setupLogger(level string, development bool) (*zap.Logger, func(), error) {
	var cfg zap.Config
	if development {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	lvl, lvlErr := zapcore.ParseLevel(level)
	if lvlErr == nil {
		cfg.Level = zap.NewAtomicLevelAt(lvl)
	}

	log, err := cfg.Build()
	if err != nil {
		return nil, nil, err
	}

	if lvlErr != nil {
		log.Error("error parsing log level",
			zap.String("flag", "--log-level"),
			zap.Error(lvlErr))
	}

	zap.ReplaceGlobals(log)
	return log, func() {
		_ = log.Sync()
	}, nil
}

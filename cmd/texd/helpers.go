package main

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/digineo/texd"
	"github.com/digineo/texd/xlog"
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
func setupLogger(level string, development bool) (xlog.Logger, func(), error) {
	opts := []xlog.Option{
		xlog.LeveledString(level),
	}
	if development {
		opts = append(opts, xlog.AsText(), xlog.Color())
	} else {
		opts = append(opts, xlog.AsJSON())
	}

	log, err := xlog.New(opts...)
	if err != nil {
		return nil, nil, err
	}

	return log, func() {}, nil
}

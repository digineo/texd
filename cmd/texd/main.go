package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/digineo/texd"
	"github.com/digineo/texd/service"
	"github.com/digineo/texd/xlog"
)

const (
	exitSuccess = 0
	exitFlagErr = 2
)

func main() {
	exitCode, err := run(os.Args, os.Stdout, os.Stderr)
	if err != nil {
		// Don't print error for help/version requests
		if !errors.Is(err, errHelpRequested) && !errors.Is(err, errVersionRequested) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
	os.Exit(exitCode)
}

// run is the main application logic, separated from main() for testability.
func run(args []string, stdout, stderr io.Writer) (int, error) {
	// Print banner
	texd.PrintBanner(stdout)

	// Parse flags
	cfg, err := parseFlags(args[0], args[1:], stderr)
	if err != nil {
		if errors.Is(err, errHelpRequested) {
			return exitSuccess, errHelpRequested
		}
		if errors.Is(err, errVersionRequested) {
			printVersion(stdout)
			return exitSuccess, errVersionRequested
		}
		_, _ = fmt.Fprintf(stderr, "Error parsing flags:\n\t%v\n", err)
		return exitFlagErr, err
	}

	// Setup logger
	log, sync, err := setupLogger(cfg.logLevel, texd.Development())
	if err != nil {
		return exitFlagErr, fmt.Errorf("failed to setup logger: %w", err)
	}
	defer sync()

	// Configure TeX package
	if err := configureTeX(cfg, log); err != nil {
		return exitFlagErr, err
	}

	// Build service options
	opts, err := buildServiceOptions(cfg, log)
	if err != nil {
		return exitFlagErr, err
	}

	// Start service
	stop, err := service.Start(opts, log)
	if err != nil {
		log.Fatal("failed to start service", xlog.Error(err))
		return exitFlagErr, err
	}

	// Wait for shutdown signal
	handleGracefulShutdown(log, stop)
	return exitSuccess, nil
}

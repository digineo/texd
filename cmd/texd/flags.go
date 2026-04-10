package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/digineo/texd/refstore"
	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/urfave/cli/v3"
)

// Sentinel errors for special flag cases.
var (
	errHelpRequested    = errors.New("help requested")
	errVersionRequested = errors.New("version requested")
)

// parseFlags parses command-line flags and returns a config.
// Returns errHelpRequested or errVersionRequested for special cases.
func parseFlags(progname string, args []string, stderr io.Writer) (*config, error) {
	cfg := defaultConfig()

	// Temporary variables for flag parsing
	var shellEscape, noShellEscape bool

	// Check if help was requested (manual handling for clean exit)
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			app := buildApp(progname, stderr, cfg, &shellEscape, &noShellEscape, nil)
			_ = cli.ShowRootCommandHelp(app)
			return nil, errHelpRequested
		}
	}

	// Build and run the app
	app := buildApp(progname, stderr, cfg, &shellEscape, &noShellEscape, func(ctx context.Context, cmd *cli.Command) error {
		// Handle shell escape flags
		if shellEscape && noShellEscape {
			return fmt.Errorf("flags --shell-escape and --no-shell-escape are mutually exclusive")
		} else if shellEscape {
			cfg.shellEscape = 1
		} else if noShellEscape {
			cfg.shellEscape = -1
		}

		// Check version flag
		if cfg.showVersion {
			return errVersionRequested
		}

		// Remaining args are Docker images
		cfg.images = cmd.Args().Slice()

		return nil
	})

	// Run the app
	if err := app.Run(context.Background(), append([]string{progname}, args...)); err != nil {
		if errors.Is(err, errVersionRequested) {
			return nil, errVersionRequested
		}
		return nil, err
	}

	return cfg, nil
}

// buildApp constructs the CLI application with all flags.
func buildApp(progname string, stderr io.Writer, cfg *config, shellEscape, noShellEscape *bool, action cli.ActionFunc) *cli.Command { //nolint: funlen
	const (
		catServer   = "Server Options:"
		catTeX      = "TeX Options:"
		catDocker   = "Docker Options:"
		catRefStore = "Reference Store Options:"
		catMisc     = "Miscellaneous:"
	)

	return &cli.Command{
		Name:                      progname,
		Usage:                     "[flags] [images...]",
		Writer:                    stderr,
		ErrWriter:                 stderr,
		HideHelpCommand:           true,
		DisableSliceFlagSeparator: true,
		Flags: []cli.Flag{
			// Server Options
			&cli.StringFlag{
				Name:        "listen-address",
				Aliases:     []string{"b"},
				Value:       cfg.addr,
				Usage:       "bind `address` for the HTTP API",
				Category:    catServer,
				Destination: &cfg.addr,
			},
			&cli.IntFlag{
				Name:        "parallel-jobs",
				Aliases:     []string{"P"},
				Value:       cfg.queueLength,
				Usage:       "maximum `number` of parallel rendering jobs",
				Category:    catServer,
				Destination: &cfg.queueLength,
			},
			&cli.DurationFlag{
				Name:        "queue-wait",
				Aliases:     []string{"w"},
				Value:       cfg.queueTimeout,
				Usage:       "maximum wait time in full rendering queue",
				Category:    catServer,
				Destination: &cfg.queueTimeout,
			},
			&cli.StringFlag{
				Name:        "max-job-size",
				Value:       cfg.maxJobSize,
				Usage:       "maximum size of job, a value <= 0 disables check",
				Category:    catServer,
				Destination: &cfg.maxJobSize,
			},
			&cli.DurationFlag{
				Name:        "compile-timeout",
				Aliases:     []string{"t"},
				Value:       cfg.compileTimeout,
				Usage:       "maximum rendering time",
				Category:    catServer,
				Destination: &cfg.compileTimeout,
			},

			// TeX Options
			&cli.StringFlag{
				Name:        "tex-engine",
				Aliases:     []string{"X"},
				Value:       cfg.engine,
				Usage:       fmt.Sprintf("`name` of default TeX engine, acceptable values are: %v", tex.SupportedEngines()),
				Category:    catTeX,
				Destination: &cfg.engine,
			},
			&cli.BoolFlag{
				Name:        "shell-escape",
				Usage:       "enable shell escaping to arbitrary commands (mutually exclusive with --no-shell-escape)",
				Category:    catTeX,
				Destination: shellEscape,
			},
			&cli.BoolFlag{
				Name:        "no-shell-escape",
				Usage:       "disable shell escaping to arbitrary commands (mutually exclusive with --shell-escape)",
				Category:    catTeX,
				Destination: noShellEscape,
			},
			&cli.StringFlag{
				Name:        "job-directory",
				Aliases:     []string{"D"},
				Value:       cfg.jobDir,
				Usage:       "`path` to base directory to place temporary jobs into (path must exist and it must be writable; defaults to the OS's temp directory)",
				Category:    catTeX,
				Destination: &cfg.jobDir,
			},
			&cli.StringFlag{
				Name:     "keep-jobs",
				Value:    keepJobsToString(cfg.keepJobs),
				Usage:    "keep jobs [never, on-failure, always]",
				Category: catTeX,
				Action: func(ctx context.Context, cmd *cli.Command, value string) error {
					parsed, err := parseKeepJobs(value)
					if err != nil {
						return err
					}
					cfg.keepJobs = parsed
					return nil
				},
			},

			// Docker Options
			&cli.BoolFlag{
				Name:        "pull",
				Value:       cfg.pull,
				Usage:       "always pull Docker images",
				Category:    catDocker,
				Destination: &cfg.pull,
			},

			// Reference Store Options
			&cli.StringFlag{
				Name:        "reference-store",
				Value:       cfg.storageDSN,
				Usage:       fmt.Sprintf("enable reference store and configure with `DSN`, available adapters are: %v", refstore.AvailableAdapters()),
				Category:    catRefStore,
				Destination: &cfg.storageDSN,
			},
			&cli.StringFlag{
				Name:     "retention-policy",
				Aliases:  []string{"R"},
				Value:    retentionPolicyToString(cfg.retPolicy),
				Usage:    "how to handle reference store quota [keep, purge-on-start, access]",
				Category: catRefStore,
				Action: func(ctx context.Context, cmd *cli.Command, value string) error {
					parsed, err := parseRetentionPolicy(value)
					if err != nil {
						return err
					}
					cfg.retPolicy = parsed
					return nil
				},
			},
			&cli.IntFlag{
				Name:        "rp-access-items",
				Value:       cfg.retPolItems,
				Usage:       "for retention-policy=access: maximum number of items to keep in access list, before evicting files",
				Category:    catRefStore,
				Destination: &cfg.retPolItems,
			},
			&cli.StringFlag{
				Name:        "rp-access-size",
				Value:       cfg.retPolSize,
				Usage:       "for retention-policy=access: maximum total size of items in access list, before evicting files",
				Category:    catRefStore,
				Destination: &cfg.retPolSize,
			},

			// Miscellaneous
			&cli.StringFlag{
				Name:        "log-level",
				Value:       cfg.logLevel,
				Usage:       "set logging verbosity, acceptable values are: [debug, info, warn, error, dpanic, panic, fatal]",
				Category:    catMisc,
				Destination: &cfg.logLevel,
			},
			&cli.BoolFlag{
				Name:        "version",
				Aliases:     []string{"v"},
				Usage:       "print version information and exit",
				Category:    catMisc,
				Destination: &cfg.showVersion,
			},
		},
		Action: action,
	}
}

var keepJobsMap = map[int]string{
	service.KeepJobsNever:     "never",
	service.KeepJobsAlways:    "always",
	service.KeepJobsOnFailure: "on-failure",
}

// keepJobsToString converts keepJobs int value to string.
func keepJobsToString(value int) string {
	if s, ok := keepJobsMap[value]; ok {
		return s
	}
	return "never"
}

// parseKeepJobs converts string to keepJobs int value.
func parseKeepJobs(value string) (int, error) {
	for k, v := range keepJobsMap {
		if v == value {
			return k, nil
		}
	}
	return 0, fmt.Errorf("invalid value %q for --keep-jobs: must be one of [never, on-failure, always]", value)
}

var retPolMap = map[int][]string{
	0: {"keep", "none"},
	1: {"purge-on-start", "purge"},
	2: {"access"},
}

// retentionPolicyToString converts retention policy int value to string.
func retentionPolicyToString(value int) string {
	if v, ok := retPolMap[value]; ok {
		return v[0]
	}
	return "keep"
}

// parseRetentionPolicy converts string to retention policy int value.
func parseRetentionPolicy(value string) (int, error) {
	for k, v := range retPolMap {
		if slices.Contains(v, value) {
			return k, nil
		}
	}
	return 0, fmt.Errorf("invalid value %q for --retention-policy: must be one of [keep, purge-on-start, access]", value)
}

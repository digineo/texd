package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/digineo/texd/refstore"
	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/spf13/pflag"
	"github.com/thediveo/enumflag"
)

// Sentinel errors for special flag cases.
var (
	errHelpRequested    = errors.New("help requested")
	errVersionRequested = errors.New("version requested")
)

var (
	keepJobValues = map[int][]string{
		service.KeepJobsNever:     {"never"},
		service.KeepJobsAlways:    {"always"},
		service.KeepJobsOnFailure: {"on-failure"},
	}

	retPolValues = map[int][]string{
		0: {"keep", "none"},
		1: {"purge", "purge-on-start"},
		2: {"access"},
	}
)

// parseFlags parses command-line flags and returns a config.
// Returns errHelpRequested or errVersionRequested for special cases.
func parseFlags(progname string, args []string, stderr io.Writer) (*config, error) { //nolint: funlen
	cfg := defaultConfig()

	// Temporary variables for flag parsing
	var shellEscape, noShellEscape bool

	fs := pflag.NewFlagSet(progname, pflag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage of %s:\n", progname)
		fs.PrintDefaults()
	}

	fs.StringVarP(&cfg.addr, "listen-address", "b", cfg.addr,
		"bind `address` for the HTTP API")
	fs.StringVarP(&cfg.engine, "tex-engine", "X", cfg.engine,
		fmt.Sprintf("`name` of default TeX engine, acceptable values are: %v", tex.SupportedEngines()))
	fs.BoolVarP(&shellEscape, "shell-escape", "", shellEscape,
		"enable shell escaping to arbitrary commands (mutually exclusive with --no-shell-escape)")
	fs.BoolVarP(&noShellEscape, "no-shell-escape", "", noShellEscape,
		"enable shell escaping to arbitrary commands (mutually exclusive with --shell-escape)")
	fs.DurationVarP(&cfg.compileTimeout, "compile-timeout", "t", cfg.compileTimeout,
		"maximum rendering time")
	fs.IntVarP(&cfg.queueLength, "parallel-jobs", "P", cfg.queueLength,
		"maximum `number` of parallel rendering jobs")
	fs.StringVar(&cfg.maxJobSize, "max-job-size", cfg.maxJobSize,
		"maximum size of job, a value <= 0 disables check")
	fs.DurationVarP(&cfg.queueTimeout, "queue-wait", "w", cfg.queueTimeout,
		"maximum wait time in full rendering queue")
	fs.StringVarP(&cfg.jobDir, "job-directory", "D", cfg.jobDir,
		"`path` to base directory to place temporary jobs into (path must exist and it must be writable; defaults to the OS's temp directory)")
	fs.StringVar(&cfg.storageDSN, "reference-store", cfg.storageDSN,
		fmt.Sprintf("enable reference store and configure with `DSN`, available adapters are: %v", refstore.AvailableAdapters()))
	fs.BoolVar(&cfg.pull, "pull", cfg.pull, "always pull Docker images")
	fs.StringVar(&cfg.logLevel, "log-level", cfg.logLevel,
		"set logging verbosity, acceptable values are: [debug, info, warn, error, dpanic, panic, fatal]")
	fs.BoolVarP(&cfg.showVersion, "version", "v", cfg.showVersion,
		`print version information and exit`)

	keepJobsFlag := enumflag.New(&cfg.keepJobs, "value", keepJobValues, enumflag.EnumCaseInsensitive)
	fs.Var(keepJobsFlag, "keep-jobs", "keep jobs [never, on-failure, always]")

	retPolFlag := enumflag.New(&cfg.retPolicy, "retention-policy", retPolValues, enumflag.EnumCaseInsensitive)
	fs.VarP(retPolFlag, "retention-policy", "R", "how to handle reference store quota [keep, purge-on-start, access]")
	fs.IntVar(&cfg.retPolItems, "rp-access-items", cfg.retPolItems,
		"for retention-policy=access: maximum number of items to keep in access list, before evicting files")
	fs.StringVar(&cfg.retPolSize, "rp-access-size", cfg.retPolSize,
		"for retention-policy=access: maximum total size of items in access list, before evicting files")

	switch err := fs.Parse(args); {
	case errors.Is(err, pflag.ErrHelp):
		return nil, errHelpRequested
	case err != nil:
		return nil, err
	}

	// Handle shell escape flags
	if shellEscape && noShellEscape {
		return nil, fmt.Errorf("flags --shell-escape and --no-shell-escape are mutually exclusive")
	} else if shellEscape {
		cfg.shellEscape = 1
	} else if noShellEscape {
		cfg.shellEscape = -1
	}

	// Check version flag
	if cfg.showVersion {
		return nil, errVersionRequested
	}

	// Remaining args are Docker images
	cfg.images = fs.Args()

	return cfg, nil
}

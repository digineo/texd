package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/digineo/texd"
	"github.com/digineo/texd/exec"
	"github.com/digineo/texd/refstore"
	_ "github.com/digineo/texd/refstore/dir"
	_ "github.com/digineo/texd/refstore/memcached"
	"github.com/digineo/texd/refstore/nop"
	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/digineo/texd/xlog"
	"github.com/docker/go-units"
	"github.com/spf13/pflag"
	"github.com/thediveo/enumflag"
)

const (
	defaultQueueTimeout      = 10 * time.Second
	defaultMaxJobSize        = 50 * units.MiB
	defaultCompileTimeout    = time.Minute
	defaultRetentionPoolSize = 100 * units.MiB

	exitSuccess = 0
	exitFlagErr = 2
	exitTimeout = 10 * time.Second
)

var opts = service.Options{
	Addr:           ":2201",
	QueueLength:    runtime.GOMAXPROCS(0),
	QueueTimeout:   defaultQueueTimeout,
	MaxJobSize:     defaultMaxJobSize,
	CompileTimeout: defaultCompileTimeout,
	Mode:           "local",
	Executor:       exec.LocalExec,
	KeepJobs:       service.KeepJobsNever,
}

var (
	engine      = tex.DefaultEngine.Name()
	jobdir      = ""
	pull        = false
	logLevel    = slog.LevelInfo.String()
	maxJobSize  = units.BytesSize(float64(opts.MaxJobSize))
	storageDSN  = ""
	showVersion = false

	keepJobValues = map[int][]string{
		service.KeepJobsNever:     {"never"},
		service.KeepJobsAlways:    {"always"},
		service.KeepJobsOnFailure: {"on-failure"},
	}

	retPol       int
	retPolValues = map[int][]string{
		0: {"keep", "none"},
		1: {"purge", "purge-on-start"},
		2: {"access"},
	}
	retPolItems = 1000                                               // number of items in refstore
	retPolSize  = units.BytesSize(float64(defaultRetentionPoolSize)) // max total file size
)

func retentionPolicy() (refstore.RetentionPolicy, error) {
	switch retPol {
	case 0:
		return &refstore.KeepForever{}, nil
	case 1:
		return &refstore.PurgeOnStart{}, nil
	case 2: //nolint:mnd
		sz, err := units.FromHumanSize(retPolSize)
		if err != nil {
			return nil, err
		}
		pol, err := refstore.NewAccessList(retPolItems, int(sz))
		if err != nil {
			return nil, err
		}
		return pol, nil
	}
	panic("not reached")
}

func parseFlags(progname string, args ...string) []string {
	fs := pflag.NewFlagSet(progname, pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", progname)
		fs.PrintDefaults()
	}

	fs.StringVarP(&opts.Addr, "listen-address", "b", opts.Addr,
		"bind `address` for the HTTP API")
	fs.StringVarP(&engine, "tex-engine", "X", engine,
		fmt.Sprintf("`name` of default TeX engine, acceptable values are: %v", tex.SupportedEngines()))
	fs.DurationVarP(&opts.CompileTimeout, "compile-timeout", "t", opts.CompileTimeout,
		"maximum rendering time")
	fs.IntVarP(&opts.QueueLength, "parallel-jobs", "P", opts.QueueLength,
		"maximum `number` of parallel rendering jobs")
	fs.StringVar(&maxJobSize, "max-job-size", maxJobSize,
		"maximum size of job, a value <= 0 disables check")
	fs.DurationVarP(&opts.QueueTimeout, "queue-wait", "w", opts.QueueTimeout,
		"maximum wait time in full rendering queue")
	fs.StringVarP(&jobdir, "job-directory", "D", jobdir,
		"`path` to base directory to place temporary jobs into (path must exist and it must be writable; defaults to the OS's temp directory)")
	fs.StringVar(&storageDSN, "reference-store", storageDSN,
		fmt.Sprintf("enable reference store and configure with `DSN`, available adapters are: %v", refstore.AvailableAdapters()))
	fs.BoolVar(&pull, "pull", pull, "always pull Docker images")
	fs.StringVar(&logLevel, "log-level", logLevel,
		"set logging verbosity, acceptable values are: [debug, info, warn, error, fatal]")
	fs.BoolVarP(&showVersion, "version", "v", showVersion,
		`print version information and exit`)

	keepJobsFlag := enumflag.New(&opts.KeepJobs, "value", keepJobValues, enumflag.EnumCaseInsensitive)
	fs.Var(keepJobsFlag, "keep-jobs", "keep jobs [never, on-failure, always]")

	retPolFlag := enumflag.New(&retPol, "retention-policy", retPolValues, enumflag.EnumCaseInsensitive)
	fs.VarP(retPolFlag, "retention-policy", "R", "how to handle reference store quota [keep, purge-on-start, access]")
	fs.IntVar(&retPolItems, "rp-access-items", retPolItems,
		"for retention-policy=access: maximum number of items to keep in access list, before evicting files")
	fs.StringVar(&retPolSize, "rp-access-size", retPolSize,
		"for retention-policy=access: maximum total size of items in access list, before evicting files")

	switch err := fs.Parse(args); {
	case errors.Is(err, pflag.ErrHelp):
		// pflag already has called fs.Usage
		os.Exit(exitSuccess)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error parsing flags:\n\t%v\n", err)
		os.Exit(exitFlagErr)
	}

	return fs.Args()
}

func main() { //nolint:funlen
	texd.PrintBanner(os.Stdout)
	images := parseFlags(os.Args[0], os.Args[1:]...)
	log, err := setupLogger()
	if err != nil {
		panic(err)
	}

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	if err := tex.SetJobBaseDir(jobdir); err != nil {
		log.Fatal("error setting job directory",
			xlog.String("flag", "--job-directory"),
			xlog.Error(err))
	}
	if err := tex.SetDefaultEngine(engine); err != nil {
		log.Fatal("error setting default TeX engine",
			xlog.String("flag", "--tex-engine"),
			xlog.Error(err))
	}
	if maxsz, err := units.FromHumanSize(maxJobSize); err != nil {
		log.Fatal("error parsing maximum job size",
			xlog.String("flag", "--max-job-size"),
			xlog.Error(err))
	} else {
		opts.MaxJobSize = maxsz
	}
	if storageDSN != "" {
		rp, err := retentionPolicy()
		if err != nil {
			log.Fatal("error initializing retention policy",
				xlog.String("flag", "--retention-policy, and/or --rp-access-items, --rp-access-size"),
				xlog.Error(err))
		}
		if adapter, err := refstore.NewStore(storageDSN, rp); err != nil {
			log.Fatal("error parsing reference store DSN",
				xlog.String("flag", "--reference-store"),
				xlog.Error(err))
		} else {
			opts.RefStore = adapter
		}
	} else {
		opts.RefStore, _ = nop.New(nil, nil)
	}

	if len(images) > 0 {
		log.Info("using docker", xlog.String("images", strings.Join(images, ",")))
		cli, err := exec.NewDockerClient(log, tex.JobBaseDir())
		if err != nil {
			log.Error("error connecting to dockerd", xlog.Error(err))
			os.Exit(1)
		}

		opts.Images, err = cli.SetImages(context.Background(), pull, images...)
		opts.Mode = "container"
		if err != nil {
			log.Error("error setting images", xlog.Error(err))
			os.Exit(1)
		}
		opts.Executor = cli.Executor
	}

	stop, err := service.Start(opts, log)
	if err != nil {
		log.Fatal("failed to start service", xlog.Error(err))
	}
	onExit(log, stop)
}

type stopFun func(context.Context) error

func onExit(log xlog.Logger, stopper ...stopFun) {
	exitCh := make(chan os.Signal, 2) //nolint:mnd // idiomatic
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-exitCh

	log.Info("performing shutdown, press Ctrl+C to exit now",
		xlog.String("signal", sig.String()),
		slog.Duration("graceful-wait-timeout", exitTimeout))

	ctx, cancel := context.WithTimeout(context.Background(), exitTimeout)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(len(stopper))
	for _, stop := range stopper {
		go func(f stopFun) {
			if err := f(ctx); err != nil {
				log.Error("error while shutting down", xlog.Error(err))
			}
			wg.Done()
		}(stop)
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-exitCh:
		log.Warn("forcing exit")
	case <-doneCh:
		log.Info("shutdown complete")
	case <-ctx.Done():
		log.Warn("shutdown incomplete, exiting anyway")
	}
}

func printVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	fmt.Printf("\nGo: %s, %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	const l = "  %-10s %-50s %s\n"
	fmt.Println("Dependencies:")
	fmt.Printf(l, "main", info.Main.Path, texd.Version())
	for _, i := range info.Deps {
		if r := i.Replace; r == nil {
			fmt.Printf(l, "dep", i.Path, i.Version)
		} else {
			fmt.Printf(l, "dep", r.Path, r.Version)
			fmt.Printf(l, "  replaces", i.Path, i.Version)
		}
	}
}

func setupLogger() (xlog.Logger, error) {
	lvl, err := xlog.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}

	o := &slog.HandlerOptions{
		AddSource: true,
		// XXX: provide ReplaceAttr callback to normalize Source locations?
		Level: lvl,
	}

	if texd.Development() {
		return xlog.New(xlog.TypeText, os.Stderr, o)
	}
	return xlog.New(xlog.TypeJSON, os.Stdout, o)
}

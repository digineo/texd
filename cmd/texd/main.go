package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/digineo/texd"
	"github.com/digineo/texd/exec"
	"github.com/digineo/texd/refstore"
	_ "github.com/digineo/texd/refstore/dir" // sideeffect
	"github.com/digineo/texd/refstore/nop"
	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	"github.com/docker/go-units"
	"github.com/spf13/pflag"
	"github.com/thediveo/enumflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var opts = service.Options{
	Addr:           ":2201",
	QueueLength:    runtime.GOMAXPROCS(0),
	QueueTimeout:   10 * time.Second,
	MaxJobSize:     50 * units.MiB,
	CompileTimeout: time.Minute,
	Mode:           "local",
	Executor:       exec.LocalExec,
}

var (
	engine      = tex.DefaultEngine.Name()
	jobdir      = ""
	pull        = false
	logLevel    = zapcore.InfoLevel.String()
	maxJobSize  = units.BytesSize(float64(opts.MaxJobSize))
	storageDSN  = ""
	showVersion = false

	keepJobValues = map[int][]string{
		service.KeepJobsNever:     {"never"},
		service.KeepJobsAlways:    {"always"},
		service.KeepJobsOnFailure: {"on-failure"},
	}
)

func parseFlags() []string {
	fs := pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fs.PrintDefaults()
	}

	fs.StringVarP(&opts.Addr, "listen-address", "b", opts.Addr,
		"bind `address` for the HTTP API")
	fs.StringVarP(&engine, "tex-engine", "X", engine,
		fmt.Sprintf("`name` of default TeX engine, acceptable values are: %v", tex.SupportedEngines()))
	fs.DurationVarP(&opts.CompileTimeout, "compile-timeout", "t", opts.CompileTimeout,
		"maximum rendering time")
	fs.IntVarP(&opts.QueueLength, "parallel-jobs", "P", opts.QueueLength,
		"maximum `number` of parallel rendereing jobs")
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
		"set logging verbosity, acceptable values are: [debug, info, warn, error, dpanic, panic, fatal]")
	fs.BoolVarP(&showVersion, "version", "v", showVersion,
		`print version information and exit`)

	keepJobsFlag := enumflag.New(&opts.KeepJobs, "value", keepJobValues, enumflag.EnumCaseInsensitive)
	fs.Var(keepJobsFlag, "keep-jobs", "keep jobs (never | on-failure | always)")

	switch err := fs.Parse(os.Args[1:]); {
	case errors.Is(err, pflag.ErrHelp):
		// pflag already has called fs.Usage
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error parsing flags:\n\t%v\n", err)
		os.Exit(2)
	}

	return fs.Args()
}

func main() {
	texd.PrintBanner(os.Stdout)
	images := parseFlags() //nolint:ifshort // func has sideeffects
	log, sync := setupLogger()
	defer sync()

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	if err := tex.SetJobBaseDir(jobdir); err != nil {
		log.Fatal("error setting job directory",
			zap.String("flag", "--job-directory"),
			zap.Error(err))
	}
	if err := tex.SetDefaultEngine(engine); err != nil {
		log.Fatal("error setting default TeX engine",
			zap.String("flag", "--tex-engine"),
			zap.Error(err))
	}
	if max, err := units.FromHumanSize(maxJobSize); err != nil {
		log.Fatal("error parsing maximum job size",
			zap.String("flag", "--max-job-size"),
			zap.Error(err))
	} else {
		opts.MaxJobSize = max
	}
	if storageDSN != "" {
		if adapter, err := refstore.NewStore(storageDSN); err != nil {
			log.Fatal("error parsing reference store DSN",
				zap.String("flag", "--reference-store"),
				zap.Error(err))
		} else {
			opts.RefStore = adapter
		}
	} else {
		opts.RefStore, _ = nop.New()
	}

	if len(images) > 0 {
		log.Info("using docker", zap.Strings("images", images))
		cli, err := exec.NewDockerClient(log)
		if err != nil {
			log.Fatal("error connecting to dockerd", zap.Error(err))
		}

		opts.Images, err = cli.SetImages(context.Background(), pull, images...)
		opts.Mode = "container"
		if err != nil {
			log.Fatal("error setting images", zap.Error(err))
		}
		opts.Executor = cli.Executor
	}

	stop, err := service.Start(opts, log)
	if err != nil {
		log.Fatal("failed to start service", zap.Error(err))
	}
	onExit(log, stop)
}

const exitTimeout = 10 * time.Second

type stopFun func(context.Context) error

func onExit(log *zap.Logger, stopper ...stopFun) {
	exitCh := make(chan os.Signal, 2)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-exitCh

	log.Info("performing shutdown, press Ctrl+C to exit now",
		zap.String("signal", sig.String()),
		zap.Duration("graceful-wait-timeout", exitTimeout))

	ctx, cancel := context.WithTimeout(context.Background(), exitTimeout)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(len(stopper))
	for _, stop := range stopper {
		go func(f stopFun) {
			if err := f(ctx); err != nil {
				log.Error("error while shutting down", zap.Error(err))
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

func setupLogger() (*zap.Logger, func()) {
	var cfg zap.Config
	if texd.Development() {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	lvl, lvlErr := zapcore.ParseLevel(logLevel)
	if lvlErr == nil {
		cfg.Level = zap.NewAtomicLevelAt(lvl)
	}

	log, err := cfg.Build()
	if err != nil {
		// we don't have a logger yet, so logging the error
		// proves to be complicatet :)
		panic(err)
	}

	if lvlErr != nil {
		log.Error("error parsing log level",
			zap.String("flag", "--log-level"),
			zap.Error(lvlErr))
	}

	zap.ReplaceGlobals(log)
	return log, func() {
		_ = log.Sync()
	}
}

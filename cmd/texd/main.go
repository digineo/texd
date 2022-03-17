package main

import (
	"context"
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
	"github.com/digineo/texd/service"
	"github.com/digineo/texd/tex"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var opts = service.Options{
	Addr:           ":2201",
	QueueLength:    runtime.GOMAXPROCS(0),
	QueueTimeout:   10 * time.Second,
	CompileTimeout: time.Minute,
	Mode:           "local",
	Executor:       exec.LocalExec,
}

var (
	engine   = tex.DefaultEngine.Name()
	jobdir   = ""
	pull     = false
	logLevel = zapcore.InfoLevel.String()
)

var log = zap.L()

func main() { //nolint:funlen
	texd.PrintBanner(os.Stdout)

	flag.StringVarP(&opts.Addr, "listen-address", "b", opts.Addr,
		"bind `address` for the HTTP API")
	flag.StringVarP(&engine, "tex-engine", "X", engine,
		fmt.Sprintf("`name` of default TeX engine, acceptable values are: %v", tex.SupportedEngines()))
	flag.DurationVarP(&opts.CompileTimeout, "compile-timeout", "t", opts.CompileTimeout,
		"maximum rendering time")
	flag.IntVarP(&opts.QueueLength, "parallel-jobs", "P", opts.QueueLength,
		"maximum `number` of parallel rendereing jobs")
	flag.DurationVarP(&opts.QueueTimeout, "queue-wait", "w", opts.QueueTimeout,
		"maximum wait time in full rendering queue")
	flag.StringVarP(&jobdir, "job-directory", "D", jobdir,
		"`path` to base directory to place temporary jobs into (path must exist and it must be writable; defaults to the OS's temp directory)")
	flag.BoolVar(&pull, "pull", pull, "always pull Docker images")
	flag.StringVar(&logLevel, "log-level", logLevel,
		"set logging verbosity, acceptable values are: [debug, info, warn, error, dpanic, panic, fatal]")
	versionRequested := flag.BoolP("version", "v", false, `print version information and exit`)
	flag.Parse()

	if *versionRequested {
		printVersion()
		os.Exit(0)
	}

	if lvl, err := zapcore.ParseLevel(logLevel); err != nil {
		zap.L().Fatal("error parsing log level",
			zap.String("flag", "--log-level"),
			zap.Error(err))
	} else if log, err = newLogger(lvl); err != nil {
		zap.L().Fatal("error constructing logger",
			zap.Error(err))
	} else {
		defer func() { _ = log.Sync() }()
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

	if images := flag.Args(); len(images) > 0 {
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

	stop := service.Start(opts, log)
	onExit(stop)
}

const exitTimeout = 10 * time.Second

type stopFun func(context.Context) error

func onExit(stopper ...stopFun) {
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

func newLogger(level zapcore.Level) (*zap.Logger, error) {
	var cfg zap.Config
	if texd.Development() {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	cfg.Level = zap.NewAtomicLevelAt(level)
	return cfg.Build()
}

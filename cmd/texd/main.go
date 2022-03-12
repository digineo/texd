package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/dmke/texd"
	"github.com/dmke/texd/exec"
	"github.com/dmke/texd/service"
	"github.com/dmke/texd/tex"
	flag "github.com/spf13/pflag"
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
	engine = tex.DefaultEngine.Name()
	jobdir = ""
	pull   = false
)

func main() {
	log.SetFlags(log.Lshortfile)
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
	versionRequested := flag.BoolP("version", "v", false, `print version information and exit`)
	flag.Parse()

	if *versionRequested {
		printVersion()
		os.Exit(0)
	}

	if err := tex.SetJobBaseDir(jobdir); err != nil {
		log.Fatalf("error parsing --job-directory: %v", err)
	}

	if err := tex.SetDefaultEngine(engine); err != nil {
		log.Fatalf("error parsing --tex-engine: %v", err)
	}

	if images := flag.Args(); len(images) > 0 {
		cli, err := exec.NewDockerClient()
		if err != nil {
			log.Fatalf("error connecting to dockerd: %v", err)
		}

		opts.Images, err = cli.SetImages(context.Background(), pull, images...)
		opts.Mode = "container"
		if err != nil {
			log.Fatalf("error setting images: %v", err)
		}
		opts.Executor = cli.Executor
	}

	stop := service.Start(opts)
	onExit(stop)
}

const exitTimeout = 10 * time.Second

type stopFun func(context.Context) error

func onExit(stopper ...stopFun) {
	exitCh := make(chan os.Signal, 2)
	signal.Notify(exitCh, os.Interrupt, os.Kill)
	sig := <-exitCh
	log.Printf("received signal %s, performing shutdown (max. %s, press Ctrl+C to exit now)",
		sig, exitTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), exitTimeout)

	wg := sync.WaitGroup{}
	wg.Add(len(stopper))
	for _, stop := range stopper {
		go func(f stopFun) {
			if err := f(ctx); err != nil {
				log.Printf("error while shutting down: %v", err)
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
		log.Println("forcing exit")
	case <-doneCh:
		log.Println("shutdown complete")
	case <-ctx.Done():
		log.Println("shutdown incomplete, exiting anyway")
	}
	cancel()
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

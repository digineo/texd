package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/dmke/texd/exec"
	"github.com/dmke/texd/service"
	"github.com/dmke/texd/tex"
	flag "github.com/spf13/pflag"
)

var opts = service.Options{
	Addr:        ":2201",
	QueueLength: runtime.GOMAXPROCS(0),
	Timeout:     time.Minute,
	Mode:        "local",
	Executor:    exec.LocalExec,
}

var (
	engine = tex.DefaultEngine.Name()
	jobdir = ""
	pull   = false
)

func main() {
	log.SetFlags(log.Llongfile)

	flag.StringVarP(&opts.Addr, "listen-address", "b", opts.Addr,
		"bind `address` for the HTTP API")
	flag.StringVarP(&engine, "tex-engine", "X", engine,
		fmt.Sprintf("`name` of default TeX engine, acceptable values are: %v", tex.SupportedEngines()))
	flag.DurationVarP(&opts.Timeout, "processing-timeout", "t", opts.Timeout,
		"maximum rendering time")
	flag.IntVarP(&opts.QueueLength, "parallel-jobs", "P", opts.QueueLength,
		"maximum `number` of parallel rendereing jobs")
	flag.StringVarP(&jobdir, "job-directory", "D", jobdir,
		"`path` to base directory to place temporary jobs into (path must exist and it must be writable; defaults to the OS's temp directory)")
	flag.BoolVar(&pull, "pull", pull, "always pull Docker images")
	flag.Parse()

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

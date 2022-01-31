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

	"github.com/dmke/texd/cmd"
	"github.com/dmke/texd/tex"
	flag "github.com/spf13/pflag"
)

type opMode uint8

const (
	modeLocal opMode = iota
	modeContainer
)

var (
	addr        = ":2201"
	engine      = tex.XeLaTeX.String()
	timeout     = time.Minute
	concurrency = runtime.GOMAXPROCS(0)
	queueLen    = 1000
	mode        = modeLocal
	images      []string
)

func main() {
	flag.StringVarP(&addr, "listen-address", "b", addr,
		"bind `address` for the HTTP API")
	flag.StringVarP(&engine, "tex-engine", "X", engine,
		fmt.Sprintf("`name` of default TeX engine, acceptable values are: %v", tex.SupportedEngines()))
	flag.DurationVarP(&timeout, "processing-timeout", "t", timeout,
		"maximum rendering time")
	flag.IntVarP(&concurrency, "parallel-jobs", "P", concurrency,
		"maximum `number` of parallel rendereing jobs")
	flag.IntVarP(&queueLen, "queue-length", "q", queueLen,
		"maximum `length` of queue")
	flag.Parse()

	if images = flag.Args(); len(images) > 0 {
		mode = modeContainer
	}

	if x, err := tex.ParseTeXEngine(engine); err == nil {
		tex.DefaultEngine = x
	} else {
		log.Fatalf("error parsing --tex-engine: %v", err)
	}

	stop := cmd.StartWeb(addr)
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

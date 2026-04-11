package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/digineo/texd/xlog"
)

const (
	exitTimeout = 10 * time.Second
)

type stopFun func(context.Context) error

// handleGracefulShutdown waits for termination signals and performs graceful shutdown.
func handleGracefulShutdown(log xlog.Logger, stopper ...stopFun) {
	exitCh := make(chan os.Signal, 2) //nolint:mnd // idiomatic
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-exitCh

	log.Info("performing shutdown, press Ctrl+C to exit now",
		xlog.String("signal", sig.String()),
		xlog.Duration("graceful-wait-timeout", exitTimeout))

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

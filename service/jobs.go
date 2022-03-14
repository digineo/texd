package service

import (
	"context"
	"time"

	"github.com/digineo/texd/tex"
)

func (svc *service) acquire(ctx context.Context) error {
	// don't wait too long for other jobs to complete.
	maxWait := svc.queueTimeout
	if maxWait < 0 {
		maxWait = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	select {
	case svc.jobs <- struct{}{}:
		// success
		return nil
	case <-ctx.Done():
		return tex.QueueError("queue full, please try again later", ctx.Err(), nil)
	}
}

func (svc *service) release() {
	<-svc.jobs
}

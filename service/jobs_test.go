package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextMutex(t *testing.T) {
	t.Parallel()

	svc := &service{
		jobs:         make(chan struct{}, 1),
		queueTimeout: 10 * time.Millisecond,
	}

	err := svc.acquire(context.Background())
	require.NoError(t, err)
	defer svc.release()

	// full queue should timeout
	t0 := time.Now()
	err = svc.acquire(context.Background())
	require.EqualError(t, err, "queue full, please try again later: context deadline exceeded")
	assert.True(t, time.Since(t0) >= 10*time.Millisecond)

	// already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t0 = time.Now()
	err = svc.acquire(ctx)
	require.EqualError(t, err, "queue full, please try again later: context canceled")
	assert.True(t, time.Since(t0) < 10*time.Millisecond)
}

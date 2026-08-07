package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_RunsTaskOnInterval(t *testing.T) {
	var calls int32
	s := &Scheduler{
		Interval: 5 * time.Millisecond,
		Task: func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	s.Run(ctx)

	if got := atomic.LoadInt32(&calls); got < 2 {
		t.Fatalf("expected at least 2 task calls in 25ms at 5ms interval, got %d", got)
	}
}

func TestScheduler_StopsOnContextCancel(t *testing.T) {
	done := make(chan struct{})
	s := &Scheduler{
		Interval: time.Hour,
		Task:     func(ctx context.Context) error { return nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		s.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

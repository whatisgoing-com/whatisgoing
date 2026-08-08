package scheduler

import (
	"context"
	"log"
	"time"
)

// Scheduler runs Task on a fixed interval until the context is cancelled.
type Scheduler struct {
	Interval time.Duration
	Task     func(ctx context.Context) error
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Task(ctx); err != nil {
				log.Printf("scheduler: task error: %v", err)
			}
		}
	}
}

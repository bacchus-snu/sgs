package worker

import (
	"context"
	"log"
	"time"

	"github.com/bacchus-snu/sgs/model"
)

// Queue schedules Worker invocations. Multiple queue requests are coalesced
// when made in quick succession.
type Queue struct {
	wsSvc model.WorkspaceService
	work  Worker

	period  time.Duration
	timeout time.Duration

	queue chan struct{}
}

// NewQueue creates a new Queue.
func NewQueue(wsSvc model.WorkspaceService, work Worker, period, timeout time.Duration) Queue {
	return Queue{
		wsSvc:   wsSvc,
		work:    work,
		period:  period,
		timeout: timeout,
		queue:   make(chan struct{}, 1),
	}
}

// Enqueue a Worker invokcation.
func (q Queue) Enqueue() {
	select {
	case q.queue <- struct{}{}:
	default:
		// coalesce, and also callers don't block
	}
}

// Start the Queue loop. Exits and returns when the context is done.
func (q Queue) Start(ctx context.Context) error {
	// periodic enqueue
	go func() {
		tick := time.NewTicker(q.period)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				q.Enqueue()
			case <-ctx.Done():
				return
			}
		}
	}()

	// main loop
	for {
		select {
		case <-q.queue:
			if err := q.run(ctx); err != nil {
				log.Println("queue:", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (q Queue) run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	wss, err := q.wsSvc.ListCreatedWorkspaces(ctx)
	if err != nil {
		return err
	}
	err = q.work.Work(ctx, toVWorkspaces(wss))
	if err != nil {
		return err
	}

	return nil
}

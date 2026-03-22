package notify

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// DispatcherConfig holds configuration for the Dispatcher.
type DispatcherConfig struct {
	PollInterval time.Duration
	BatchSize    int
	QueueSize    int
	SendTimeout  time.Duration
}

// Dispatcher orchestrates polling pending alert groups from the store,
// fanning out notifications to channel queues, and collecting results.
type Dispatcher struct {
	store    Store
	channels []Channel
	cfg      DispatcherConfig
	logger   *slog.Logger
	running  atomic.Bool
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(store Store, channels []Channel, cfg DispatcherConfig, logger *slog.Logger) *Dispatcher {
	logger.Info("creating dispatcher",
		"channels_count", len(channels),
		"poll_interval", cfg.PollInterval,
		"batch_size", cfg.BatchSize,
		"queue_size", cfg.QueueSize,
		"send_timeout", cfg.SendTimeout,
	)
	return &Dispatcher{
		store:    store,
		channels: channels,
		cfg:      cfg,
		logger:   logger,
	}
}

// Name implements server.Checker.
func (d *Dispatcher) Name() string {
	return "dispatcher"
}

// Check implements server.Checker. Returns an error if the polling loop is not active.
func (d *Dispatcher) Check(_ context.Context) error {
	if !d.running.Load() {
		return errors.New("dispatcher is not running")
	}
	return nil
}

// Run starts the polling loop. It blocks until the context is cancelled.
// Returns nil on graceful shutdown.
func (d *Dispatcher) Run(ctx context.Context) error {
	d.logger.Info("dispatcher starting")

	if len(d.channels) == 0 {
		d.logger.Warn("dispatcher has no channels configured — polling without dispatch")
	}

	// Create per-channel queues and workers.
	resultCh := make(chan SendResult, len(d.channels)*d.cfg.BatchSize+1)
	queues := make(map[string]*Queue, len(d.channels))
	var workers []*Worker

	for _, ch := range d.channels {
		q := NewQueue(ch.Name(), d.cfg.QueueSize, d.logger)
		queues[ch.Name()] = q
		w := NewWorker(ch, q, resultCh, d.cfg.SendTimeout, d.logger)
		workers = append(workers, w)
	}

	// Start workers.
	var workerWg sync.WaitGroup
	for _, w := range workers {
		workerWg.Add(1)
		go func(w *Worker) {
			defer workerWg.Done()
			w.Run(ctx)
		}(w)
	}

	d.running.Store(true)
	defer d.running.Store(false)

	d.logger.Info("dispatcher running", "workers", len(workers))

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	// Main polling loop.
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("dispatcher shutting down")
			d.shutdown(queues, &workerWg, resultCh)
			return nil
		case <-ticker.C:
			d.poll(ctx, queues, resultCh)
		}
	}
}

// poll executes a single poll cycle: GetPending → fan-out → collect → MarkSent.
func (d *Dispatcher) poll(ctx context.Context, queues map[string]*Queue, resultCh chan SendResult) {
	d.logger.Debug("polling for pending alert groups", "batch_size", d.cfg.BatchSize)

	groups, err := d.store.GetPending(ctx, d.cfg.BatchSize)
	if err != nil {
		d.logger.Error("failed to get pending alert groups", "error", err)
		return
	}

	if len(groups) == 0 {
		d.logger.Debug("no pending alert groups")
		return
	}

	d.logger.Info("fetched pending alert groups", "count", len(groups))

	if len(d.channels) == 0 {
		d.logger.Debug("no channels configured, skipping dispatch")
		return
	}

	for i := range groups {
		n := NewNotification(&groups[i])
		d.fanOutAndCollect(ctx, n, queues, resultCh)
	}
}

// fanOutAndCollect enqueues a notification to all channel queues and
// collects results. Calls MarkSent only if ALL channels succeed.
func (d *Dispatcher) fanOutAndCollect(
	ctx context.Context,
	n *Notification,
	queues map[string]*Queue,
	resultCh chan SendResult,
) {
	enqueued := 0
	for _, ch := range d.channels {
		q := queues[ch.Name()]
		sendCtx, sendCancel := context.WithTimeout(ctx, d.cfg.SendTimeout)
		err := q.Enqueue(sendCtx, n)
		sendCancel()
		if err != nil {
			d.logger.Error("failed to enqueue notification",
				"channel", ch.Name(),
				"group_key", n.GroupKey,
				"error", err,
			)
			continue
		}
		enqueued++
	}

	if enqueued == 0 {
		d.logger.Warn("notification not enqueued to any channel", "group_key", n.GroupKey)
		return
	}

	// Collect results from workers.
	allOK := true
	for range enqueued {
		select {
		case r := <-resultCh:
			if r.GroupKey != n.GroupKey {
				d.logger.Error("result group key mismatch",
					"expected", n.GroupKey,
					"got", r.GroupKey,
					"channel", r.Channel,
				)
			}
			if r.Err != nil {
				d.logger.Warn("channel delivery failed",
					"channel", r.Channel,
					"group_key", r.GroupKey,
					"error", r.Err,
				)
				allOK = false
			} else {
				d.logger.Debug("channel delivery succeeded",
					"channel", r.Channel,
					"group_key", r.GroupKey,
				)
			}
		case <-ctx.Done():
			d.logger.Warn("context cancelled while collecting results", "group_key", n.GroupKey)
			return
		}
	}

	if !allOK {
		d.logger.Warn("not all channels succeeded, leaving group pending",
			"group_key", n.GroupKey,
		)
		return
	}

	d.logger.Info("all channels succeeded, marking sent", "group_key", n.GroupKey)
	if err := d.store.MarkSent(ctx, n.GroupKey); err != nil {
		d.logger.Error("failed to mark group as sent",
			"group_key", n.GroupKey,
			"error", err,
		)
	}
}

// shutdown gracefully stops all queues and waits for workers to finish.
func (d *Dispatcher) shutdown(queues map[string]*Queue, workerWg *sync.WaitGroup, resultCh chan SendResult) {
	d.logger.Info("closing all queues")
	for name, q := range queues {
		d.logger.Debug("closing queue", "queue_name", name)
		q.Close()
	}

	d.logger.Debug("waiting for workers to finish")
	workerWg.Wait()

	close(resultCh)
	// Drain any remaining results.
	for r := range resultCh {
		d.logger.Debug("draining result",
			"channel", r.Channel,
			"group_key", r.GroupKey,
			"error", r.Err,
		)
	}

	d.logger.Info("dispatcher shutdown complete")
}

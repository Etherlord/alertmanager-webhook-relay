package notify

import (
	"context"
	"log/slog"
	"time"
)

// Worker processes notifications from a queue by sending them through a Channel.
// Each Channel has its own Worker + Queue pair.
type Worker struct {
	ch          Channel
	queue       *Queue
	resultCh    chan<- SendResult
	sendTimeout time.Duration
	logger      *slog.Logger
}

// NewWorker creates a new worker for the given channel and queue.
// Results are sent to resultCh for the dispatcher to collect.
func NewWorker(ch Channel, queue *Queue, resultCh chan<- SendResult, sendTimeout time.Duration, logger *slog.Logger) *Worker {
	logger.Debug("creating worker",
		"channel", ch.Name(),
		"queue", queue.Name(),
		"send_timeout", sendTimeout,
	)
	return &Worker{
		ch:          ch,
		queue:       queue,
		resultCh:    resultCh,
		sendTimeout: sendTimeout,
		logger:      logger,
	}
}

// Run starts the worker loop. It dequeues notifications and sends them
// through the channel until the context is cancelled or the queue is closed.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started", "channel", w.ch.Name())
	defer w.logger.Info("worker stopped", "channel", w.ch.Name())

	processed := 0
	for {
		n, err := w.queue.Dequeue(ctx)
		if err != nil {
			w.logger.Info("worker dequeue ended",
				"channel", w.ch.Name(),
				"processed", processed,
				"reason", err.Error(),
			)
			return
		}

		w.logger.Debug("worker sending notification",
			"channel", w.ch.Name(),
			"group_key", n.GroupKey,
		)

		// Use background context for send timeout so in-flight sends
		// are not aborted when the dispatcher context is cancelled.
		// Workers stay alive during drain via a separate workerCtx.
		sendCtx, sendCancel := context.WithTimeout(context.Background(), w.sendTimeout)
		sendErr := w.ch.Send(sendCtx, n)
		sendCancel()

		if sendErr != nil {
			w.logger.Error("notification delivery failed",
				"channel", w.ch.Name(),
				"group_key", n.GroupKey,
				"error", sendErr,
			)
		} else {
			w.logger.Debug("notification delivered",
				"channel", w.ch.Name(),
				"group_key", n.GroupKey,
			)
		}

		w.resultCh <- SendResult{
			GroupKey: n.GroupKey,
			Channel:  w.ch.Name(),
			Err:      sendErr,
		}
		processed++
	}
}

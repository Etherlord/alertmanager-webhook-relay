package notify

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
)

// ErrQueueFull is returned when enqueue cannot complete within the context deadline.
var ErrQueueFull = errors.New("queue full: enqueue timed out")

// ErrQueueClosed is returned when operating on a closed queue.
var ErrQueueClosed = errors.New("queue closed")

// Queue is a bounded, context-aware notification queue backed by a Go buffered channel.
// Each notification channel (pachca, email) has its own Queue instance.
type Queue struct {
	name   string
	ch     chan *Notification
	done   chan struct{}
	closed atomic.Bool
	logger *slog.Logger
}

// NewQueue creates a new bounded queue with the given capacity.
func NewQueue(name string, size int, logger *slog.Logger) *Queue {
	logger.Debug("creating notification queue",
		"queue_name", name,
		"queue_size", size,
	)
	return &Queue{
		name:   name,
		ch:     make(chan *Notification, size),
		done:   make(chan struct{}),
		logger: logger,
	}
}

// Name returns the queue name (matches the channel name).
func (q *Queue) Name() string {
	return q.name
}

// Len returns the current number of items in the queue.
func (q *Queue) Len() int {
	return len(q.ch)
}

// Enqueue adds a notification to the queue. Blocks until space is available,
// the context is cancelled, or the queue is closed.
// Returns ErrQueueFull if the context expires while waiting.
// Returns ErrQueueClosed if the queue has been closed.
func (q *Queue) Enqueue(ctx context.Context, n *Notification) error {
	if q.closed.Load() {
		q.logger.Warn("enqueue on closed queue",
			"queue_name", q.name,
			"group_key", n.GroupKey,
		)
		return ErrQueueClosed
	}

	select {
	case q.ch <- n:
		q.logger.Debug("notification enqueued",
			"queue_name", q.name,
			"group_key", n.GroupKey,
			"queue_len", len(q.ch),
		)
		return nil
	case <-ctx.Done():
		q.logger.Warn("enqueue back-pressure: queue full",
			"queue_name", q.name,
			"group_key", n.GroupKey,
			"queue_len", len(q.ch),
		)
		return ErrQueueFull
	case <-q.done:
		return ErrQueueClosed
	}
}

// Dequeue removes and returns the next notification from the queue.
// Blocks until a notification is available, the context is cancelled, or the queue is closed.
// After Close(), remaining items can still be dequeued until the queue is drained.
func (q *Queue) Dequeue(ctx context.Context) (*Notification, error) {
	// First try non-blocking read to drain remaining items (even after close).
	select {
	case n, ok := <-q.ch:
		if !ok {
			return nil, ErrQueueClosed
		}
		q.logger.Debug("notification dequeued",
			"queue_name", q.name,
			"group_key", n.GroupKey,
			"queue_len", len(q.ch),
		)
		return n, nil
	default:
	}

	// Blocking wait.
	select {
	case n, ok := <-q.ch:
		if !ok {
			return nil, ErrQueueClosed
		}
		q.logger.Debug("notification dequeued",
			"queue_name", q.name,
			"group_key", n.GroupKey,
			"queue_len", len(q.ch),
		)
		return n, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-q.done:
		// After close signal, drain remaining items.
		select {
		case n, ok := <-q.ch:
			if !ok {
				return nil, ErrQueueClosed
			}
			return n, nil
		default:
			return nil, ErrQueueClosed
		}
	}
}

// Close signals the queue to stop accepting new items and unblocks waiting consumers.
// Items already in the queue can still be dequeued.
func (q *Queue) Close() {
	if q.closed.CompareAndSwap(false, true) {
		q.logger.Info("closing notification queue",
			"queue_name", q.name,
			"remaining_items", len(q.ch),
		)
		close(q.done)
	}
}

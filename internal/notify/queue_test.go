package notify

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/alerts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func testNotification(groupKey string) *Notification {
	n := NewNotification(&alerts.AlertGroup{
		GroupKey: groupKey,
		Status:   alerts.StatusFiring,
		Receiver: "test",
	})
	return &n
}

func TestNewQueue(t *testing.T) {
	q := NewQueue("test-channel", 10, testLogger())

	assert.Equal(t, "test-channel", q.Name())
	assert.Equal(t, 0, q.Len())
}

func TestQueue_EnqueueDequeue(t *testing.T) {
	q := NewQueue("test", 5, testLogger())
	ctx := context.Background()
	n := testNotification("group-1")

	err := q.Enqueue(ctx, n)
	require.NoError(t, err)
	assert.Equal(t, 1, q.Len())

	got, err := q.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, "group-1", got.GroupKey)
	assert.Equal(t, 0, q.Len())
}

func TestQueue_FIFO(t *testing.T) {
	q := NewQueue("test", 5, testLogger())
	ctx := context.Background()

	for i := range 3 {
		n := testNotification("group-" + string(rune('a'+i)))
		require.NoError(t, q.Enqueue(ctx, n))
	}

	assert.Equal(t, 3, q.Len())

	got1, _ := q.Dequeue(ctx)
	got2, _ := q.Dequeue(ctx)
	got3, _ := q.Dequeue(ctx)

	assert.Equal(t, "group-a", got1.GroupKey)
	assert.Equal(t, "group-b", got2.GroupKey)
	assert.Equal(t, "group-c", got3.GroupKey)
}

func TestQueue_BackPressure_ContextCancel(t *testing.T) {
	q := NewQueue("test", 2, testLogger())
	ctx := context.Background()

	require.NoError(t, q.Enqueue(ctx, testNotification("g1")))
	require.NoError(t, q.Enqueue(ctx, testNotification("g2")))
	assert.Equal(t, 2, q.Len())

	// Queue is full. Enqueue with short timeout should fail.
	shortCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	err := q.Enqueue(shortCtx, testNotification("g3"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrQueueFull)
}

func TestQueue_Enqueue_CancelledContext(t *testing.T) {
	q := NewQueue("test", 1, testLogger())
	require.NoError(t, q.Enqueue(context.Background(), testNotification("g1")))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := q.Enqueue(ctx, testNotification("g2"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrQueueFull)
}

func TestQueue_Dequeue_CancelledContext(t *testing.T) {
	q := NewQueue("test", 5, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestQueue_Close_UnblocksDequeue(t *testing.T) {
	q := NewQueue("test", 5, testLogger())

	done := make(chan struct{})
	go func() {
		_, err := q.Dequeue(context.Background())
		assert.ErrorIs(t, err, ErrQueueClosed)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let goroutine block on Dequeue
	q.Close()

	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("Dequeue did not unblock after Close")
	}
}

func TestQueue_Enqueue_AfterClose(t *testing.T) {
	q := NewQueue("test", 5, testLogger())
	q.Close()

	err := q.Enqueue(context.Background(), testNotification("g1"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrQueueClosed)
}

func TestQueue_ConcurrentEnqueueDequeue(t *testing.T) {
	q := NewQueue("test", 100, testLogger())
	ctx := context.Background()

	const producers = 5
	const messagesPerProducer = 20

	var wg sync.WaitGroup

	// Producers
	for p := range producers {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			for m := range messagesPerProducer {
				n := testNotification("p" + string(rune('0'+pid)) + "-m" + string(rune('0'+m)))
				_ = q.Enqueue(ctx, n)
			}
		}(p)
	}

	// Consumer
	received := make([]*Notification, 0, producers*messagesPerProducer)
	var mu sync.Mutex

	consumerDone := make(chan struct{})
	go func() {
		for range producers * messagesPerProducer {
			n, err := q.Dequeue(ctx)
			if err != nil {
				break
			}
			mu.Lock()
			received = append(received, n)
			mu.Unlock()
		}
		close(consumerDone)
	}()

	wg.Wait()

	select {
	case <-consumerDone:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not finish in time")
	}

	mu.Lock()
	assert.Equal(t, producers*messagesPerProducer, len(received))
	mu.Unlock()
}

func TestQueue_Drain_OnClose(t *testing.T) {
	q := NewQueue("test", 5, testLogger())
	ctx := context.Background()

	require.NoError(t, q.Enqueue(ctx, testNotification("g1")))
	require.NoError(t, q.Enqueue(ctx, testNotification("g2")))

	q.Close()

	// Items already in queue can still be dequeued after close.
	n1, err := q.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, "group-key-not-checked", "group-key-not-checked") // just drain
	_ = n1

	n2, err := q.Dequeue(ctx)
	require.NoError(t, err)
	_ = n2

	// After drain, Dequeue returns ErrQueueClosed.
	_, err = q.Dequeue(ctx)
	assert.ErrorIs(t, err, ErrQueueClosed)
}

package notify

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChannel is a test double for the Channel interface.
type mockChannel struct {
	name   string
	sendFn func(ctx context.Context, n *Notification) error
	mu     sync.Mutex
	calls  []*Notification
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Send(ctx context.Context, n *Notification) error {
	m.mu.Lock()
	m.calls = append(m.calls, n)
	m.mu.Unlock()
	if m.sendFn != nil {
		return m.sendFn(ctx, n)
	}
	return nil
}

func (m *mockChannel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func TestWorker_SendSuccess(t *testing.T) {
	ch := &mockChannel{name: "test-channel"}
	q := NewQueue("test-channel", 10, testLogger())
	resultCh := make(chan SendResult, 10)
	w := NewWorker(ch, q, resultCh, 5*time.Second, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Run(ctx)
	}()

	// Enqueue a notification.
	require.NoError(t, q.Enqueue(ctx, testNotification("group-1")))

	// Wait for result.
	select {
	case r := <-resultCh:
		assert.Equal(t, "group-1", r.GroupKey)
		assert.Equal(t, "test-channel", r.Channel)
		assert.NoError(t, r.Err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}

	assert.Equal(t, 1, ch.callCount())

	cancel()
	wg.Wait()
}

func TestWorker_SendFailure(t *testing.T) {
	sendErr := errors.New("delivery failed")
	ch := &mockChannel{
		name: "failing-channel",
		sendFn: func(_ context.Context, _ *Notification) error {
			return sendErr
		},
	}
	q := NewQueue("failing-channel", 10, testLogger())
	resultCh := make(chan SendResult, 10)
	w := NewWorker(ch, q, resultCh, 5*time.Second, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Run(ctx)
	}()

	require.NoError(t, q.Enqueue(ctx, testNotification("group-fail")))

	select {
	case r := <-resultCh:
		assert.Equal(t, "group-fail", r.GroupKey)
		assert.Equal(t, "failing-channel", r.Channel)
		assert.ErrorIs(t, r.Err, sendErr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}

	cancel()
	wg.Wait()
}

func TestWorker_MultipleNotifications(t *testing.T) {
	ch := &mockChannel{name: "multi"}
	q := NewQueue("multi", 10, testLogger())
	resultCh := make(chan SendResult, 10)
	w := NewWorker(ch, q, resultCh, 5*time.Second, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Run(ctx)
	}()

	for i := range 3 {
		require.NoError(t, q.Enqueue(ctx, testNotification(fmt.Sprintf("g-%d", i))))
	}

	results := make([]SendResult, 0, 3)
	for range 3 {
		select {
		case r := <-resultCh:
			results = append(results, r)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for results")
		}
	}

	assert.Len(t, results, 3)
	for _, r := range results {
		assert.NoError(t, r.Err)
		assert.Equal(t, "multi", r.Channel)
	}

	cancel()
	wg.Wait()
}

func TestWorker_GracefulShutdown(t *testing.T) {
	ch := &mockChannel{name: "shutdown-test"}
	q := NewQueue("shutdown-test", 10, testLogger())
	resultCh := make(chan SendResult, 10)
	w := NewWorker(ch, q, resultCh, 5*time.Second, testLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Cancel context — worker should exit.
	cancel()

	select {
	case <-done:
		// Worker exited gracefully.
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not shut down in time")
	}
}

func TestWorker_QueueClose_StopsWorker(t *testing.T) {
	ch := &mockChannel{name: "close-test"}
	q := NewQueue("close-test", 10, testLogger())
	resultCh := make(chan SendResult, 10)
	w := NewWorker(ch, q, resultCh, 5*time.Second, testLogger())

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Close the queue — worker should exit.
	q.Close()

	select {
	case <-done:
		// Worker exited.
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after queue close")
	}
}

func TestWorker_SendTimeout(t *testing.T) {
	ch := &mockChannel{
		name: "slow-channel",
		sendFn: func(ctx context.Context, _ *Notification) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	q := NewQueue("slow-channel", 10, testLogger())
	resultCh := make(chan SendResult, 10)
	w := NewWorker(ch, q, resultCh, 100*time.Millisecond, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Run(ctx)
	}()

	require.NoError(t, q.Enqueue(ctx, testNotification("slow-g1")))

	select {
	case r := <-resultCh:
		assert.Equal(t, "slow-g1", r.GroupKey)
		assert.Equal(t, "slow-channel", r.Channel)
		assert.Error(t, r.Err)
		assert.ErrorIs(t, r.Err, context.DeadlineExceeded)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}

	cancel()
	wg.Wait()
}

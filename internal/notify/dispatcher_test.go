package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/alerts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore is a test double for the notify.Store interface.
type mockStore struct {
	mu       sync.Mutex
	pending  []alerts.AlertGroup
	sentKeys []string
	getErr   error
	markErr  error
	getCalls int
}

func (m *mockStore) GetPending(_ context.Context, limit int) ([]alerts.AlertGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	if m.getErr != nil {
		return nil, m.getErr
	}
	end := limit
	if end > len(m.pending) {
		end = len(m.pending)
	}
	result := make([]alerts.AlertGroup, end)
	copy(result, m.pending[:end])
	m.pending = m.pending[end:]
	return result, nil
}

func (m *mockStore) MarkSent(_ context.Context, groupKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.markErr != nil {
		return m.markErr
	}
	m.sentKeys = append(m.sentKeys, groupKey)
	return nil
}

func (m *mockStore) getSentKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.sentKeys))
	copy(out, m.sentKeys)
	return out
}

func (m *mockStore) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getCalls
}

func TestDispatcher_PollAndDispatch(t *testing.T) {
	store := &mockStore{
		pending: []alerts.AlertGroup{
			{GroupKey: "g1", Status: alerts.StatusFiring, Receiver: "test"},
		},
	}
	ch := &mockChannel{name: "ch1"}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, []Channel{ch}, cfg, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	require.NoError(t, err)

	sentKeys := store.getSentKeys()
	assert.Contains(t, sentKeys, "g1")
	assert.Equal(t, 1, ch.callCount())
}

func TestDispatcher_EmptyPoll(t *testing.T) {
	store := &mockStore{pending: nil}
	ch := &mockChannel{name: "ch1"}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, []Channel{ch}, cfg, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	require.NoError(t, err)

	// No notifications sent.
	assert.Equal(t, 0, ch.callCount())
	// But store was polled at least once.
	assert.GreaterOrEqual(t, store.getCallCount(), 1)
}

func TestDispatcher_MultipleChannels(t *testing.T) {
	store := &mockStore{
		pending: []alerts.AlertGroup{
			{GroupKey: "g1", Status: alerts.StatusFiring, Receiver: "test"},
		},
	}
	ch1 := &mockChannel{name: "ch1"}
	ch2 := &mockChannel{name: "ch2"}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, []Channel{ch1, ch2}, cfg, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	require.NoError(t, err)

	// Both channels should receive the notification.
	assert.Equal(t, 1, ch1.callCount())
	assert.Equal(t, 1, ch2.callCount())

	// MarkSent should be called once (all channels succeeded).
	sentKeys := store.getSentKeys()
	assert.Contains(t, sentKeys, "g1")
}

func TestDispatcher_ChannelError_NoMarkSent(t *testing.T) {
	store := &mockStore{
		pending: []alerts.AlertGroup{
			{GroupKey: "g-err", Status: alerts.StatusFiring, Receiver: "test"},
		},
	}
	failCh := &mockChannel{
		name: "fail-ch",
		sendFn: func(_ context.Context, _ *Notification) error {
			return errors.New("delivery failed")
		},
	}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, []Channel{failCh}, cfg, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	require.NoError(t, err)

	// Channel was called.
	assert.Equal(t, 1, failCh.callCount())
	// But MarkSent should NOT be called since channel failed.
	sentKeys := store.getSentKeys()
	assert.NotContains(t, sentKeys, "g-err")
}

func TestDispatcher_PartialChannelError(t *testing.T) {
	store := &mockStore{
		pending: []alerts.AlertGroup{
			{GroupKey: "g-partial", Status: alerts.StatusFiring, Receiver: "test"},
		},
	}
	okCh := &mockChannel{name: "ok-ch"}
	failCh := &mockChannel{
		name: "fail-ch",
		sendFn: func(_ context.Context, _ *Notification) error {
			return errors.New("fail")
		},
	}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, []Channel{okCh, failCh}, cfg, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	require.NoError(t, err)

	// Both channels called.
	assert.Equal(t, 1, okCh.callCount())
	assert.Equal(t, 1, failCh.callCount())
	// Not marked sent — at least one channel failed.
	sentKeys := store.getSentKeys()
	assert.NotContains(t, sentKeys, "g-partial")
}

func TestDispatcher_NoChannels(t *testing.T) {
	store := &mockStore{
		pending: []alerts.AlertGroup{
			{GroupKey: "g-no-ch", Status: alerts.StatusFiring, Receiver: "test"},
		},
	}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, nil, cfg, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	require.NoError(t, err)

	// With no channels, pending alerts are polled but not dispatched.
	// MarkSent should NOT be called — no channels to confirm delivery.
	sentKeys := store.getSentKeys()
	assert.Empty(t, sentKeys)
}

func TestDispatcher_GracefulShutdown(t *testing.T) {
	store := &mockStore{}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, nil, cfg, testLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()

	// Give dispatcher time to start polling.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher did not shut down in time")
	}
}

func TestDispatcher_ReadinessCheck(t *testing.T) {
	store := &mockStore{}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, nil, cfg, testLogger())

	// Before Run — check should fail.
	err := d.Check(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "dispatcher", d.Name())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()

	// Wait for dispatcher to start.
	time.Sleep(100 * time.Millisecond)

	// After Run started — check should pass.
	err = d.Check(context.Background())
	assert.NoError(t, err)

	cancel()
	<-done
}

func TestDispatcher_GetPendingError(t *testing.T) {
	store := &mockStore{
		getErr: errors.New("db connection lost"),
	}

	cfg := DispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		QueueSize:    10,
		SendTimeout:  5 * time.Second,
	}

	d := NewDispatcher(store, nil, cfg, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Should not crash — logs error, continues polling.
	err := d.Run(ctx)
	require.NoError(t, err)

	// Store was polled multiple times despite errors.
	assert.GreaterOrEqual(t, store.getCallCount(), 1)
}

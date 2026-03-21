package alerts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore implements Store for testing.
type mockStore struct {
	saveFunc       func(ctx context.Context, group *AlertGroup) error
	getPendingFunc func(ctx context.Context, limit int) ([]AlertGroup, error)
	markSentFunc   func(ctx context.Context, id string) error
}

func (m *mockStore) Save(ctx context.Context, group *AlertGroup) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, group)
	}
	return nil
}

func (m *mockStore) GetPending(ctx context.Context, limit int) ([]AlertGroup, error) {
	if m.getPendingFunc != nil {
		return m.getPendingFunc(ctx, limit)
	}
	return nil, nil
}

func (m *mockStore) MarkSent(ctx context.Context, id string) error {
	if m.markSentFunc != nil {
		return m.markSentFunc(ctx, id)
	}
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestService_Receive_Success(t *testing.T) {
	var savedGroup AlertGroup
	store := &mockStore{
		saveFunc: func(_ context.Context, group *AlertGroup) error {
			savedGroup = *group
			return nil
		},
	}

	svc := NewService(store, testLogger(), 100)
	group := validGroup()

	err := svc.Receive(context.Background(), &group)
	require.NoError(t, err)

	assert.Equal(t, group.GroupKey, savedGroup.GroupKey)
	assert.Equal(t, group.Receiver, savedGroup.Receiver)

	t.Logf("receive success: saved group_key=%s", savedGroup.GroupKey)
}

func TestService_Receive_ValidationError(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger(), 100)

	tests := []struct {
		name  string
		setup func(g *AlertGroup)
		err   error
	}{
		{
			name:  "invalid version",
			setup: func(g *AlertGroup) { g.Version = "3" },
			err:   ErrInvalidPayload,
		},
		{
			name:  "empty alerts",
			setup: func(g *AlertGroup) { g.Alerts = nil },
			err:   ErrInvalidPayload,
		},
		{
			name: "too many alerts",
			setup: func(g *AlertGroup) {
				g.Alerts = make([]Alert, 101)
				for i := range g.Alerts {
					g.Alerts[i] = Alert{
						Status:      StatusFiring,
						Labels:      map[string]string{"alertname": "T"},
						Fingerprint: "fp",
					}
				}
			},
			err: ErrPayloadTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := validGroup()
			tt.setup(&group)

			err := svc.Receive(context.Background(), &group)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.err)

			t.Logf("%s → error: %v", tt.name, err)
		})
	}
}

func TestService_Receive_StoreError(t *testing.T) {
	storeErr := errors.New("database unavailable")
	store := &mockStore{
		saveFunc: func(_ context.Context, _ *AlertGroup) error {
			return storeErr
		},
	}

	svc := NewService(store, testLogger(), 100)
	group := validGroup()

	err := svc.Receive(context.Background(), &group)
	require.Error(t, err)
	assert.ErrorIs(t, err, storeErr)

	t.Logf("store error propagated: %v", err)
}

func TestService_Receive_DoesNotCallStore_OnValidationFailure(t *testing.T) {
	storeCalled := false
	store := &mockStore{
		saveFunc: func(_ context.Context, _ *AlertGroup) error {
			storeCalled = true
			return nil
		},
	}

	svc := NewService(store, testLogger(), 100)

	group := validGroup()
	group.Version = "3" // invalid

	_ = svc.Receive(context.Background(), &group) //nolint:errcheck // intentionally ignoring
	assert.False(t, storeCalled, "store.Save must not be called on validation failure")

	t.Log("store was not called when validation failed")
}

func TestService_Receive_CustomMaxAlerts(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, testLogger(), 5)

	group := validGroup()
	group.Alerts = make([]Alert, 6)
	for i := range group.Alerts {
		group.Alerts[i] = Alert{
			Status:      StatusFiring,
			Labels:      map[string]string{"alertname": "T"},
			StartsAt:    time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC),
			Fingerprint: "fp",
		}
	}

	err := svc.Receive(context.Background(), &group)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPayloadTooLarge)

	t.Logf("custom maxAlerts=5, 6 alerts → error: %v", err)
}

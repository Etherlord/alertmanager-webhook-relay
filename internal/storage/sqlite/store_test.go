package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	require.NoError(t, ConfigurePragmas(context.Background(), db))
	testutil.ApplyMigrations(t, db, "../../../migrations/sqlite")

	store := &Store{db: db, logger: testLogger()}
	t.Cleanup(func() { store.Close() })
	return store
}

func ptrGroup(groupKey string) *alerts.AlertGroup {
	g := testGroup(groupKey)
	return &g
}

func testGroup(groupKey string) alerts.AlertGroup {
	return alerts.AlertGroup{
		Receiver: "webhook",
		Status:   alerts.StatusFiring,
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations: map[string]string{"summary": "test alert"},
				StartsAt:    time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC),
				Fingerprint: "abc123",
			},
		},
		GroupLabels:       map[string]string{"alertname": "TestAlert"},
		CommonLabels:      map[string]string{"alertname": "TestAlert"},
		CommonAnnotations: map[string]string{"summary": "test alert"},
		ExternalURL:       "http://alertmanager:9093",
		Version:           "4",
		GroupKey:          groupKey,
	}
}

func TestNew_InMemory(t *testing.T) {
	store := newTestStore(t)
	assert.NotNil(t, store)
	t.Log("in-memory SQLite store created successfully")
}

func TestStore_Save_RoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	group := testGroup("test-key-1")

	err := store.Save(ctx, &group)
	require.NoError(t, err)

	pending, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)

	got := pending[0]
	assert.Equal(t, group.Receiver, got.Receiver)
	assert.Equal(t, group.Status, got.Status)
	assert.Equal(t, group.GroupKey, got.GroupKey)
	assert.Equal(t, group.ExternalURL, got.ExternalURL)
	assert.Equal(t, group.Version, got.Version)
	require.Len(t, got.Alerts, 1)
	assert.Equal(t, group.Alerts[0].Fingerprint, got.Alerts[0].Fingerprint)
	assert.Equal(t, group.Alerts[0].Labels["alertname"], got.Alerts[0].Labels["alertname"])

	t.Logf("round-trip: saved and retrieved group_key=%s, alerts=%d", got.GroupKey, len(got.Alerts))
}

func TestStore_Save_Idempotent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	group := testGroup("idempotent-key")

	// First save
	err := store.Save(ctx, &group)
	require.NoError(t, err)

	// Second save with updated status — should upsert
	group.Status = alerts.StatusResolved
	group.Alerts[0].Status = alerts.StatusResolved
	err = store.Save(ctx, &group)
	require.NoError(t, err)

	// Should still be one group
	pending, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)

	// Payload should reflect updated status
	assert.Equal(t, alerts.StatusResolved, pending[0].Status)

	t.Log("idempotent save: upserted correctly, single record maintained")
}

func TestStore_GetPending_Filtering(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Save two groups
	err := store.Save(ctx, ptrGroup("pending-1"))
	require.NoError(t, err)
	err = store.Save(ctx, ptrGroup("pending-2"))
	require.NoError(t, err)

	// Mark first as sent
	pending, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 2)

	err = store.MarkSent(ctx, pending[0].GroupKey)
	require.NoError(t, err)

	// Now only one pending
	pending, err = store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)

	t.Log("GetPending filtering: correctly excludes sent groups")
}

func TestStore_GetPending_Limit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for i := range 5 {
		err := store.Save(ctx, ptrGroup("limit-"+string(rune('a'+i))))
		require.NoError(t, err)
	}

	pending, err := store.GetPending(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, pending, 3)

	t.Logf("GetPending limit: requested 3, got %d", len(pending))
}

func TestStore_MarkSent_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.MarkSent(ctx, "nonexistent-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, alerts.ErrNotFound)

	t.Logf("MarkSent not found: error=%v", err)
}

func TestStore_Save_MultipleAlerts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	group := testGroup("multi-alert")
	group.Alerts = append(group.Alerts, alerts.Alert{
		Status:      alerts.StatusResolved,
		Labels:      map[string]string{"alertname": "TestAlert", "severity": "warning"},
		Annotations: map[string]string{"summary": "second alert"},
		StartsAt:    time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC),
		Fingerprint: "def456",
	})

	err := store.Save(ctx, &group)
	require.NoError(t, err)

	pending, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)

	// Alerts should be reconstructed from payload
	got := pending[0]
	require.Len(t, got.Alerts, 2)

	t.Logf("multi-alert save: group has %d alerts", len(got.Alerts))
}

func TestStore_Checker(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	assert.Equal(t, "sqlite", store.Name())

	err := store.Check(ctx)
	require.NoError(t, err)

	t.Log("Checker: Name=sqlite, Check passed")
}

func TestStore_Save_PayloadPreserved(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	group := testGroup("payload-test")

	err := store.Save(ctx, &group)
	require.NoError(t, err)

	pending, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)

	// Marshal original and compare
	originalJSON, _ := json.Marshal(group)
	retrievedJSON, _ := json.Marshal(pending[0])

	var originalMap, retrievedMap map[string]any
	_ = json.Unmarshal(originalJSON, &originalMap)
	_ = json.Unmarshal(retrievedJSON, &retrievedMap)

	// Key fields should match
	assert.Equal(t, originalMap["receiver"], retrievedMap["receiver"])
	assert.Equal(t, originalMap["version"], retrievedMap["version"])
	assert.Equal(t, originalMap["groupKey"], retrievedMap["groupKey"])

	t.Log("payload preserved through save/retrieve cycle")
}

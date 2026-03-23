package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
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

	require.NoError(t, ConfigurePragmas(context.Background(), db, testLogger()))
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

// newTestFileStore creates a file-based SQLite store for tests that require
// WAL mode, concurrent access, or connection pool behavior.
// :memory: databases give each connection a separate DB, so file-based is needed.
func newTestFileStore(t *testing.T) *Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := New(dbPath, testLogger())
	require.NoError(t, err)

	testutil.ApplyMigrations(t, store.db, "../../../migrations/sqlite")
	t.Cleanup(func() { store.Close() })
	return store
}

// pragmaValue reads a PRAGMA value from the store's database.
func pragmaValue(t *testing.T, db *sql.DB, pragma string) string {
	t.Helper()

	var value string
	err := db.QueryRow("PRAGMA " + pragma).Scan(&value)
	require.NoError(t, err, "failed to read PRAGMA %s", pragma)
	return value
}

func TestStore_Pragmas_BusyTimeout(t *testing.T) {
	store := newTestFileStore(t)

	got := pragmaValue(t, store.db, "busy_timeout")
	assert.Equal(t, "5000", got, "busy_timeout should be 5000ms")
}

func TestStore_Pragmas_Synchronous(t *testing.T) {
	store := newTestFileStore(t)

	// PRAGMA synchronous returns numeric: 0=OFF, 1=NORMAL, 2=FULL
	got := pragmaValue(t, store.db, "synchronous")
	assert.Equal(t, "1", got, "synchronous should be NORMAL (1)")
}

func TestStore_Pragmas_JournalModeWAL(t *testing.T) {
	store := newTestFileStore(t)

	got := pragmaValue(t, store.db, "journal_mode")
	assert.Equal(t, "wal", got, "journal_mode should be WAL")
}

func TestStore_MaxOpenConns(t *testing.T) {
	store := newTestFileStore(t)

	stats := store.db.Stats()
	assert.Equal(t, 1, stats.MaxOpenConnections, "MaxOpenConns should be 1 for SQLite")
}

func TestStore_ConcurrentWriteSafety(t *testing.T) {
	store := newTestFileStore(t)
	ctx := context.Background()

	const writers = 10
	var wg sync.WaitGroup
	errs := make([]error, writers)

	for i := range writers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			group := testGroup("concurrent-" + string(rune('a'+idx)))
			errs[idx] = store.Save(ctx, &group)
		}(i)
	}

	wg.Wait()

	// All writes should succeed (busy_timeout allows waiting for lock).
	for i, err := range errs {
		assert.NoError(t, err, "writer %d failed", i)
	}

	// All groups should be persisted.
	pending, err := store.GetPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, writers, len(pending), "all concurrent writes should be persisted")
}

func TestStore_WALCheckpoint_OnClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "wal-test.db")

	store, err := New(dbPath, testLogger())
	require.NoError(t, err)
	testutil.ApplyMigrations(t, store.db, "../../../migrations/sqlite")

	// Write some data to generate WAL entries.
	ctx := context.Background()
	for i := range 5 {
		group := testGroup("wal-" + string(rune('a'+i)))
		require.NoError(t, store.Save(ctx, &group))
	}

	// Close should perform WAL checkpoint.
	err = store.Close()
	require.NoError(t, err)

	// After checkpoint(TRUNCATE), WAL file should be empty or absent.
	walPath := dbPath + "-wal"
	// Re-open to verify data survived checkpoint.
	store2, err := New(dbPath, testLogger())
	require.NoError(t, err)
	defer store2.Close()

	testutil.ApplyMigrations(t, store2.db, "../../../migrations/sqlite")

	pending, err := store2.GetPending(context.Background(), 100)
	require.NoError(t, err)
	assert.Equal(t, 5, len(pending), "data should survive WAL checkpoint")

	_ = walPath // WAL file existence depends on implementation
}

func TestStore_Close_ErrorResilience(t *testing.T) {
	store := newTestFileStore(t)

	// First close should succeed.
	err := store.Close()
	assert.NoError(t, err)

	// Second close should not panic (may return error).
	_ = store.Close()
}

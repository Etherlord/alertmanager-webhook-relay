// Package sqlite provides a SQLite-based implementation of the alerts.Store interface.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"alertmanager-webhook-relay/internal/alerts"

	_ "modernc.org/sqlite"
)

// Store implements alerts.Store and server.Checker using SQLite (modernc.org/sqlite, pure Go).
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

// ConfigurePragmas enables WAL mode, foreign key constraints, busy timeout,
// and synchronous mode on a SQLite connection.
func ConfigurePragmas(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	pragmas := []struct {
		sql  string
		name string
	}{
		{"PRAGMA journal_mode=WAL", "journal_mode=WAL"},
		{"PRAGMA foreign_keys=ON", "foreign_keys=ON"},
		{"PRAGMA busy_timeout=5000", "busy_timeout=5000"},
		{"PRAGMA synchronous=NORMAL", "synchronous=NORMAL"},
	}

	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p.sql); err != nil {
			return fmt.Errorf("sqlite pragma %s: %w", p.name, err)
		}
		logger.Debug("SQLite pragma set", "pragma", p.name)
	}

	return nil
}

// New creates a new SQLite store, enables WAL mode, foreign keys, busy timeout,
// and sets connection pool to single connection.
// Migrations must be applied externally (goose) before calling New.
func New(dsn string, logger *slog.Logger) (*Store, error) {
	logger.Debug("opening SQLite database", "dsn", dsn)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	// Serialize all writes through a single connection.
	// SQLite only supports one writer at a time; multiple connections cause SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	logger.Debug("SQLite connection pool configured", "max_open_conns", 1)

	ctx := context.Background()
	if err := ConfigurePragmas(ctx, db, logger); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db, logger: logger}

	logger.Info("SQLite store initialized", "dsn", dsn)
	return s, nil
}

// Save persists an alert group. Idempotent — upserts by GroupKey.
func (s *Store) Save(ctx context.Context, group *alerts.AlertGroup) error {
	s.logger.Debug("saving alert group", "group_key", group.GroupKey, "alerts_count", len(group.Alerts))

	payload, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("sqlite marshal payload: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is no-op

	// Upsert alert_group.
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO alert_groups (id, group_key, receiver, status, external_url, payload)
		VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?)
		ON CONFLICT(group_key) DO UPDATE SET
			receiver = excluded.receiver,
			status = excluded.status,
			external_url = excluded.external_url,
			payload = excluded.payload,
			received_at = datetime('now'),
			notification_status = 'pending'
		RETURNING id
	`, group.GroupKey, group.Receiver, string(group.Status), group.ExternalURL, string(payload)).Scan(&id)
	if err != nil {
		return fmt.Errorf("sqlite upsert alert_group: %w", err)
	}

	s.logger.Debug("upserted alert group", "id", id, "group_key", group.GroupKey)

	// Delete old alerts for this group (on upsert, replace them).
	if _, err := tx.ExecContext(ctx, "DELETE FROM alerts WHERE alert_group_id = ?", id); err != nil {
		return fmt.Errorf("sqlite delete old alerts: %w", err)
	}

	// Insert new alerts.
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO alerts (id, alert_group_id, fingerprint, status, alertname, severity, starts_at, ends_at)
		VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("sqlite prepare alert insert: %w", err)
	}
	defer stmt.Close()

	for i, a := range group.Alerts {
		var endsAt *string
		if !a.EndsAt.IsZero() {
			formatted := a.EndsAt.UTC().Format("2006-01-02T15:04:05Z")
			endsAt = &formatted
		}

		_, err := stmt.ExecContext(ctx,
			id,
			a.Fingerprint,
			string(a.Status),
			a.Labels["alertname"],
			a.Labels["severity"],
			a.StartsAt.UTC().Format("2006-01-02T15:04:05Z"),
			endsAt,
		)
		if err != nil {
			return fmt.Errorf("sqlite insert alert[%d]: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite commit: %w", err)
	}

	s.logger.Debug("alert group saved", "id", id, "group_key", group.GroupKey, "alerts_count", len(group.Alerts))
	return nil
}

// GetPending returns up to limit alert groups with notification_status='pending'.
func (s *Store) GetPending(ctx context.Context, limit int) ([]alerts.AlertGroup, error) {
	s.logger.Debug("getting pending alert groups", "limit", limit)

	rows, err := s.db.QueryContext(ctx, `
		SELECT group_key, payload
		FROM alert_groups
		WHERE notification_status = 'pending'
		ORDER BY received_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite query pending: %w", err)
	}
	defer rows.Close()

	var groups []alerts.AlertGroup
	for rows.Next() {
		var groupKey, payload string
		if err := rows.Scan(&groupKey, &payload); err != nil {
			return nil, fmt.Errorf("sqlite scan pending row: %w", err)
		}

		var group alerts.AlertGroup
		if err := json.Unmarshal([]byte(payload), &group); err != nil {
			return nil, fmt.Errorf("sqlite unmarshal payload for group_key=%s: %w", groupKey, err)
		}

		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite rows iteration: %w", err)
	}

	s.logger.Debug("pending alert groups retrieved", "count", len(groups))
	return groups, nil
}

// MarkSent marks an alert group as sent by its group_key.
// Returns alerts.ErrNotFound if the group_key does not exist.
func (s *Store) MarkSent(ctx context.Context, groupKey string) error {
	s.logger.Debug("marking alert group as sent", "group_key", groupKey)

	result, err := s.db.ExecContext(ctx, `
		UPDATE alert_groups SET notification_status = 'sent' WHERE group_key = ?
	`, groupKey)
	if err != nil {
		return fmt.Errorf("sqlite mark sent: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite rows affected: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("group_key=%q: %w", groupKey, alerts.ErrNotFound)
	}

	s.logger.Debug("alert group marked as sent", "group_key", groupKey)
	return nil
}

// Name returns the checker name for readiness probes.
func (s *Store) Name() string {
	return "sqlite"
}

// Check performs a lightweight write test to verify the database is writable.
// This tests write lock acquisition, WAL write, and disk I/O —
// critical for rolling update readiness (new pod must be able to write).
func (s *Store) Check(ctx context.Context) error {
	result, err := s.db.ExecContext(ctx, "UPDATE health_check SET checked_at = datetime('now') WHERE id = 1")
	if err != nil {
		s.logger.Warn("readiness write check failed", "error", err)
		return fmt.Errorf("sqlite write check: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite write check rows affected: %w", err)
	}

	if rows != 1 {
		s.logger.Warn("readiness write check: health_check row missing")
		return fmt.Errorf("sqlite write check: expected 1 row affected, got %d", rows)
	}

	s.logger.Debug("readiness write check passed")
	return nil
}

// Close performs a WAL checkpoint and closes the database connection.
// The checkpoint ensures all WAL data is flushed to the main database file,
// which is critical during rolling updates to prevent data loss.
func (s *Store) Close() error {
	s.logger.Debug("closing SQLite store, performing WAL checkpoint")

	var busyPages, logPages, checkpointedPages int
	err := s.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busyPages, &logPages, &checkpointedPages)
	if err != nil {
		s.logger.Warn("WAL checkpoint failed, closing anyway", "error", err)
	} else {
		s.logger.Info("WAL checkpoint completed",
			"log_pages", logPages,
			"checkpointed_pages", checkpointedPages,
			"busy_pages", busyPages,
		)
	}

	return s.db.Close()
}

// Package sqlite provides a SQLite-based implementation of the alerts.Store interface.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"

	"alertmanager-webhook-relay/internal/alerts"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store implements alerts.Store and server.Checker using SQLite (modernc.org/sqlite, pure Go).
type Store struct {
	db *sql.DB
}

// New creates a new SQLite store, enables WAL mode and foreign keys,
// and runs embedded migrations.
func New(dsn string) (*Store, error) {
	slog.Debug("opening SQLite database", "dsn", dsn)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	// Enable WAL mode for concurrent reads.
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite enable WAL: %w", err)
	}

	// Enable foreign key constraints.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite enable foreign keys: %w", err)
	}

	slog.Debug("SQLite pragmas set", "journal_mode", "WAL", "foreign_keys", "ON")

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	slog.Info("SQLite store initialized", "dsn", dsn)
	return s, nil
}

// migrate runs embedded SQL migration files.
func (s *Store) migrate() error {
	slog.Debug("running SQLite migrations")

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("sqlite read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("sqlite read migration %s: %w", entry.Name(), err)
		}

		slog.Debug("applying migration", "file", entry.Name())

		if _, err := s.db.ExecContext(context.Background(), string(data)); err != nil {
			return fmt.Errorf("sqlite apply migration %s: %w", entry.Name(), err)
		}
	}

	slog.Debug("SQLite migrations complete")
	return nil
}

// Save persists an alert group. Idempotent — upserts by GroupKey.
func (s *Store) Save(ctx context.Context, group *alerts.AlertGroup) error {
	slog.Debug("saving alert group", "group_key", group.GroupKey, "alerts_count", len(group.Alerts))

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

	slog.Debug("upserted alert group", "id", id, "group_key", group.GroupKey)

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
			s := a.EndsAt.UTC().Format("2006-01-02T15:04:05Z")
			endsAt = &s
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

	slog.Debug("alert group saved", "id", id, "group_key", group.GroupKey, "alerts_count", len(group.Alerts))
	return nil
}

// GetPending returns up to limit alert groups with notification_status='pending'.
func (s *Store) GetPending(ctx context.Context, limit int) ([]alerts.AlertGroup, error) {
	slog.Debug("getting pending alert groups", "limit", limit)

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

	slog.Debug("pending alert groups retrieved", "count", len(groups))
	return groups, nil
}

// MarkSent marks an alert group as sent by its group_key.
// Returns alerts.ErrNotFound if the group_key does not exist.
func (s *Store) MarkSent(ctx context.Context, groupKey string) error {
	slog.Debug("marking alert group as sent", "group_key", groupKey)

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

	slog.Debug("alert group marked as sent", "group_key", groupKey)
	return nil
}

// Name returns the checker name for readiness probes.
func (s *Store) Name() string {
	return "sqlite"
}

// Check verifies the database connection is alive.
func (s *Store) Check(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close closes the database connection.
func (s *Store) Close() error {
	slog.Debug("closing SQLite store")
	return s.db.Close()
}

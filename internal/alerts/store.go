package alerts

import (
	"context"
	"errors"
)

// ErrNotFound indicates that the requested alert group was not found.
var ErrNotFound = errors.New("alert group not found")

// Store defines the persistence interface for alert groups.
// Implementations reside in storage/sqlite/ and storage/postgres/.
// Health check is separate — concrete stores implement server.Checker.
type Store interface {
	// Save persists an alert group. Idempotent — upserts by GroupKey
	// (ON CONFLICT UPDATE). Subsequent calls with the same GroupKey
	// update the existing record.
	Save(ctx context.Context, group *AlertGroup) error

	// GetPending returns up to limit alert groups with notification_status='pending',
	// ordered by received_at ASC (oldest first).
	GetPending(ctx context.Context, limit int) ([]AlertGroup, error)

	// MarkSent marks an alert group as sent by its ID.
	// Returns ErrNotFound if the ID does not exist.
	MarkSent(ctx context.Context, id string) error
}

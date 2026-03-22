package notify

import (
	"context"

	"alertmanager-webhook-relay/internal/alerts"
)

// Store defines the consumer-side persistence interface for the notification system.
// Method signatures match alerts.Store — sqlite.Store satisfies both interfaces
// without modification.
type Store interface {
	// GetPending returns up to limit alert groups with notification_status='pending',
	// ordered by received_at ASC (oldest first).
	GetPending(ctx context.Context, limit int) ([]alerts.AlertGroup, error)

	// MarkSent marks an alert group as sent by its group key.
	MarkSent(ctx context.Context, groupKey string) error
}

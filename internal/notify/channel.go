package notify

import "context"

// Channel defines the interface for notification delivery channels.
// Implementations reside in channel/pachca/ and channel/email/.
type Channel interface {
	// Name returns the human-readable channel name (e.g., "pachca", "email").
	Name() string

	// Send delivers a notification through the channel.
	// Returns an error if delivery fails.
	Send(ctx context.Context, notification *Notification) error
}

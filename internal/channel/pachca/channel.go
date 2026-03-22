package pachca

import (
	"context"
	"log/slog"

	"alertmanager-webhook-relay/internal/notify"
)

// Channel implements notify.Channel for Pachca.
type Channel struct {
	client *Client
	chatID int
}

// NewChannel creates a new Pachca notification channel.
func NewChannel(baseURL, token string, chatID int, opts ...Option) *Channel {
	slog.Debug("creating pachca channel", "chat_id", chatID, "base_url", baseURL)
	return &Channel{
		client: NewClient(baseURL, token, opts...),
		chatID: chatID,
	}
}

// Name returns the channel name.
func (ch *Channel) Name() string {
	return "pachca"
}

// Send formats the notification and sends it to Pachca.
func (ch *Channel) Send(ctx context.Context, n *notify.Notification) error {
	slog.Debug("pachca channel: formatting notification",
		"group_key", n.GroupKey,
		"status", n.Status,
		"alerts_count", n.AlertsCount,
	)

	content := FormatNotification(n)

	slog.Debug("pachca channel: sending message",
		"chat_id", ch.chatID,
		"content_len", len(content),
	)

	if err := ch.client.SendMessage(ctx, ch.chatID, content); err != nil {
		slog.Error("pachca channel: failed to send notification",
			"chat_id", ch.chatID,
			"group_key", n.GroupKey,
			"error", err,
		)
		return err
	}

	slog.Info("pachca channel: notification sent",
		"chat_id", ch.chatID,
		"group_key", n.GroupKey,
		"status", n.Status,
		"alerts_count", n.AlertsCount,
	)
	return nil
}

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
	logger *slog.Logger
}

// NewChannel creates a new Pachca notification channel.
func NewChannel(baseURL, token string, chatID int, logger *slog.Logger, opts ...Option) *Channel {
	logger.Debug("creating pachca channel", "chat_id", chatID, "base_url", baseURL)
	return &Channel{
		client: NewClient(baseURL, token, logger, opts...),
		chatID: chatID,
		logger: logger,
	}
}

// Name returns the channel name.
func (ch *Channel) Name() string {
	return "pachca"
}

// Send formats the notification and sends it to Pachca.
func (ch *Channel) Send(ctx context.Context, n *notify.Notification) error {
	ch.logger.Debug("pachca channel: formatting notification",
		"group_key", n.GroupKey,
		"status", n.Status,
		"alerts_count", n.AlertsCount,
	)

	content := FormatNotification(n)

	ch.logger.Debug("pachca channel: sending message",
		"chat_id", ch.chatID,
		"content_len", len(content),
	)

	if err := ch.client.SendMessage(ctx, ch.chatID, content); err != nil {
		ch.logger.Error("pachca channel: failed to send notification",
			"chat_id", ch.chatID,
			"group_key", n.GroupKey,
			"error", err,
		)
		return err
	}

	ch.logger.Info("pachca channel: notification sent",
		"chat_id", ch.chatID,
		"group_key", n.GroupKey,
		"status", n.Status,
		"alerts_count", n.AlertsCount,
	)
	return nil
}

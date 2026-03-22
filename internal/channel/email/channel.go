package email

import (
	"context"
	"fmt"
	"log/slog"

	"alertmanager-webhook-relay/internal/notify"
)

// Channel implements notify.Channel for Email.
type Channel struct {
	sender        *Sender
	to            []string
	subjectPrefix string
	logger        *slog.Logger
}

// NewChannel creates a new Email notification channel.
func NewChannel(sender *Sender, to []string, subjectPrefix string, logger *slog.Logger) *Channel {
	logger.Debug("creating email channel",
		"to", to,
		"subject_prefix", subjectPrefix,
	)
	return &Channel{
		sender:        sender,
		to:            to,
		subjectPrefix: subjectPrefix,
		logger:        logger,
	}
}

// Name returns the channel name.
func (ch *Channel) Name() string {
	return "email"
}

// Send formats the notification and sends it via email.
func (ch *Channel) Send(_ context.Context, n *notify.Notification) error {
	ch.logger.Debug("email channel: formatting notification",
		"group_key", n.GroupKey,
		"status", n.Status,
		"alerts_count", n.AlertsCount,
	)

	subject := FormatSubject(n, ch.subjectPrefix)
	body := FormatBodyDefault(n)

	ch.logger.Debug("email channel: sending message",
		"to", ch.to,
		"subject", subject,
		"body_len", len(body),
	)

	if err := ch.sender.Send(ch.to, subject, body); err != nil {
		ch.logger.Error("email channel: failed to send notification",
			"to", ch.to,
			"group_key", n.GroupKey,
			"error", err,
		)
		return fmt.Errorf("email send: %w", err)
	}

	ch.logger.Info("email channel: notification sent",
		"to", ch.to,
		"group_key", n.GroupKey,
		"status", n.Status,
		"alerts_count", n.AlertsCount,
	)
	return nil
}

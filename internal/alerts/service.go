package alerts

import (
	"context"
	"fmt"
	"log/slog"
)

// Service orchestrates alert validation and persistence.
type Service struct {
	store     Store
	logger    *slog.Logger
	maxAlerts int
}

// NewService creates a new alert service.
func NewService(store Store, logger *slog.Logger, maxAlerts int) *Service {
	return &Service{
		store:     store,
		logger:    logger,
		maxAlerts: maxAlerts,
	}
}

// Receive validates and persists an incoming alert group.
func (s *Service) Receive(ctx context.Context, group *AlertGroup) error {
	s.logger.Debug("receiving alert group",
		"group_key", group.GroupKey,
		"receiver", group.Receiver,
		"status", group.Status,
		"alerts_count", len(group.Alerts),
	)

	if err := ValidatePayload(group, s.maxAlerts); err != nil {
		s.logger.Warn("payload validation failed",
			"group_key", group.GroupKey,
			"error", err,
		)
		return err
	}

	s.logger.Debug("payload validation passed", "group_key", group.GroupKey)

	if err := s.store.Save(ctx, group); err != nil {
		s.logger.Error("failed to save alert group",
			"group_key", group.GroupKey,
			"error", err,
		)
		return fmt.Errorf("save alert group: %w", err)
	}

	s.logger.Info("alert group received",
		"group_key", group.GroupKey,
		"receiver", group.Receiver,
		"status", group.Status,
		"alerts_count", len(group.Alerts),
	)

	return nil
}

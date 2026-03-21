package alerts

import (
	"errors"
	"fmt"
)

const (
	// maxStringLength is the maximum allowed length for individual string fields
	// in alert payloads (labels, annotations, fingerprint, URLs).
	// Protects against DoS via oversized strings.
	maxStringLength = 4096
)

// Sentinel errors for payload validation.
var (
	// ErrInvalidPayload indicates a structurally invalid webhook payload.
	ErrInvalidPayload = errors.New("invalid payload")

	// ErrPayloadTooLarge indicates the payload exceeds size limits.
	ErrPayloadTooLarge = errors.New("payload too large")
)

// ValidatePayload validates an AlertGroup received from Alertmanager webhook.
// maxAlerts limits the number of alerts per payload (DoS protection).
func ValidatePayload(group *AlertGroup, maxAlerts int) error {
	if group.GroupKey == "" {
		return fmt.Errorf("groupKey must be set: %w", ErrInvalidPayload)
	}

	if group.Receiver == "" {
		return fmt.Errorf("receiver must be set: %w", ErrInvalidPayload)
	}

	if group.Status == "" {
		return fmt.Errorf("group status must be set: %w", ErrInvalidPayload)
	}

	if group.Status != StatusFiring && group.Status != StatusResolved {
		return fmt.Errorf("invalid group status=%q, expected firing/resolved: %w",
			group.Status, ErrInvalidPayload)
	}

	if err := validateStringLength("groupKey", group.GroupKey); err != nil {
		return err
	}
	if err := validateStringLength("receiver", group.Receiver); err != nil {
		return err
	}
	if err := validateStringLength("externalURL", group.ExternalURL); err != nil {
		return err
	}

	if group.Version != "4" {
		return fmt.Errorf("unsupported version=%q, expected \"4\": %w", group.Version, ErrInvalidPayload)
	}

	if len(group.Alerts) == 0 {
		return fmt.Errorf("alerts must be non-empty: %w", ErrInvalidPayload)
	}

	if len(group.Alerts) > maxAlerts {
		return fmt.Errorf("alerts count %d exceeds max %d: %w",
			len(group.Alerts), maxAlerts, ErrPayloadTooLarge)
	}

	for i := range group.Alerts {
		if err := validateAlert(&group.Alerts[i], i); err != nil {
			return err
		}
	}

	return nil
}

// validateAlert checks required fields and string length limits for a single alert.
func validateAlert(alert *Alert, index int) error {
	prefix := fmt.Sprintf("alerts[%d]", index)

	if alert.Fingerprint == "" {
		return fmt.Errorf("%s: fingerprint must be set: %w", prefix, ErrInvalidPayload)
	}

	if alert.Status == "" {
		return fmt.Errorf("%s: status must be set: %w", prefix, ErrInvalidPayload)
	}

	if alert.Status != StatusFiring && alert.Status != StatusResolved {
		return fmt.Errorf("%s: invalid status=%q, expected firing/resolved: %w",
			prefix, alert.Status, ErrInvalidPayload)
	}

	if alert.Labels["alertname"] == "" {
		return fmt.Errorf("%s: labels.alertname must be set: %w", prefix, ErrInvalidPayload)
	}

	// String length limits (DoS protection).
	if err := validateStringLength(prefix+".fingerprint", alert.Fingerprint); err != nil {
		return err
	}
	if err := validateStringLength(prefix+".generatorURL", alert.GeneratorURL); err != nil {
		return err
	}
	if err := validateMapLengths(prefix+".labels", alert.Labels); err != nil {
		return err
	}
	if err := validateMapLengths(prefix+".annotations", alert.Annotations); err != nil {
		return err
	}

	return nil
}

// validateStringLength checks that a string field does not exceed maxStringLength.
func validateStringLength(field, value string) error {
	if len(value) > maxStringLength {
		return fmt.Errorf("%s: length %d exceeds max %d: %w",
			field, len(value), maxStringLength, ErrPayloadTooLarge)
	}
	return nil
}

// validateMapLengths checks that all keys and values in a map do not exceed maxStringLength.
func validateMapLengths(field string, m map[string]string) error {
	for k, v := range m {
		if len(k) > maxStringLength {
			return fmt.Errorf("%s: key length %d exceeds max %d: %w",
				field, len(k), maxStringLength, ErrPayloadTooLarge)
		}
		if len(v) > maxStringLength {
			return fmt.Errorf("%s[%s]: value length %d exceeds max %d: %w",
				field, k, len(v), maxStringLength, ErrPayloadTooLarge)
		}
	}
	return nil
}

-- 001_create_alerts.sql
-- Alert groups and individual alerts tables for Alertmanager webhook relay.

CREATE TABLE IF NOT EXISTS alert_groups (
    id                  TEXT PRIMARY KEY,
    group_key           TEXT NOT NULL UNIQUE,
    receiver            TEXT NOT NULL,
    status              TEXT NOT NULL,
    external_url        TEXT NOT NULL DEFAULT '',
    payload             TEXT NOT NULL,
    received_at         DATETIME NOT NULL DEFAULT (datetime('now')),
    notification_status TEXT NOT NULL DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS alerts (
    id             TEXT PRIMARY KEY,
    alert_group_id TEXT NOT NULL REFERENCES alert_groups(id) ON DELETE CASCADE,
    fingerprint    TEXT NOT NULL,
    status         TEXT NOT NULL,
    alertname      TEXT NOT NULL,
    severity       TEXT NOT NULL DEFAULT '',
    starts_at      DATETIME NOT NULL,
    ends_at        DATETIME
);

CREATE INDEX IF NOT EXISTS idx_alert_groups_notification_status ON alert_groups(notification_status);
CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX IF NOT EXISTS idx_alerts_alert_group_id ON alerts(alert_group_id);

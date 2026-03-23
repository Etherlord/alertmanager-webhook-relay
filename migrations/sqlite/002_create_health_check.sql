-- +goose Up
-- 002_create_health_check.sql
-- Health check table for write-aware readiness probe.
-- Single-row table: UPDATE tests write lock acquisition, WAL write, and disk I/O.

CREATE TABLE health_check (
    id         INTEGER PRIMARY KEY CHECK (id = 1),
    checked_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO health_check (id) VALUES (1);

-- +goose Down

DROP TABLE health_check;

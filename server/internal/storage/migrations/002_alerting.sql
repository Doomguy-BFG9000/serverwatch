CREATE TABLE IF NOT EXISTS monitor_alert_state (
    monitor_id BIGINT PRIMARY KEY REFERENCES monitors(id) ON DELETE CASCADE,
    last_status VARCHAR(30),
    last_down_alert_at TIMESTAMPTZ,
    last_recovery_alert_at TIMESTAMPTZ,
    ssl_expiry_not_after TIMESTAMPTZ,
    ssl_warned_30 BOOLEAN NOT NULL DEFAULT FALSE,
    ssl_warned_14 BOOLEAN NOT NULL DEFAULT FALSE,
    ssl_warned_7 BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

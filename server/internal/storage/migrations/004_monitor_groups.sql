CREATE TABLE IF NOT EXISTS monitor_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_groups_name_lower
    ON monitor_groups (LOWER(name));

ALTER TABLE monitors
    ADD COLUMN IF NOT EXISTS group_id BIGINT
    REFERENCES monitor_groups(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_monitors_group_id
    ON monitors (group_id);

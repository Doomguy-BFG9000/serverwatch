-- Phase 0.51: data retention.
--
-- monitor_results grows without bound on a busy install: each monitor writes a
-- row every interval. The primary key index is (monitor_id, checked_at DESC),
-- but the retention job deletes globally by checked_at, so it needs an index
-- that starts with checked_at. Without it the prune degrades to a sequential
-- scan as the table grows.
CREATE INDEX IF NOT EXISTS idx_monitor_results_checked_at
    ON monitor_results (checked_at);

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

var ErrMonitorNotFound = errors.New("monitor not found")

type MonitorRepository struct {
	DB *pgxpool.Pool
}

func encodeMonitorConfig(config *service.MonitorConfig) ([]byte, error) {
	if config == nil {
		return []byte(`{}`), nil
	}
	return json.Marshal(config)
}

func decodeMonitorConfig(raw []byte) *service.MonitorConfig {
	if len(raw) == 0 || string(raw) == "{}" {
		return nil
	}
	var config service.MonitorConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil
	}
	return &config
}

func (r *MonitorRepository) Create(
	ctx context.Context,
	input service.CreateMonitorInput,
) (service.Monitor, error) {
	var monitor service.Monitor
	config, err := encodeMonitorConfig(input.Config)
	if err != nil {
		return monitor, err
	}

	err = r.DB.QueryRow(
		ctx,
		`
		WITH inserted AS (
			INSERT INTO monitors (
				name,
				monitor_type,
				target,
				config,
				interval_seconds,
				timeout_seconds,
				group_id
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING
				id,
				name,
				monitor_type,
				target,
				config,
				interval_seconds,
				timeout_seconds,
				enabled,
				maintenance,
				group_id,
				created_at,
				updated_at
		)
		SELECT
			inserted.id,
			inserted.name,
			inserted.monitor_type,
			inserted.target,
			inserted.config,
			inserted.interval_seconds,
			inserted.timeout_seconds,
			inserted.enabled,
			inserted.maintenance,
			inserted.group_id,
			COALESCE(monitor_groups.name, ''),
			inserted.created_at,
			inserted.updated_at
		FROM inserted
		LEFT JOIN monitor_groups ON monitor_groups.id = inserted.group_id
		`,
		input.Name,
		input.MonitorType,
		input.Target,
		config,
		input.IntervalSeconds,
		input.TimeoutSeconds,
		input.GroupID,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.MonitorType,
		&monitor.Target,
		&config,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Enabled,
		&monitor.Maintenance,
		&monitor.GroupID,
		&monitor.GroupName,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

	monitor.Config = decodeMonitorConfig(config)
	if isForeignKeyViolation(err) {
		return service.Monitor{}, ErrGroupNotFound
	}
	return monitor, err
}

func (r *MonitorRepository) List(
	ctx context.Context,
) ([]service.Monitor, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT
			m.id,
			m.name,
			m.monitor_type,
			m.target,
			m.config,
			m.interval_seconds,
			m.timeout_seconds,
			m.enabled,
			m.maintenance,
			m.group_id,
			COALESCE(monitor_groups.name, ''),
			m.created_at,
			m.updated_at
		FROM monitors m
		LEFT JOIN monitor_groups ON monitor_groups.id = m.group_id
		ORDER BY m.id DESC
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monitors := make([]service.Monitor, 0)

	for rows.Next() {
		var monitor service.Monitor
		var config []byte

		if err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.MonitorType,
			&monitor.Target,
			&config,
			&monitor.IntervalSeconds,
			&monitor.TimeoutSeconds,
			&monitor.Enabled,
			&monitor.Maintenance,
			&monitor.GroupID,
			&monitor.GroupName,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		); err != nil {
			return nil, err
		}
		monitor.Config = decodeMonitorConfig(config)

		monitors = append(monitors, monitor)
	}

	return monitors, rows.Err()
}

func (r *MonitorRepository) GetByID(
	ctx context.Context,
	id int64,
) (service.Monitor, error) {
	var monitor service.Monitor
	var config []byte

	err := r.DB.QueryRow(
		ctx,
		`
		SELECT
			m.id,
			m.name,
			m.monitor_type,
			m.target,
			m.config,
			m.interval_seconds,
			m.timeout_seconds,
			m.enabled,
			m.maintenance,
			m.group_id,
			COALESCE(monitor_groups.name, ''),
			m.created_at,
			m.updated_at
		FROM monitors m
		LEFT JOIN monitor_groups ON monitor_groups.id = m.group_id
		WHERE m.id = $1
		`,
		id,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.MonitorType,
		&monitor.Target,
		&config,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Enabled,
		&monitor.Maintenance,
		&monitor.GroupID,
		&monitor.GroupName,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)
	monitor.Config = decodeMonitorConfig(config)

	if errors.Is(err, pgx.ErrNoRows) {
		return service.Monitor{}, ErrMonitorNotFound
	}

	return monitor, err
}

func (r *MonitorRepository) Delete(
	ctx context.Context,
	id int64,
) error {
	tag, err := r.DB.Exec(
		ctx,
		`DELETE FROM monitors WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrMonitorNotFound
	}

	return nil
}

// listDueQuery intentionally does not join or filter on monitor_groups.
// Grouping is metadata only, and every due monitor (grouped or ungrouped) must
// remain eligible for scheduler dispatch.
const listDueQuery = `
	SELECT
		m.id,
		m.name,
		m.monitor_type,
		m.target,
		m.config,
		m.interval_seconds,
		m.timeout_seconds,
		m.enabled,
		m.created_at,
		m.updated_at
	FROM monitors m
	LEFT JOIN LATERAL (
		SELECT checked_at
		FROM monitor_results
		WHERE monitor_id = m.id
		ORDER BY checked_at DESC
		LIMIT 1
	) last_result ON TRUE
	WHERE
		m.enabled = TRUE
		AND m.maintenance = FALSE
		AND m.monitor_type IN ('http', 'https', 'tcp', 'ssl', 'ping', 'dns', 'port')
		AND (
			last_result.checked_at IS NULL
			OR last_result.checked_at <=
				NOW() - (m.interval_seconds * INTERVAL '1 second')
		)
	ORDER BY
		COALESCE(
			last_result.checked_at,
			'1970-01-01 00:00:00+00'::timestamptz
		) ASC
	LIMIT $1
`

func (r *MonitorRepository) ListDue(
	ctx context.Context,
	limit int,
) ([]service.Monitor, error) {
	rows, err := r.DB.Query(
		ctx,
		listDueQuery,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monitors := make([]service.Monitor, 0)

	for rows.Next() {
		var monitor service.Monitor
		var config []byte

		if err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.MonitorType,
			&monitor.Target,
			&config,
			&monitor.IntervalSeconds,
			&monitor.TimeoutSeconds,
			&monitor.Enabled,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		); err != nil {
			return nil, err
		}
		monitor.Config = decodeMonitorConfig(config)

		monitors = append(monitors, monitor)
	}

	return monitors, rows.Err()
}

func (r *MonitorRepository) SaveResult(
	ctx context.Context,
	input service.CreateMonitorResultInput,
) error {
	_, err := r.DB.Exec(
		ctx,
		`
		INSERT INTO monitor_results (
			monitor_id,
			status,
			response_time_ms,
			status_code,
			message
		)
		VALUES ($1, $2, $3, $4, $5)
		`,
		input.MonitorID,
		input.Status,
		input.ResponseTimeMS,
		input.StatusCode,
		input.Message,
	)

	return err
}

func (r *MonitorRepository) EvaluateAlerts(ctx context.Context, observation service.AlertObservation) ([]service.AlertEvent, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO monitor_alert_state (monitor_id) VALUES ($1) ON CONFLICT (monitor_id) DO NOTHING`, observation.Monitor.ID)
	if err != nil {
		return nil, err
	}
	var state service.AlertState
	var lastStatus *string
	err = tx.QueryRow(ctx, `
		SELECT last_status, last_down_alert_at, last_recovery_alert_at,
		       ssl_expiry_not_after, ssl_warned_30, ssl_warned_14, ssl_warned_7
		FROM monitor_alert_state WHERE monitor_id = $1 FOR UPDATE`, observation.Monitor.ID).Scan(
		&lastStatus, &state.LastDownAlertAt, &state.LastRecoveryAlertAt, &state.SSLExpiryNotAfter,
		&state.SSLWarned30, &state.SSLWarned14, &state.SSLWarned7)
	if err != nil {
		return nil, err
	}
	state.LastStatus = ""
	if lastStatus != nil {
		state.LastStatus = *lastStatus
	}

	events := make([]service.AlertEvent, 0, 4)
	if observation.Status == "down" && (state.LastStatus == "" || state.LastStatus == "up") {
		events = append(events, service.AlertEvent{Type: "down"})
		state.LastDownAlertAt = timePtr(observation.CheckedAt)
	}
	if observation.Status == "up" && state.LastStatus == "down" {
		events = append(events, service.AlertEvent{Type: "recovery"})
		state.LastRecoveryAlertAt = timePtr(observation.CheckedAt)
	}
	if observation.SSL != nil && observation.Status == "up" {
		ssl := observation.SSL
		if state.SSLExpiryNotAfter == nil || !state.SSLExpiryNotAfter.Equal(ssl.NotAfter) {
			state.SSLExpiryNotAfter = timePtr(ssl.NotAfter)
			state.SSLWarned30, state.SSLWarned14, state.SSLWarned7 = false, false, false
		}
		if ssl.DaysRemaining <= 30 && !state.SSLWarned30 {
			events = append(events, service.AlertEvent{Type: "ssl_30"})
			state.SSLWarned30 = true
		}
		if ssl.DaysRemaining <= 14 && !state.SSLWarned14 {
			events = append(events, service.AlertEvent{Type: "ssl_14"})
			state.SSLWarned14 = true
		}
		if ssl.DaysRemaining <= 7 && !state.SSLWarned7 {
			events = append(events, service.AlertEvent{Type: "ssl_7"})
			state.SSLWarned7 = true
		}
	}
	_, err = tx.Exec(ctx, `UPDATE monitor_alert_state SET last_status=$2, last_down_alert_at=$3, last_recovery_alert_at=$4, ssl_expiry_not_after=$5, ssl_warned_30=$6, ssl_warned_14=$7, ssl_warned_7=$8, updated_at=NOW() WHERE monitor_id=$1`, observation.Monitor.ID, observation.Status, state.LastDownAlertAt, state.LastRecoveryAlertAt, state.SSLExpiryNotAfter, state.SSLWarned30, state.SSLWarned14, state.SSLWarned7)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return events, nil
}

func timePtr(value time.Time) *time.Time { return &value }
func (r *MonitorRepository) ListStatus(
	ctx context.Context,
) ([]service.MonitorStatus, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT
			m.id,
			m.name,
			m.monitor_type,
			m.target,
			m.config,
			m.enabled,
			m.maintenance,
			m.interval_seconds,
			m.timeout_seconds,
			COALESCE(lr.status, 'pending') AS status,
			lr.response_time_ms,
			lr.status_code,
			COALESCE(lr.message, ''),
			lr.checked_at,
			m.group_id,
			COALESCE(monitor_groups.name, '')
		FROM monitors m
		LEFT JOIN monitor_groups ON monitor_groups.id = m.group_id
		LEFT JOIN LATERAL (
			SELECT
				status,
				response_time_ms,
				status_code,
				message,
				checked_at
			FROM monitor_results
			WHERE monitor_id = m.id
			ORDER BY checked_at DESC
			LIMIT 1
		) lr ON TRUE
		ORDER BY m.id DESC
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := make([]service.MonitorStatus, 0)

	for rows.Next() {
		var item service.MonitorStatus
		var config []byte

		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.MonitorType,
			&item.Target,
			&config,
			&item.Enabled,
			&item.Maintenance,
			&item.IntervalSeconds,
			&item.TimeoutSeconds,
			&item.Status,
			&item.ResponseTimeMS,
			&item.StatusCode,
			&item.Message,
			&item.LastCheckedAt,
			&item.GroupID,
			&item.GroupName,
		); err != nil {
			return nil, err
		}
		item.Config = decodeMonitorConfig(config)

		statuses = append(statuses, item)
	}

	return statuses, rows.Err()
}

func (r *MonitorRepository) ListResults(
	ctx context.Context,
	monitorID int64,
	limit int,
) ([]service.MonitorResult, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT
			id,
			monitor_id,
			status,
			response_time_ms,
			status_code,
			COALESCE(message, ''),
			checked_at
		FROM monitor_results
		WHERE monitor_id = $1
		ORDER BY checked_at DESC
		LIMIT $2
		`,
		monitorID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]service.MonitorResult, 0)

	for rows.Next() {
		var result service.MonitorResult

		if err := rows.Scan(
			&result.ID,
			&result.MonitorID,
			&result.Status,
			&result.ResponseTimeMS,
			&result.StatusCode,
			&result.Message,
			&result.CheckedAt,
		); err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, rows.Err()
}
func (r *MonitorRepository) Update(
	ctx context.Context,
	id int64,
	input service.UpdateMonitorInput,
) (service.Monitor, error) {
	var monitor service.Monitor
	config, err := encodeMonitorConfig(input.Config)
	if err != nil {
		return monitor, err
	}

	err = r.DB.QueryRow(
		ctx,
		`
		WITH updated AS (
			UPDATE monitors
			SET
				name = $2,
				monitor_type = $3,
				target = $4,
				config = $5,
				interval_seconds = $6,
				timeout_seconds = $7,
				enabled = $8,
				group_id = CASE
					WHEN $9 THEN $10::bigint
					ELSE group_id
				END,
				maintenance = CASE
					WHEN $11 THEN $12::boolean
					ELSE maintenance
				END,
				updated_at = NOW()
			WHERE id = $1
			RETURNING
				id,
				name,
				monitor_type,
				target,
				config,
				interval_seconds,
				timeout_seconds,
				enabled,
				maintenance,
				group_id,
				created_at,
				updated_at
		)
		SELECT
			updated.id,
			updated.name,
			updated.monitor_type,
			updated.target,
			updated.config,
			updated.interval_seconds,
			updated.timeout_seconds,
			updated.enabled,
			updated.maintenance,
			updated.group_id,
			COALESCE(monitor_groups.name, ''),
			updated.created_at,
			updated.updated_at
		FROM updated
		LEFT JOIN monitor_groups ON monitor_groups.id = updated.group_id
		`,
		id,
		input.Name,
		input.MonitorType,
		input.Target,
		config,
		input.IntervalSeconds,
		input.TimeoutSeconds,
		input.Enabled,
		input.GroupID.Set,
		input.GroupID.Value,
		input.Maintenance.Set,
		input.Maintenance.Value,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.MonitorType,
		&monitor.Target,
		&config,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Enabled,
		&monitor.Maintenance,
		&monitor.GroupID,
		&monitor.GroupName,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)
	monitor.Config = decodeMonitorConfig(config)

	if errors.Is(err, pgx.ErrNoRows) {
		return service.Monitor{}, ErrMonitorNotFound
	}
	if isForeignKeyViolation(err) {
		return service.Monitor{}, ErrGroupNotFound
	}

	return monitor, err
}

// setMaintenanceQuery toggles only monitors.maintenance. It deliberately does
// not read or write monitor_results or monitor_alert_state, so entering or
// exiting maintenance never creates a synthetic result or an alert transition.
const setMaintenanceQuery = `
	WITH updated AS (
		UPDATE monitors
		SET
			maintenance = $2,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			name,
			monitor_type,
			target,
			config,
			interval_seconds,
			timeout_seconds,
			enabled,
			maintenance,
			group_id,
			created_at,
			updated_at
	)
	SELECT
		updated.id,
		updated.name,
		updated.monitor_type,
		updated.target,
		updated.config,
		updated.interval_seconds,
		updated.timeout_seconds,
		updated.enabled,
		updated.maintenance,
		updated.group_id,
		COALESCE(monitor_groups.name, ''),
		updated.created_at,
		updated.updated_at
	FROM updated
	LEFT JOIN monitor_groups ON monitor_groups.id = updated.group_id
`

// SetMaintenance toggles the manual maintenance flag for one monitor and
// returns the updated representation. It only touches monitors.maintenance, so
// check history and monitor_alert_state are left untouched and no scheduled
// check is run here.
func (r *MonitorRepository) SetMaintenance(
	ctx context.Context,
	id int64,
	maintenance bool,
) (service.Monitor, error) {
	var monitor service.Monitor
	var config []byte

	err := r.DB.QueryRow(
		ctx,
		setMaintenanceQuery,
		id,
		maintenance,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.MonitorType,
		&monitor.Target,
		&config,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Enabled,
		&monitor.Maintenance,
		&monitor.GroupID,
		&monitor.GroupName,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)
	monitor.Config = decodeMonitorConfig(config)

	if errors.Is(err, pgx.ErrNoRows) {
		return service.Monitor{}, ErrMonitorNotFound
	}

	return monitor, err
}

func (r *MonitorRepository) GetStats(
	ctx context.Context,
	monitorID int64,
) (service.MonitorStats, error) {
	var stats service.MonitorStats

	stats.MonitorID = monitorID

	err := r.DB.QueryRow(
		ctx,
		`
		SELECT
			COUNT(*) AS total_checks,
			COUNT(*) FILTER (WHERE status = 'up') AS up_checks,
			COUNT(*) FILTER (WHERE status = 'down') AS down_checks,
			COALESCE(
				ROUND(
					(
						COUNT(*) FILTER (WHERE status = 'up')
						::numeric
						/
						NULLIF(COUNT(*), 0)
					) * 100,
					2
				),
				0
			) AS uptime_percentage,
			COALESCE(
				ROUND(AVG(response_time_ms)::numeric, 2),
				0
			) AS average_response_ms
		FROM monitor_results
		WHERE monitor_id = $1
		`,
		monitorID,
	).Scan(
		&stats.TotalChecks,
		&stats.UpChecks,
		&stats.DownChecks,
		&stats.UptimePercentage,
		&stats.AverageResponseMS,
	)

	if err != nil {
		return stats, err
	}

	return stats, nil
}
func (r *MonitorRepository) ListResultsSince(
	ctx context.Context,
	monitorID int64,
	sinceSeconds int,
	limit int,
) ([]service.MonitorResult, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT
			id,
			monitor_id,
			status,
			response_time_ms,
			status_code,
			COALESCE(message, ''),
			checked_at
		FROM monitor_results
		WHERE monitor_id = $1
		  AND checked_at >= NOW() - make_interval(secs => $2)
		ORDER BY checked_at DESC
		LIMIT $3
		`,
		monitorID,
		sinceSeconds,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]service.MonitorResult, 0)

	for rows.Next() {
		var item service.MonitorResult

		if err := rows.Scan(
			&item.ID,
			&item.MonitorID,
			&item.Status,
			&item.ResponseTimeMS,
			&item.StatusCode,
			&item.Message,
			&item.CheckedAt,
		); err != nil {
			return nil, err
		}

		results = append(results, item)
	}

	return results, rows.Err()
}
func (r *MonitorRepository) GetStatsSince(
	ctx context.Context,
	monitorID int64,
	sinceSeconds int,
) (service.MonitorStats, error) {
	var stats service.MonitorStats

	stats.MonitorID = monitorID

	err := r.DB.QueryRow(
		ctx,
		`
		SELECT
			COUNT(*) AS total_checks,
			COUNT(*) FILTER (
				WHERE status = 'up'
			) AS up_checks,
			COUNT(*) FILTER (
				WHERE status = 'down'
			) AS down_checks,
			COALESCE(
				ROUND(
					(
						COUNT(*) FILTER (
							WHERE status = 'up'
						)::numeric
						/
						NULLIF(COUNT(*), 0)
					) * 100,
					2
				),
				0
			) AS uptime_percentage,
			COALESCE(
				ROUND(
					AVG(response_time_ms)::numeric,
					2
				),
				0
			) AS average_response_ms
		FROM monitor_results
		WHERE monitor_id = $1
		  AND checked_at >= NOW() - make_interval(secs => $2)
		`,
		monitorID,
		sinceSeconds,
	).Scan(
		&stats.TotalChecks,
		&stats.UpChecks,
		&stats.DownChecks,
		&stats.UptimePercentage,
		&stats.AverageResponseMS,
	)

	if err != nil {
		return stats, err
	}

	return stats, nil
}

func (r *MonitorRepository) ListChartPoints(
	ctx context.Context,
	monitorID int64,
	sinceSeconds int,
	bucketSeconds int,
) ([]service.MonitorChartPoint, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT
			to_timestamp(
				floor(
					extract(epoch FROM checked_at) / $3::double precision
				) * $3::double precision
			) AS bucket_checked_at,
			ROUND(AVG(response_time_ms)::numeric, 2) AS average_response_ms,
			COUNT(*) FILTER (WHERE status = 'up') AS up_checks,
			COUNT(*) FILTER (WHERE status = 'down') AS down_checks
		FROM monitor_results
		WHERE monitor_id = $1
		  AND checked_at >= NOW() - make_interval(secs => $2::double precision)
		GROUP BY bucket_checked_at
		ORDER BY bucket_checked_at ASC
		`,
		monitorID,
		sinceSeconds,
		bucketSeconds,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]service.MonitorChartPoint, 0)

	for rows.Next() {
		var point service.MonitorChartPoint

		if err := rows.Scan(
			&point.CheckedAt,
			&point.AverageResponseMS,
			&point.UpChecks,
			&point.DownChecks,
		); err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, rows.Err()
}

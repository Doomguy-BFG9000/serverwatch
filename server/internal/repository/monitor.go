package repository

import (
	"context"
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

func (r *MonitorRepository) Create(
	ctx context.Context,
	input service.CreateMonitorInput,
) (service.Monitor, error) {
	var monitor service.Monitor

	err := r.DB.QueryRow(
		ctx,
		`
		INSERT INTO monitors (
			name,
			monitor_type,
			target,
			interval_seconds,
			timeout_seconds
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			name,
			monitor_type,
			target,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		`,
		input.Name,
		input.MonitorType,
		input.Target,
		input.IntervalSeconds,
		input.TimeoutSeconds,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.MonitorType,
		&monitor.Target,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Enabled,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

	return monitor, err
}

func (r *MonitorRepository) List(
	ctx context.Context,
) ([]service.Monitor, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT
			id,
			name,
			monitor_type,
			target,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		FROM monitors
		ORDER BY id DESC
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monitors := make([]service.Monitor, 0)

	for rows.Next() {
		var monitor service.Monitor

		if err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.MonitorType,
			&monitor.Target,
			&monitor.IntervalSeconds,
			&monitor.TimeoutSeconds,
			&monitor.Enabled,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		); err != nil {
			return nil, err
		}

		monitors = append(monitors, monitor)
	}

	return monitors, rows.Err()
}

func (r *MonitorRepository) GetByID(
	ctx context.Context,
	id int64,
) (service.Monitor, error) {
	var monitor service.Monitor

	err := r.DB.QueryRow(
		ctx,
		`
		SELECT
			id,
			name,
			monitor_type,
			target,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		FROM monitors
		WHERE id = $1
		`,
		id,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.MonitorType,
		&monitor.Target,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Enabled,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

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
func (r *MonitorRepository) ListDue(
	ctx context.Context,
	limit int,
) ([]service.Monitor, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT
			m.id,
			m.name,
			m.monitor_type,
			m.target,
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
			AND m.monitor_type IN ('http', 'https', 'tcp', 'ssl')
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
		`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monitors := make([]service.Monitor, 0)

	for rows.Next() {
		var monitor service.Monitor

		if err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.MonitorType,
			&monitor.Target,
			&monitor.IntervalSeconds,
			&monitor.TimeoutSeconds,
			&monitor.Enabled,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		); err != nil {
			return nil, err
		}

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
			m.enabled,
			m.interval_seconds,
			m.timeout_seconds,
			COALESCE(lr.status, 'pending') AS status,
			lr.response_time_ms,
			lr.status_code,
			COALESCE(lr.message, ''),
			lr.checked_at
		FROM monitors m
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

		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.MonitorType,
			&item.Target,
			&item.Enabled,
			&item.IntervalSeconds,
			&item.TimeoutSeconds,
			&item.Status,
			&item.ResponseTimeMS,
			&item.StatusCode,
			&item.Message,
			&item.LastCheckedAt,
		); err != nil {
			return nil, err
		}

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

	err := r.DB.QueryRow(
		ctx,
		`
		UPDATE monitors
		SET
			name = $2,
			monitor_type = $3,
			target = $4,
			interval_seconds = $5,
			timeout_seconds = $6,
			enabled = $7,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			name,
			monitor_type,
			target,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		`,
		id,
		input.Name,
		input.MonitorType,
		input.Target,
		input.IntervalSeconds,
		input.TimeoutSeconds,
		input.Enabled,
	).Scan(
		&monitor.ID,
		&monitor.Name,
		&monitor.MonitorType,
		&monitor.Target,
		&monitor.IntervalSeconds,
		&monitor.TimeoutSeconds,
		&monitor.Enabled,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

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

package service

import "time"

type Monitor struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	MonitorType     string         `json:"monitor_type"`
	Target          string         `json:"target"`
	Config          *MonitorConfig `json:"config,omitempty"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Enabled         bool           `json:"enabled"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type MonitorConfig struct {
	RecordType     string   `json:"record_type,omitempty"`
	ExpectedValues []string `json:"expected_values,omitempty"`
}

type CreateMonitorInput struct {
	Name            string         `json:"name"`
	MonitorType     string         `json:"monitor_type"`
	Target          string         `json:"target"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Config          *MonitorConfig `json:"config,omitempty"`
}
type MonitorResult struct {
	ID             int64     `json:"id"`
	MonitorID      int64     `json:"monitor_id"`
	Status         string    `json:"status"`
	ResponseTimeMS int64     `json:"response_time_ms"`
	StatusCode     *int      `json:"status_code,omitempty"`
	Message        string    `json:"message,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}
type MonitorStats struct {
	MonitorID         int64   `json:"monitor_id"`
	TotalChecks       int64   `json:"total_checks"`
	UpChecks          int64   `json:"up_checks"`
	DownChecks        int64   `json:"down_checks"`
	UptimePercentage  float64 `json:"uptime_percentage"`
	AverageResponseMS float64 `json:"average_response_ms"`
}

type MonitorChartPoint struct {
	CheckedAt         time.Time `json:"checked_at"`
	AverageResponseMS float64   `json:"average_response_ms"`
	UpChecks          int64     `json:"up_checks"`
	DownChecks        int64     `json:"down_checks"`
}

type CreateMonitorResultInput struct {
	MonitorID      int64
	Status         string
	ResponseTimeMS int64
	StatusCode     *int
	Message        string
}

type AlertObservation struct {
	Monitor        Monitor
	Status         string
	ResponseTimeMS int64
	StatusCode     *int
	Message        string
	CheckedAt      time.Time
	SSL            *SSLAlertMetadata
}

type SSLAlertMetadata struct {
	Subject       string
	Issuer        string
	NotAfter      time.Time
	DaysRemaining int
}

type AlertEvent struct {
	Type string
}

type AlertState struct {
	LastStatus          string
	LastDownAlertAt     *time.Time
	LastRecoveryAlertAt *time.Time
	SSLExpiryNotAfter   *time.Time
	SSLWarned30         bool
	SSLWarned14         bool
	SSLWarned7          bool
}
type MonitorStatus struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	MonitorType     string         `json:"monitor_type"`
	Target          string         `json:"target"`
	Config          *MonitorConfig `json:"config,omitempty"`
	Enabled         bool           `json:"enabled"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Status          string         `json:"status"`
	ResponseTimeMS  *int64         `json:"response_time_ms,omitempty"`
	StatusCode      *int           `json:"status_code,omitempty"`
	Message         string         `json:"message,omitempty"`
	LastCheckedAt   *time.Time     `json:"last_checked_at,omitempty"`
}
type UpdateMonitorInput struct {
	Name            string         `json:"name"`
	MonitorType     string         `json:"monitor_type"`
	Target          string         `json:"target"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Enabled         bool           `json:"enabled"`
	Config          *MonitorConfig `json:"config,omitempty"`
}

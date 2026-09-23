package service

import (
	"encoding/json"
	"time"
)

type Monitor struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	MonitorType     string         `json:"monitor_type"`
	Target          string         `json:"target"`
	Config          *MonitorConfig `json:"config,omitempty"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Enabled         bool           `json:"enabled"`
	Maintenance     bool           `json:"maintenance"`
	GroupID         *int64         `json:"group_id"`
	GroupName       string         `json:"group_name"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type MonitorConfig struct {
	RecordType     string   `json:"record_type,omitempty"`
	ExpectedValues []string `json:"expected_values,omitempty"`
	Ports          []int    `json:"ports,omitempty"`
}

type CreateMonitorInput struct {
	Name            string         `json:"name"`
	MonitorType     string         `json:"monitor_type"`
	Target          string         `json:"target"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Config          *MonitorConfig `json:"config,omitempty"`
	GroupID         *int64         `json:"group_id"`
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
	Maintenance     bool           `json:"maintenance"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Status          string         `json:"status"`
	ResponseTimeMS  *int64         `json:"response_time_ms,omitempty"`
	StatusCode      *int           `json:"status_code,omitempty"`
	Message         string         `json:"message,omitempty"`
	LastCheckedAt   *time.Time     `json:"last_checked_at,omitempty"`
	GroupID         *int64         `json:"group_id"`
	GroupName       string         `json:"group_name"`
}

// OptionalInt64 distinguishes an omitted JSON field from an explicit null.
// Monitor updates use it so older clients can preserve a monitor's group while
// newer clients can explicitly remove the group with `"group_id": null`.
type OptionalInt64 struct {
	Set   bool
	Value *int64
}

func (value *OptionalInt64) UnmarshalJSON(data []byte) error {
	value.Set = true

	if string(data) == "null" {
		value.Value = nil
		return nil
	}

	var parsed int64
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	value.Value = &parsed
	return nil
}

// OptionalBool distinguishes an omitted JSON field from an explicit boolean.
// Monitor updates use it so older clients that never send "maintenance" preserve
// the monitor's current maintenance state, while newer clients can explicitly
// enter (true) or exit (false) maintenance.
type OptionalBool struct {
	Set   bool
	Value bool
}

func (value *OptionalBool) UnmarshalJSON(data []byte) error {
	value.Set = true

	var parsed bool
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	value.Value = parsed
	return nil
}

type UpdateMonitorInput struct {
	Name            string         `json:"name"`
	MonitorType     string         `json:"monitor_type"`
	Target          string         `json:"target"`
	IntervalSeconds int            `json:"interval_seconds"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Enabled         bool           `json:"enabled"`
	Maintenance     OptionalBool   `json:"maintenance"`
	Config          *MonitorConfig `json:"config,omitempty"`
	GroupID         OptionalInt64  `json:"group_id"`
}

package service

import "time"

type Monitor struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	MonitorType     string    `json:"monitor_type"`
	Target          string    `json:"target"`
	IntervalSeconds int       `json:"interval_seconds"`
	TimeoutSeconds  int       `json:"timeout_seconds"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateMonitorInput struct {
	Name            string `json:"name"`
	MonitorType     string `json:"monitor_type"`
	Target          string `json:"target"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
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

type CreateMonitorResultInput struct {
	MonitorID      int64
	Status         string
	ResponseTimeMS int64
	StatusCode     *int
	Message        string
}
type MonitorStatus struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	MonitorType    string     `json:"monitor_type"`
	Target         string     `json:"target"`
	Enabled        bool       `json:"enabled"`
	Status         string     `json:"status"`
	ResponseTimeMS *int64     `json:"response_time_ms,omitempty"`
	StatusCode     *int       `json:"status_code,omitempty"`
	Message        string     `json:"message,omitempty"`
	LastCheckedAt  *time.Time `json:"last_checked_at,omitempty"`
}

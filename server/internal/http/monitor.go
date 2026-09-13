package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	monitorengine "github.com/serverwatch/serverwatch/server/internal/monitor"
	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

type MonitorHandler struct {
	Repository *repository.MonitorRepository
	SSLChecker *monitorengine.SSLChecker
}

func (h *MonitorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input service.CreateMonitorInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_json",
		})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.MonitorType = strings.ToLower(strings.TrimSpace(input.MonitorType))
	input.Target = strings.TrimSpace(input.Target)

	if input.Name == "" || input.MonitorType == "" || input.Target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "name, monitor_type and target are required",
		})
		return
	}

	switch input.MonitorType {
	case "http", "https", "tcp", "ssl", "ping":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "unsupported monitor_type",
		})
		return
	}

	if input.IntervalSeconds == 0 {
		input.IntervalSeconds = 60
	}

	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = 10
	}

	if input.IntervalSeconds < 10 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "interval_seconds must be at least 10",
		})
		return
	}

	if input.TimeoutSeconds < 1 || input.TimeoutSeconds > 60 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "timeout_seconds must be between 1 and 60",
		})
		return
	}
	if input.MonitorType == "ssl" {
		if _, err := monitorengine.ParseSSLAddress(input.Target); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}
	if input.MonitorType == "ping" {
		if _, err := monitorengine.ParsePingTarget(input.Target); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}

	monitor, err := h.Repository.Create(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to create monitor",
		})
		return
	}

	writeJSON(w, http.StatusCreated, monitor)
}

func parseRangeSeconds(value string) (int, bool) {
	switch value {
	case "", "24h":
		return 24 * 60 * 60, true
	case "1h":
		return 60 * 60, true
	case "7d":
		return 7 * 24 * 60 * 60, true
	case "30d":
		return 30 * 24 * 60 * 60, true
	default:
		return 0, false
	}
}

func chartRangeConfig(value string) (int, int, bool) {
	switch value {
	case "", "24h":
		return 24 * 60 * 60, 5 * 60, true
	case "1h":
		return 60 * 60, 60, true
	case "7d":
		return 7 * 24 * 60 * 60, 60 * 60, true
	case "30d":
		return 30 * 24 * 60 * 60, 6 * 60 * 60, true
	default:
		return 0, 0, false
	}
}

func (h *MonitorHandler) List(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.Repository.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to list monitors",
		})
		return
	}

	writeJSON(w, http.StatusOK, monitors)
}

func (h *MonitorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseMonitorID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid monitor id",
		})
		return
	}

	monitor, err := h.Repository.GetByID(r.Context(), id)
	if errors.Is(err, repository.ErrMonitorNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "monitor not found",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to get monitor",
		})
		return
	}

	writeJSON(w, http.StatusOK, monitor)
}

func (h *MonitorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseMonitorID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid monitor id",
		})
		return
	}

	err = h.Repository.Delete(r.Context(), id)

	if errors.Is(err, repository.ErrMonitorNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "monitor not found",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to delete monitor",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseMonitorID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
func (h *MonitorHandler) Status(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.Repository.ListStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to load monitor status",
		})
		return
	}

	writeJSON(w, http.StatusOK, statuses)
}

func (h *MonitorHandler) Results(w http.ResponseWriter, r *http.Request) {
	idValue := r.PathValue("id")

	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid monitor id",
		})
		return
	}

	rangeValue := r.URL.Query().Get("range")

	sinceSeconds, ok := parseRangeSeconds(rangeValue)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid range",
		})
		return
	}

	limit := 500

	if limitValue := r.URL.Query().Get("limit"); limitValue != "" {
		parsedLimit, err := strconv.Atoi(limitValue)

		if err != nil || parsedLimit < 1 || parsedLimit > 5000 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid limit",
			})
			return
		}

		limit = parsedLimit
	}

	results, err := h.Repository.ListResultsSince(
		r.Context(),
		id,
		sinceSeconds,
		limit,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to load monitor results",
		})
		return
	}

	writeJSON(w, http.StatusOK, results)
}
func (h *MonitorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseMonitorID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid monitor id",
		})
		return
	}

	var input service.UpdateMonitorInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_json",
		})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.MonitorType = strings.ToLower(strings.TrimSpace(input.MonitorType))
	input.Target = strings.TrimSpace(input.Target)

	if input.Name == "" || input.MonitorType == "" || input.Target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "name, monitor_type and target are required",
		})
		return
	}

	switch input.MonitorType {
	case "http", "https", "tcp", "ssl", "ping":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "unsupported monitor_type",
		})
		return
	}

	if input.IntervalSeconds < 10 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "interval_seconds must be at least 10",
		})
		return
	}

	if input.TimeoutSeconds < 1 || input.TimeoutSeconds > 60 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "timeout_seconds must be between 1 and 60",
		})
		return
	}
	if input.MonitorType == "ssl" {
		if _, err := monitorengine.ParseSSLAddress(input.Target); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}
	if input.MonitorType == "ping" {
		if _, err := monitorengine.ParsePingTarget(input.Target); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}

	monitor, err := h.Repository.Update(
		r.Context(),
		id,
		input,
	)

	if errors.Is(err, repository.ErrMonitorNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "monitor not found",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to update monitor",
		})
		return
	}

	writeJSON(w, http.StatusOK, monitor)
}

func (h *MonitorHandler) SSL(w http.ResponseWriter, r *http.Request) {
	id, err := parseMonitorID(r)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid monitor id"})
		return
	}
	monitor, err := h.Repository.GetByID(r.Context(), id)
	if errors.Is(err, repository.ErrMonitorNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "monitor not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get monitor"})
		return
	}
	if monitor.MonitorType != "ssl" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "monitor is not an SSL monitor"})
		return
	}
	checker := h.SSLChecker
	if checker == nil {
		checker = monitorengine.NewSSLChecker()
	}
	checked := checker.CheckCertificate(r.Context(), monitor)
	response := struct {
		MonitorID        int64     `json:"monitor_id"`
		Status           string    `json:"status"`
		ResponseTimeMS   int64     `json:"response_time_ms"`
		Subject          string    `json:"subject"`
		Issuer           string    `json:"issuer"`
		NotBefore        time.Time `json:"not_before"`
		NotAfter         time.Time `json:"not_after"`
		DaysRemaining    int       `json:"days_remaining"`
		CertificateValid bool      `json:"certificate_valid"`
		Message          string    `json:"message"`
	}{monitor.ID, checked.Result.Status, checked.Result.ResponseTimeMS, checked.Subject, checked.Issuer, checked.NotBefore, checked.NotAfter, checked.DaysRemaining, checked.CertificateValid, checked.Result.Message}
	writeJSON(w, http.StatusOK, response)
}

func (h *MonitorHandler) Stats(w http.ResponseWriter, r *http.Request) {
	idValue := r.PathValue("id")

	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid monitor id",
		})
		return
	}

	rangeValue := r.URL.Query().Get("range")

	sinceSeconds, ok := parseRangeSeconds(rangeValue)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid range",
		})
		return
	}

	stats, err := h.Repository.GetStatsSince(
		r.Context(),
		id,
		sinceSeconds,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to load monitor stats",
		})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *MonitorHandler) Chart(w http.ResponseWriter, r *http.Request) {
	idValue := r.PathValue("id")

	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid monitor id",
		})
		return
	}

	sinceSeconds, bucketSeconds, ok := chartRangeConfig(
		r.URL.Query().Get("range"),
	)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid range",
		})
		return
	}

	points, err := h.Repository.ListChartPoints(
		r.Context(),
		id,
		sinceSeconds,
		bucketSeconds,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to load chart data",
		})
		return
	}

	writeJSON(w, http.StatusOK, points)
}

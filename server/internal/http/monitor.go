package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/service"
	"net/http"
	"strconv"
	"strings"
)

type MonitorHandler struct {
	Repository *repository.MonitorRepository
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
	case "http", "https", "tcp":
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

	monitor, err := h.Repository.Create(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to create monitor",
		})
		return
	}

	writeJSON(w, http.StatusCreated, monitor)
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
	id, err := parseMonitorID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid monitor id",
		})
		return
	}

	_, err = h.Repository.GetByID(r.Context(), id)
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

	limit := 50

	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, parseErr := strconv.Atoi(raw)

		if parseErr != nil || value < 1 || value > 500 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "limit must be between 1 and 500",
			})
			return
		}

		limit = value
	}

	results, err := h.Repository.ListResults(
		r.Context(),
		id,
		limit,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to load monitor results",
		})
		return
	}

	writeJSON(w, http.StatusOK, results)
}

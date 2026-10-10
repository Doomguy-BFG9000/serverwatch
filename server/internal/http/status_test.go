package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

type fakeStatusStore struct {
	statuses []service.MonitorStatus
	err      error
}

func (s fakeStatusStore) ListStatus(context.Context) ([]service.MonitorStatus, error) {
	return s.statuses, s.err
}

func ptrInt64(v int64) *int64 { return &v }

func ptrTime(v time.Time) *time.Time { return &v }

// TestStatusPageDisabledReturns404 guards that the feature is opt-in: with the
// handler disabled the route must be indistinguishable from an unknown path.
func TestStatusPageDisabledReturns404(t *testing.T) {
	handler := &StatusPageHandler{Enabled: false}

	for _, path := range []string{"/status", "/status.css"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if path == "/status" {
			handler.Status(recorder, request)
		} else {
			handler.Stylesheet(recorder, request)
		}
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s disabled status = %d, want 404", path, recorder.Code)
		}
	}
}

// TestStatusPageRendersStateAndSummary verifies the rendered page derives the
// same precedence the dashboard uses (disabled > maintenance > result) and that
// the summary counts match.
func TestStatusPageRendersStateAndSummary(t *testing.T) {
	checked := time.Now().UTC().Add(-2 * time.Minute)
	store := fakeStatusStore{statuses: []service.MonitorStatus{
		{Name: "API", MonitorType: "http", Enabled: true, Status: "up", ResponseTimeMS: ptrInt64(42), LastCheckedAt: ptrTime(checked), GroupName: "Prod"},
		{Name: "Site", MonitorType: "http", Enabled: true, Status: "down", GroupName: "Prod"},
		{Name: "Paused", MonitorType: "tcp", Enabled: false, Status: "down", GroupName: "Prod"},
		{Name: "Maint", MonitorType: "ping", Enabled: true, Maintenance: true, Status: "up", GroupName: "Prod"},
		{Name: "Fresh", MonitorType: "dns", Enabled: true, Status: "pending", GroupName: "Prod"},
	}}
	handler := &StatusPageHandler{Repository: store, Enabled: true}

	recorder := httptest.NewRecorder()
	handler.Status(recorder, httptest.NewRequest(http.MethodGet, "/status", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if ct := recorder.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q", ct)
	}
	body := recorder.Body.String()
	for _, want := range []string{
		"API", "Site", "Prod", "42 ms",
		"Operational", "Outage", "Maintenance", "Disabled", "No data",
		"Degraded service",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	// Target values are never rendered on the public page.
	if strings.Contains(body, "http://") || strings.Contains(body, "target") {
		t.Errorf("body leaked target/config data")
	}
}

// TestStatusPageSummaryCounts pins the exact counts the pages relies on.
func TestStatusPageSummaryCounts(t *testing.T) {
	data := buildStatusPage([]service.MonitorStatus{
		{Name: "a", Enabled: true, Status: "up"},
		{Name: "b", Enabled: true, Status: "down"},
		{Name: "c", Enabled: true, Status: "up"},
		{Name: "d", Enabled: false, Status: "down"},
		{Name: "e", Enabled: true, Maintenance: true, Status: "down"},
		{Name: "f", Enabled: true},
	}, time.Now().UTC())

	if data.Summary.Total != 6 || data.Summary.Up != 2 || data.Summary.Down != 1 ||
		data.Summary.Disabled != 1 || data.Summary.Maintenance != 1 || data.Summary.Pending != 1 {
		t.Fatalf("summary = %+v", data.Summary)
	}
	if data.Overall != "degraded" {
		t.Fatalf("overall = %q, want degraded", data.Overall)
	}
}

// TestStatusPageEmptyInstall covers a fresh install with no monitors.
func TestStatusPageEmptyInstall(t *testing.T) {
	data := buildStatusPage(nil, time.Now().UTC())
	if data.Overall != "empty" || data.Summary.Total != 0 {
		t.Fatalf("empty install data = %+v", data)
	}
}

func TestStatusPageStoreErrorIs500(t *testing.T) {
	handler := &StatusPageHandler{Repository: fakeStatusStore{err: errors.New("boom")}, Enabled: true}
	recorder := httptest.NewRecorder()
	handler.Status(recorder, httptest.NewRequest(http.MethodGet, "/status", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}

// TestStatusStoreMatchesMonitorStore ensures the narrow status interface does
// not drift from the real repository interface the handler also accepts.
func TestStatusStoreMatchesMonitorStore(t *testing.T) {
	var _ StatusStore = MonitorStore(nil)
}

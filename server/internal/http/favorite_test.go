package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

func callFavoriteHandler(
	t *testing.T,
	handler *MonitorHandler,
	id string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/monitors/"+id+"/favorite",
		strings.NewReader(body),
	)
	request.SetPathValue("id", id)
	recorder := httptest.NewRecorder()
	handler.SetFavorite(recorder, request)
	return recorder
}

func TestFavoriteAPIToggling(t *testing.T) {
	store := newFakeGroupMonitorStore()
	monitor := seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}

	if monitor.Favorite {
		t.Fatal("new monitor must default to favorite=false")
	}

	mark := callFavoriteHandler(t, handler, "1", `{"favorite":true}`)
	if mark.Code != http.StatusOK {
		t.Fatalf("mark favorite status = %d, body = %s", mark.Code, mark.Body.String())
	}
	var marked service.Monitor
	if err := json.Unmarshal(mark.Body.Bytes(), &marked); err != nil {
		t.Fatalf("decode marked monitor: %v", err)
	}
	if !marked.Favorite {
		t.Fatalf("monitor favorite = false, want true: %s", mark.Body.String())
	}

	repeatedMark := callFavoriteHandler(t, handler, "1", `{"favorite":true}`)
	if repeatedMark.Code != http.StatusOK {
		t.Fatalf("repeated mark status = %d, body = %s", repeatedMark.Code, repeatedMark.Body.String())
	}
	stored, err := store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if !stored.Favorite {
		t.Fatal("repeated mark did not keep favorite=true")
	}

	unmark := callFavoriteHandler(t, handler, "1", `{"favorite":false}`)
	if unmark.Code != http.StatusOK {
		t.Fatalf("unmark status = %d, body = %s", unmark.Code, unmark.Body.String())
	}
	var unmarked service.Monitor
	if err := json.Unmarshal(unmark.Body.Bytes(), &unmarked); err != nil {
		t.Fatalf("decode unmarked monitor: %v", err)
	}
	if unmarked.Favorite {
		t.Fatalf("monitor favorite = true, want false: %s", unmark.Body.String())
	}

	repeatedUnmark := callFavoriteHandler(t, handler, "1", `{"favorite":false}`)
	if repeatedUnmark.Code != http.StatusOK {
		t.Fatalf("repeated unmark status = %d, body = %s", repeatedUnmark.Code, repeatedUnmark.Body.String())
	}
	if stored, err = store.GetByID(context.Background(), 1); err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if stored.Favorite {
		t.Fatal("repeated unmark did not keep favorite=false")
	}
}

func TestFavoriteAPIErrors(t *testing.T) {
	store := newFakeGroupMonitorStore()
	seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}

	tests := []struct {
		name string
		id   string
		body string
		want int
	}{
		{"malformed id", "not-a-number", `{"favorite":true}`, http.StatusBadRequest},
		{"nonexistent monitor", "999", `{"favorite":true}`, http.StatusNotFound},
		{"malformed json", "1", `{"favorite":`, http.StatusBadRequest},
		{"missing field", "1", `{}`, http.StatusBadRequest},
		{"string value", "1", `{"favorite":"true"}`, http.StatusBadRequest},
		{"numeric value", "1", `{"favorite":1}`, http.StatusBadRequest},
		{"null value", "1", `{"favorite":null}`, http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := callFavoriteHandler(t, handler, test.id, test.body)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}

	// None of the rejected requests may have changed the stored state.
	stored, err := store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if stored.Favorite {
		t.Fatal("failed favorite requests must not change state")
	}
}

func TestFavoriteReturnedByListAndStatusAPI(t *testing.T) {
	store := newFakeGroupMonitorStore()
	seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}

	if recorder := callFavoriteHandler(t, handler, "1", `{"favorite":true}`); recorder.Code != http.StatusOK {
		t.Fatalf("mark favorite status = %d", recorder.Code)
	}

	statusRecorder := httptest.NewRecorder()
	handler.Status(statusRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil))
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d", statusRecorder.Code)
	}
	var statuses []service.MonitorStatus
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("decode statuses: %v", err)
	}
	if len(statuses) != 1 || !statuses[0].Favorite {
		t.Fatalf("statuses = %+v, want one monitor with favorite=true", statuses)
	}

	listRecorder := httptest.NewRecorder()
	handler.List(listRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list code = %d", listRecorder.Code)
	}
	var monitors []service.Monitor
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &monitors); err != nil {
		t.Fatalf("decode monitors: %v", err)
	}
	if len(monitors) != 1 || !monitors[0].Favorite {
		t.Fatalf("monitors = %+v, want one monitor with favorite=true", monitors)
	}
}

// TestFavoriteTogglePreservesUnrelatedState is the core guarantee of Phase
// 0.46: flipping favorite must never change enabled, maintenance, group, or
// config.
func TestFavoriteTogglePreservesUnrelatedState(t *testing.T) {
	store := newFakeGroupMonitorStore()
	groupHandler := &GroupHandler{Repository: store}
	monitorHandler := &MonitorHandler{Repository: store}

	if created := callGroupHandler(t, groupHandler, http.MethodPost, "", `{"name":"Production"}`); created.Code != http.StatusCreated {
		t.Fatalf("create group status = %d", created.Code)
	}
	if status, body := callMonitorHandler(
		t,
		monitorHandler,
		http.MethodPost,
		`{"name":"DNS","monitor_type":"dns","target":"example.com","group_id":1,"config":{"record_type":"A"}}`,
	); status != http.StatusCreated {
		t.Fatalf("create monitor status = %d, body = %s", status, body)
	}

	if recorder := callMaintenanceHandler(t, monitorHandler, "1", `{"maintenance":true}`); recorder.Code != http.StatusOK {
		t.Fatalf("enter maintenance status = %d", recorder.Code)
	}

	if recorder := callFavoriteHandler(t, monitorHandler, "1", `{"favorite":true}`); recorder.Code != http.StatusOK {
		t.Fatalf("mark favorite status = %d", recorder.Code)
	}

	stored, err := store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if !stored.Favorite {
		t.Fatal("favorite was not persisted")
	}
	if !stored.Enabled {
		t.Fatal("favorite toggle must not change enabled")
	}
	if !stored.Maintenance {
		t.Fatal("favorite toggle must not change maintenance")
	}
	if stored.GroupID == nil || *stored.GroupID != 1 || stored.GroupName != "Production" {
		t.Fatalf("favorite toggle must not change group: %+v", stored)
	}
	if stored.Config == nil || stored.Config.RecordType != "A" || stored.Target != "example.com" {
		t.Fatalf("favorite toggle must not change config: %+v", stored.Config)
	}

	if recorder := callFavoriteHandler(t, monitorHandler, "1", `{"favorite":false}`); recorder.Code != http.StatusOK {
		t.Fatalf("unmark favorite status = %d", recorder.Code)
	}
	stored, err = store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if stored.Favorite {
		t.Fatal("favorite=false was not persisted")
	}
	if !stored.Maintenance || stored.GroupID == nil || *stored.GroupID != 1 {
		t.Fatalf("unfavorite must not change unrelated state: %+v", stored)
	}
}

// TestGeneralUpdatePreservesFavorite covers the Phase 0.43/0.44 partial update
// helpers (enable/disable and group assignment): their PUT payloads omit
// favorite, so the backend must leave the stored value untouched.
func TestGeneralUpdatePreservesFavorite(t *testing.T) {
	store := newFakeGroupMonitorStore()
	seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}

	if recorder := callFavoriteHandler(t, handler, "1", `{"favorite":true}`); recorder.Code != http.StatusOK {
		t.Fatalf("mark favorite status = %d", recorder.Code)
	}

	// Payload shape sent by updateMonitorEnabled(): favorite is absent.
	enabledPayload := `{"name":"API","monitor_type":"https","target":"https://example.com","interval_seconds":60,"timeout_seconds":10,"enabled":false}`
	if status, body := callMonitorHandler(t, handler, http.MethodPut, enabledPayload); status != http.StatusOK {
		t.Fatalf("enabled update status = %d, body = %s", status, body)
	}
	stored, err := store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if !stored.Favorite {
		t.Fatal("enable/disable update must preserve favorite")
	}

	// Payload shape sent by updateMonitorGroup(): favorite is absent too.
	groupPayload := enabledPayload + `,"group_id":null`
	if status, body := callMonitorHandler(t, handler, http.MethodPut, groupPayload); status != http.StatusOK {
		t.Fatalf("group update status = %d, body = %s", status, body)
	}
	stored, err = store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if !stored.Favorite {
		t.Fatal("group update must preserve favorite")
	}
}

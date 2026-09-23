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

func callMaintenanceHandler(
	t *testing.T,
	handler *MonitorHandler,
	id string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/monitors/"+id+"/maintenance",
		strings.NewReader(body),
	)
	request.SetPathValue("id", id)
	recorder := httptest.NewRecorder()
	handler.SetMaintenance(recorder, request)
	return recorder
}

func seedMonitor(t *testing.T, store *fakeGroupMonitorStore) service.Monitor {
	t.Helper()

	monitor, err := store.Create(context.Background(), service.CreateMonitorInput{
		Name:            "API",
		MonitorType:     "https",
		Target:          "https://example.com",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
	})
	if err != nil {
		t.Fatalf("seed monitor: %v", err)
	}
	return monitor
}

func TestMaintenanceAPIToggling(t *testing.T) {
	store := newFakeGroupMonitorStore()
	monitor := seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}
	id := "1"

	if monitor.Maintenance {
		t.Fatal("new monitor must default to maintenance=false")
	}

	enter := callMaintenanceHandler(t, handler, id, `{"maintenance":true}`)
	if enter.Code != http.StatusOK {
		t.Fatalf("enter maintenance status = %d, body = %s", enter.Code, enter.Body.String())
	}
	var entered service.Monitor
	if err := json.Unmarshal(enter.Body.Bytes(), &entered); err != nil {
		t.Fatalf("decode entered monitor: %v", err)
	}
	if !entered.Maintenance {
		t.Fatalf("monitor maintenance = false, want true: %s", enter.Body.String())
	}

	repeatedEnter := callMaintenanceHandler(t, handler, id, `{"maintenance":true}`)
	if repeatedEnter.Code != http.StatusOK {
		t.Fatalf("repeated enter status = %d, body = %s", repeatedEnter.Code, repeatedEnter.Body.String())
	}
	stored, err := store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if !stored.Maintenance {
		t.Fatal("repeated enter did not keep maintenance=true")
	}

	exit := callMaintenanceHandler(t, handler, id, `{"maintenance":false}`)
	if exit.Code != http.StatusOK {
		t.Fatalf("exit maintenance status = %d, body = %s", exit.Code, exit.Body.String())
	}
	var exited service.Monitor
	if err := json.Unmarshal(exit.Body.Bytes(), &exited); err != nil {
		t.Fatalf("decode exited monitor: %v", err)
	}
	if exited.Maintenance {
		t.Fatalf("monitor maintenance = true, want false: %s", exit.Body.String())
	}

	repeatedExit := callMaintenanceHandler(t, handler, id, `{"maintenance":false}`)
	if repeatedExit.Code != http.StatusOK {
		t.Fatalf("repeated exit status = %d, body = %s", repeatedExit.Code, repeatedExit.Body.String())
	}
	if stored, err = store.GetByID(context.Background(), 1); err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if stored.Maintenance {
		t.Fatal("repeated exit did not keep maintenance=false")
	}
}

func TestMaintenanceAPIErrors(t *testing.T) {
	store := newFakeGroupMonitorStore()
	seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}

	tests := []struct {
		name string
		id   string
		body string
		want int
	}{
		{"malformed id", "not-a-number", `{"maintenance":true}`, http.StatusBadRequest},
		{"nonexistent monitor", "999", `{"maintenance":true}`, http.StatusNotFound},
		{"malformed json", "1", `{"maintenance":`, http.StatusBadRequest},
		{"missing field", "1", `{}`, http.StatusBadRequest},
		{"string value", "1", `{"maintenance":"true"}`, http.StatusBadRequest},
		{"numeric value", "1", `{"maintenance":1}`, http.StatusBadRequest},
		{"null value", "1", `{"maintenance":null}`, http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := callMaintenanceHandler(t, handler, test.id, test.body)
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
	if stored.Maintenance {
		t.Fatal("failed maintenance requests must not change state")
	}
}

func TestMaintenanceReturnedByStatusAPI(t *testing.T) {
	store := newFakeGroupMonitorStore()
	seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}

	if recorder := callMaintenanceHandler(t, handler, "1", `{"maintenance":true}`); recorder.Code != http.StatusOK {
		t.Fatalf("enter maintenance status = %d", recorder.Code)
	}

	recorder := httptest.NewRecorder()
	handler.Status(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d", recorder.Code)
	}
	var statuses []service.MonitorStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("decode statuses: %v", err)
	}
	if len(statuses) != 1 || !statuses[0].Maintenance {
		t.Fatalf("statuses = %+v, want one monitor with maintenance=true", statuses)
	}
}

func TestUpdatePreservesMaintenanceWhenOmitted(t *testing.T) {
	store := newFakeGroupMonitorStore()
	seedMonitor(t, store)
	handler := &MonitorHandler{Repository: store}

	if recorder := callMaintenanceHandler(t, handler, "1", `{"maintenance":true}`); recorder.Code != http.StatusOK {
		t.Fatalf("enter maintenance status = %d", recorder.Code)
	}

	updateBody := func(maintenanceJSON string) string {
		return `{"name":"API","monitor_type":"https","target":"https://example.com","interval_seconds":60,"timeout_seconds":10,"enabled":true` + maintenanceJSON + `}`
	}

	omitted, _ := callMonitorHandler(t, handler, http.MethodPut, updateBody(""))
	if omitted != http.StatusOK {
		t.Fatalf("update without maintenance status = %d", omitted)
	}
	stored, err := store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if !stored.Maintenance {
		t.Fatal("omitting maintenance must preserve the current value")
	}

	if status, _ := callMonitorHandler(t, handler, http.MethodPut, updateBody(`,"maintenance":false`)); status != http.StatusOK {
		t.Fatalf("explicit false status = %d", status)
	}
	if stored, err = store.GetByID(context.Background(), 1); err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if stored.Maintenance {
		t.Fatal("explicit false must clear maintenance")
	}

	if status, _ := callMonitorHandler(t, handler, http.MethodPut, updateBody(`,"maintenance":true`)); status != http.StatusOK {
		t.Fatalf("explicit true status = %d", status)
	}
	if stored, err = store.GetByID(context.Background(), 1); err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if !stored.Maintenance {
		t.Fatal("explicit true must set maintenance")
	}
}

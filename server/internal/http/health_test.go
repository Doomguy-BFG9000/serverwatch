package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/version"
)

// TestHealthExposesVersion guards the release-engineering contract that an
// operator can identify the running build from the health endpoint.
func TestHealthExposesVersion(t *testing.T) {
	version.Version = "v9.9.9"
	version.Commit = "deadbee"

	handler := &HealthHandler{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.Health(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["version"] != "v9.9.9" || body["commit"] != "deadbee" {
		t.Fatalf("health version fields = %#v", body)
	}
	if body["status"] != "ok" || body["service"] != "serverwatch" {
		t.Fatalf("health basics missing: %#v", body)
	}
}

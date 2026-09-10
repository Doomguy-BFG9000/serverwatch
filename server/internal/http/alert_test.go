package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/config"
)

func TestAlertStatusIsSafe(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := AlertHandler{Config: config.AlertEmailConfig{Enabled: true, Host: "smtp.example.com", Port: 587, From: "from@example.com", To: "to@example.com", Password: "secret"}}
	handler.Status(recorder, httptest.NewRequest("GET", "/api/v1/alerts/status", nil))
	if recorder.Code != 200 {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["email_enabled"] != true || response["smtp_configured"] != true || response["recipient_configured"] != true {
		t.Fatalf("response = %#v", response)
	}
	if _, ok := response["smtp_password"]; ok {
		t.Fatal("response exposed SMTP password")
	}
}

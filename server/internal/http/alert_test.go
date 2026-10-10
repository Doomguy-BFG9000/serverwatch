package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/config"
)

func TestAlertStatusIsSafe(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := AlertHandler{
		Config: config.AlertEmailConfig{Enabled: true, Host: "smtp.example.com", Port: 587, From: "from@example.com", To: "to@example.com, second@example.com", Password: "secret"},
		TelegramConfig: config.AlertTelegramConfig{
			Enabled: true, BotToken: "123:secret-token", ChatID: "-100123", APIBase: "https://api.telegram.org",
		},
	}
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
	// Non-secret delivery details are surfaced for the Alerts panel.
	if response["smtp_host"] != "smtp.example.com" || response["smtp_port"] != float64(587) {
		t.Fatalf("smtp details = %#v", response)
	}
	recipients, ok := response["email_recipients"].([]any)
	if !ok || len(recipients) != 2 || recipients[0] != "to@example.com" || recipients[1] != "second@example.com" {
		t.Fatalf("email_recipients = %#v", response["email_recipients"])
	}
	if response["telegram_chat_id"] != "-100123" {
		t.Fatalf("telegram_chat_id = %#v", response["telegram_chat_id"])
	}
	// Secrets must never be serialized.
	for _, key := range []string{"smtp_password", "password", "telegram_bot_token", "bot_token", "SMTP_PASSWORD", "TELEGRAM_BOT_TOKEN"} {
		if _, ok := response[key]; ok {
			t.Fatalf("response exposed %q", key)
		}
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("response body leaked a secret: %s", recorder.Body.String())
	}
}

func TestSplitRecipients(t *testing.T) {
	got := splitRecipients(" a@example.com , b@example.com ,, c@example.com ")
	want := []string{"a@example.com", "b@example.com", "c@example.com"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
	if empty := splitRecipients(""); len(empty) != 0 {
		t.Fatalf("empty input = %#v", empty)
	}
}

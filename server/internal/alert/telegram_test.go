package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/config"
)

func TestTelegramSendSuccess(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	sender := TelegramSender{
		Config: config.AlertTelegramConfig{BotToken: "secret-token", ChatID: "12345", APIBase: server.URL},
		Client: server.Client(),
	}
	err := sender.Send(context.Background(), Message{Subject: "DOWN: API", Body: "target unreachable"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/botsecret-token/sendMessage" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != "12345" {
		t.Fatalf("chat_id = %v", gotBody["chat_id"])
	}
	text, _ := gotBody["text"].(string)
	if !strings.Contains(text, "DOWN: API") || !strings.Contains(text, "target unreachable") {
		t.Fatalf("text missing content: %q", text)
	}
}

func TestTelegramSendRejectsIncompleteConfig(t *testing.T) {
	sender := TelegramSender{Config: config.AlertTelegramConfig{}}
	if err := sender.Send(context.Background(), Message{Subject: "x"}); err == nil {
		t.Fatal("expected error for incomplete config")
	}
}

func TestTelegramSendReportsAPIErrorWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer server.Close()

	sender := TelegramSender{
		Config: config.AlertTelegramConfig{BotToken: "super-secret-token", ChatID: "1", APIBase: server.URL},
		Client: server.Client(),
	}
	err := sender.Send(context.Background(), Message{Subject: "x"})
	if err == nil {
		t.Fatal("expected error for rejected message")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("error should describe the API failure: %v", err)
	}
	if strings.Contains(err.Error(), "super-secret-token") {
		t.Fatalf("error must not leak the bot token: %v", err)
	}
}

func TestTelegramSendTruncatesLongMessage(t *testing.T) {
	var gotLen int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		text, _ := body["text"].(string)
		gotLen = len(text)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sender := TelegramSender{
		Config: config.AlertTelegramConfig{BotToken: "t", ChatID: "1", APIBase: server.URL},
		Client: server.Client(),
	}
	if err := sender.Send(context.Background(), Message{Subject: "s", Body: strings.Repeat("x", 10000)}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLen > telegramMessageLimit {
		t.Fatalf("message length %d exceeds limit %d", gotLen, telegramMessageLimit)
	}
}

func TestTelegramTransportErrorHidesToken(t *testing.T) {
	// Point at a closed port to force a transport error containing the URL.
	sender := TelegramSender{
		Config: config.AlertTelegramConfig{BotToken: "leaky-token", ChatID: "1", APIBase: "http://127.0.0.1:1"},
	}
	err := sender.Send(context.Background(), Message{Subject: "x"})
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), "leaky-token") {
		t.Fatalf("transport error must not leak the bot token: %v", err)
	}
}

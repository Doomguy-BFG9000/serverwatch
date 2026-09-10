package alert

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

func TestEvaluateTransitions(t *testing.T) {
	state := &service.AlertState{}
	observation := service.AlertObservation{Status: "up"}
	if got := Evaluate(state, observation); len(got) != 0 {
		t.Fatalf("first up events = %#v", got)
	}
	observation.Status = "down"
	if got := Evaluate(state, observation); len(got) != 1 || got[0].Type != "down" {
		t.Fatalf("down events = %#v", got)
	}
	if got := Evaluate(state, observation); len(got) != 0 {
		t.Fatalf("repeated down events = %#v", got)
	}
	observation.Status = "up"
	if got := Evaluate(state, observation); len(got) != 1 || got[0].Type != "recovery" {
		t.Fatalf("recovery events = %#v", got)
	}
	if got := Evaluate(state, observation); len(got) != 0 {
		t.Fatalf("repeated up events = %#v", got)
	}
}

func TestEvaluateSSLWarningsAndRenewal(t *testing.T) {
	state := &service.AlertState{}
	expiry := time.Now().Add(30 * 24 * time.Hour)
	noWarning := service.AlertObservation{Status: "up", SSL: &service.SSLAlertMetadata{NotAfter: expiry, DaysRemaining: 31}}
	if got := Evaluate(state, noWarning); len(got) != 0 {
		t.Fatalf("31 day events = %#v", got)
	}
	observation := service.AlertObservation{Status: "up", SSL: &service.SSLAlertMetadata{NotAfter: expiry, DaysRemaining: 30}}
	if got := Evaluate(state, observation); len(got) != 1 || got[0].Type != "ssl_30" {
		t.Fatalf("30 day events = %#v", got)
	}
	if got := Evaluate(state, observation); len(got) != 0 {
		t.Fatalf("duplicate 30 day events = %#v", got)
	}
	observation.SSL.DaysRemaining = 14
	if got := Evaluate(state, observation); len(got) != 1 || got[0].Type != "ssl_14" {
		t.Fatalf("14 day events = %#v", got)
	}
	observation.SSL.DaysRemaining = 7
	if got := Evaluate(state, observation); len(got) != 1 || got[0].Type != "ssl_7" {
		t.Fatalf("7 day events = %#v", got)
	}
	observation.SSL.NotAfter = expiry.Add(90 * 24 * time.Hour)
	observation.SSL.DaysRemaining = 30
	if got := Evaluate(state, observation); len(got) != 1 || got[0].Type != "ssl_30" {
		t.Fatalf("renewal events = %#v", got)
	}
}

type fakeStore struct {
	events []service.AlertEvent
	err    error
}

func (f fakeStore) EvaluateAlerts(context.Context, service.AlertObservation) ([]service.AlertEvent, error) {
	return f.events, f.err
}

type fakeSender struct {
	count int
	last  Message
	err   error
}

func (f *fakeSender) Send(_ context.Context, m Message) error { f.count++; f.last = m; return f.err }

func TestServiceDisabledAndSenderFailureAreSafe(t *testing.T) {
	sender := &fakeSender{}
	s := &Service{Store: fakeStore{events: []service.AlertEvent{{Type: "down"}}}, Sender: sender, Enabled: false}
	s.Process(context.Background(), service.AlertObservation{Monitor: service.Monitor{Name: "test"}})
	if sender.count != 0 {
		t.Fatal("disabled service attempted delivery")
	}
	s.Enabled = true
	sender.err = errors.New("offline")
	s.Process(context.Background(), service.AlertObservation{Monitor: service.Monitor{Name: "test"}})
	if sender.count != 1 {
		t.Fatal("sender failure was not handled")
	}
}

func TestMessageContent(t *testing.T) {
	message := buildMessage(service.AlertObservation{Monitor: service.Monitor{Name: "API", MonitorType: "https", Target: "https://example.com"}, Status: "down", Message: "timeout"}, "down")
	for _, value := range []string{"[ServerWatch] DOWN: API", "https://example.com", "timeout"} {
		if !contains(message.Subject+" "+message.Body, value) {
			t.Fatalf("message missing %q", value)
		}
	}
}

func contains(value, part string) bool {
	return len(value) >= len(part) && strings.Contains(value, part)
}

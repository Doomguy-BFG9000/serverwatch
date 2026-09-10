package alert

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

type StateStore interface {
	EvaluateAlerts(context.Context, service.AlertObservation) ([]service.AlertEvent, error)
}

type Service struct {
	Store   StateStore
	Sender  Sender
	Enabled bool
}

// Evaluate applies the state-change rules without I/O. It is also useful to
// keep the notification policy independently testable from PostgreSQL.
func Evaluate(state *service.AlertState, observation service.AlertObservation) []service.AlertEvent {
	events := make([]service.AlertEvent, 0, 4)
	if observation.Status == "down" && (state.LastStatus == "" || state.LastStatus == "up") {
		events = append(events, service.AlertEvent{Type: "down"})
	}
	if observation.Status == "up" && state.LastStatus == "down" {
		events = append(events, service.AlertEvent{Type: "recovery"})
	}
	if observation.SSL != nil && observation.Status == "up" {
		ssl := observation.SSL
		if state.SSLExpiryNotAfter == nil || !state.SSLExpiryNotAfter.Equal(ssl.NotAfter) {
			notAfter := ssl.NotAfter
			state.SSLExpiryNotAfter = &notAfter
			state.SSLWarned30, state.SSLWarned14, state.SSLWarned7 = false, false, false
		}
		if ssl.DaysRemaining <= 30 && !state.SSLWarned30 {
			events = append(events, service.AlertEvent{Type: "ssl_30"})
			state.SSLWarned30 = true
		}
		if ssl.DaysRemaining <= 14 && !state.SSLWarned14 {
			events = append(events, service.AlertEvent{Type: "ssl_14"})
			state.SSLWarned14 = true
		}
		if ssl.DaysRemaining <= 7 && !state.SSLWarned7 {
			events = append(events, service.AlertEvent{Type: "ssl_7"})
			state.SSLWarned7 = true
		}
	}
	state.LastStatus = observation.Status
	return events
}

func (s *Service) Process(ctx context.Context, observation service.AlertObservation) {
	if s == nil || s.Store == nil {
		return
	}
	events, err := s.Store.EvaluateAlerts(ctx, observation)
	if err != nil {
		log.Printf("alert state update failed: monitor_id=%d error=%v", observation.Monitor.ID, err)
		return
	}
	if !s.Enabled || s.Sender == nil {
		return
	}
	for _, event := range events {
		message := buildMessage(observation, event.Type)
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := s.Sender.Send(sendCtx, message)
		cancel()
		if err != nil {
			log.Printf("alert delivery failed: monitor_id=%d type=%s error=%v", observation.Monitor.ID, event.Type, err)
			continue
		}
		log.Printf("alert sent: monitor_id=%d type=%s", observation.Monitor.ID, event.Type)
	}
}

func buildMessage(o service.AlertObservation, eventType string) Message {
	name, target := o.Monitor.Name, o.Monitor.Target
	switch eventType {
	case "down":
		return Message{Subject: fmt.Sprintf("[ServerWatch] DOWN: %s", name), Body: fmt.Sprintf("ServerWatch Monitor Alert\n\nStatus: DOWN\nMonitor: %s\nType: %s\nTarget: %s\nChecked At: %s\nResponse Time: %d ms\nHTTP Status: %s\nReason: %s\n", name, o.Monitor.MonitorType, target, o.CheckedAt.Format(time.RFC3339), o.ResponseTimeMS, statusCode(o.StatusCode), o.Message), To: ""}
	case "recovery":
		return Message{Subject: fmt.Sprintf("[ServerWatch] RECOVERED: %s", name), Body: fmt.Sprintf("ServerWatch Monitor Recovery\n\nStatus: UP\nMonitor: %s\nType: %s\nTarget: %s\nRecovered At: %s\nResponse Time: %d ms\n", name, o.Monitor.MonitorType, target, o.CheckedAt.Format(time.RFC3339), o.ResponseTimeMS)}
	default:
		days := strings.TrimPrefix(eventType, "ssl_")
		metadata := o.SSL
		return Message{Subject: fmt.Sprintf("[ServerWatch] SSL expires within %s days: %s", days, name), Body: fmt.Sprintf("ServerWatch SSL Certificate Warning\n\nMonitor: %s\nTarget: %s\nCertificate Subject: %s\nIssuer: %s\nExpiry Date: %s\nDays Remaining: %d\n", name, target, metadata.Subject, metadata.Issuer, metadata.NotAfter.Format(time.RFC3339), metadata.DaysRemaining)}
	}
}

func statusCode(code *int) string {
	if code == nil {
		return "—"
	}
	return fmt.Sprint(*code)
}

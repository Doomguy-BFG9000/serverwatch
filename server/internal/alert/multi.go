package alert

import (
	"context"
	"errors"
	"log"
)

// MultiSender delivers each alert to every sender. It lets a deployment enable
// email and Telegram (or several channels) at once. Every sender is attempted
// even if an earlier one fails, so one broken channel cannot silence the rest;
// the failures are joined and returned.
type MultiSender struct {
	Senders []Sender
}

func (m MultiSender) Send(ctx context.Context, message Message) error {
	var failures []error
	for _, sender := range m.Senders {
		if sender == nil {
			continue
		}
		if err := sender.Send(ctx, message); err != nil {
			log.Printf("alert channel failed: %v", err)
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

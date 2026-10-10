package alert

import (
	"context"
	"errors"
	"testing"
)

type recordingSender struct {
	calls int
	err   error
}

func (r *recordingSender) Send(context.Context, Message) error { r.calls++; return r.err }

// TestMultiSenderDeliversToAllChannels verifies one enabled channel failing
// does not prevent the others from receiving the alert.
func TestMultiSenderDeliversToAllChannels(t *testing.T) {
	failing := &recordingSender{err: errors.New("down")}
	ok := &recordingSender{}
	multi := MultiSender{Senders: []Sender{failing, ok, nil}}

	err := multi.Send(context.Background(), Message{Subject: "x"})
	if err == nil {
		t.Fatal("expected joined error from failing channel")
	}
	if failing.calls != 1 || ok.calls != 1 {
		t.Fatalf("calls: failing=%d ok=%d", failing.calls, ok.calls)
	}
}

func TestMultiSenderAllHealthy(t *testing.T) {
	a, b := &recordingSender{}, &recordingSender{}
	multi := MultiSender{Senders: []Sender{a, b}}
	if err := multi.Send(context.Background(), Message{Subject: "x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.calls != 1 || b.calls != 1 {
		t.Fatalf("calls: a=%d b=%d", a.calls, b.calls)
	}
}

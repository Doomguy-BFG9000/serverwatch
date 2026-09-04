package monitor

import (
	"context"
	"net"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

func TestTCPCheckerSuccessfulConnection(t *testing.T) {
	listener := listenTCP(t, "127.0.0.1:0")
	defer listener.Close()

	result := NewTCPChecker().Check(context.Background(), service.Monitor{
		ID:             1,
		Target:         listener.Addr().String(),
		TimeoutSeconds: 1,
	})

	if result.Status != "up" {
		t.Fatalf("status = %q, want up", result.Status)
	}
	if result.StatusCode != nil {
		t.Fatal("status code should be nil for TCP checks")
	}
	if result.ResponseTimeMS < 0 {
		t.Fatalf("response time = %d, want non-negative", result.ResponseTimeMS)
	}
}

func TestTCPCheckerFailedConnection(t *testing.T) {
	listener := listenTCP(t, "127.0.0.1:0")
	address := listener.Addr().String()
	listener.Close()

	result := NewTCPChecker().Check(context.Background(), service.Monitor{
		ID:             2,
		Target:         address,
		TimeoutSeconds: 1,
	})

	if result.Status != "down" {
		t.Fatalf("status = %q, want down", result.Status)
	}
	if result.StatusCode != nil {
		t.Fatal("status code should be nil for TCP checks")
	}
	if result.Message == "" {
		t.Fatal("failure message should not be empty")
	}
}

func TestTCPCheckerInvalidTarget(t *testing.T) {
	result := NewTCPChecker().Check(context.Background(), service.Monitor{
		ID:             3,
		Target:         "not-a-host-port",
		TimeoutSeconds: 1,
	})

	if result.Status != "down" {
		t.Fatalf("status = %q, want down", result.Status)
	}
	if result.Message == "" {
		t.Fatal("invalid target message should not be empty")
	}
}

func TestParseTCPAddressSupportsIPv6(t *testing.T) {
	address, err := parseTCPAddress("[2001:db8::1]:443")
	if err != nil {
		t.Fatalf("parse IPv6 target: %v", err)
	}
	if address != "[2001:db8::1]:443" {
		t.Fatalf("address = %q, want [2001:db8::1]:443", address)
	}
}

func listenTCP(t *testing.T, address string) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("listen on %s: %v", address, err)
	}

	return listener
}

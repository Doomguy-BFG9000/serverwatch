package monitor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

func TestPingCheckerSuccessfulTargets(t *testing.T) {
	for _, test := range []struct {
		name, target, output string
		want                 int64
	}{
		{"hostname", "localhost", "PING localhost (127.0.0.1): 56 data bytes\n64 bytes from 127.0.0.1: time=12.3 ms\n", 12},
		{"IPv4", "127.0.0.1", "64 bytes from 127.0.0.1: time=12.3 ms\n", 12},
	} {
		t.Run(test.name, func(t *testing.T) {
			var gotArgs []string
			checker := &PingChecker{
				LookPath: func(string) (string, error) { return "/usr/bin/ping", nil },
				Run: func(_ context.Context, executable string, args ...string) (string, string, error) {
					gotArgs = append([]string{executable}, args...)
					return test.output, "", nil
				},
			}
			result := checker.Check(context.Background(), service.Monitor{ID: 1, Target: test.target, TimeoutSeconds: 1})
			if result.Status != "up" || result.ResponseTimeMS != test.want {
				t.Fatalf("result = %+v", result)
			}
			if gotArgs[len(gotArgs)-1] != test.target {
				t.Fatalf("target was not passed as final standalone argument: %q", gotArgs)
			}
		})
	}
}

func TestParsePingTarget(t *testing.T) {
	if got, err := ParsePingTarget("  example.com  "); err != nil || got != "example.com" {
		t.Fatalf("whitespace target = %q, %v", got, err)
	}
	if _, err := ParsePingTarget("::1"); err != nil {
		t.Fatalf("IPv6 rejected: %v", err)
	}
	for _, target := range []string{"", "https://example.com", "example.com:443", "1.1.1.1:80", "example.com/test", ";touch /tmp/pwned", "$(touch /tmp/pwned)", "`touch /tmp/pwned`", "-n", "--help", "example.com;id", "example.com\nwhoami"} {
		if _, err := ParsePingTarget(target); err == nil {
			t.Errorf("ParsePingTarget(%q) succeeded", target)
		}
	}
}

func TestParsePingLatency(t *testing.T) {
	for _, test := range []struct {
		output string
		want   int64
	}{
		{"64 bytes from host: time=12.3 ms", 12},
		{"64 bytes from host: time=0.041 ms", 0},
		{"64 bytes from host: time<1 ms", 1},
	} {
		got, ok := ParsePingLatency(test.output)
		if !ok || got != test.want {
			t.Errorf("ParsePingLatency(%q) = %d, %v; want %d, true", test.output, got, ok, test.want)
		}
	}
}

func TestPingCheckerFailures(t *testing.T) {
	tests := []struct {
		name, stdout, stderr, message string
		err                           error
	}{
		{name: "timeout", stderr: "Request timeout", err: errors.New("exit status 1"), message: "ping timeout"},
		{name: "unreachable", stdout: "Destination Host Unreachable", err: errors.New("exit status 1"), message: "host unreachable"},
		{name: "DNS", stdout: "ping: unknown host", err: errors.New("exit status 2"), message: "name or service not known"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker := &PingChecker{
				LookPath: func(string) (string, error) { return "ping", nil },
				Run: func(context.Context, string, ...string) (string, string, error) {
					return test.stdout, test.stderr, test.err
				},
			}
			result := checker.Check(context.Background(), service.Monitor{ID: 1, Target: "example.com", TimeoutSeconds: 1})
			if result.Status != "down" || result.Message != test.message {
				t.Fatalf("result = %+v", result)
			}
		})
	}

	missing := (&PingChecker{LookPath: func(string) (string, error) { return "", errors.New("missing") }}).Check(context.Background(), service.Monitor{Target: "example.com"})
	if missing.Status != "down" || missing.Message != "ping executable not found" {
		t.Fatalf("missing executable result = %+v", missing)
	}
}

func TestPingCheckerContextCancellationAndNoShell(t *testing.T) {
	started := make(chan struct{})
	checker := &PingChecker{
		LookPath: func(string) (string, error) { return "ping", nil },
		Run: func(ctx context.Context, _ string, args ...string) (string, string, error) {
			if strings.Contains(strings.Join(args, " "), "sh -c") {
				t.Fatal("shell invocation detected")
			}
			close(started)
			<-ctx.Done()
			return "", "", ctx.Err()
		},
	}
	result := checker.Check(context.Background(), service.Monitor{ID: 1, Target: "example.com", TimeoutSeconds: 1})
	select {
	case <-started:
	default:
		t.Fatal("runner was not called")
	}
	if result.Status != "down" || result.Message != "ping timeout" {
		t.Fatalf("result = %+v", result)
	}
}

package monitor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

var pingTimePattern = regexp.MustCompile(`(?:time=|time<)\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)

// PingCommandRunner is deliberately small so ping checks can be tested without
// requiring ICMP access or a public network.
type PingCommandRunner func(context.Context, string, ...string) (string, string, error)

type PingChecker struct {
	LookPath func(string) (string, error)
	Run      PingCommandRunner
}

func NewPingChecker() *PingChecker {
	return &PingChecker{LookPath: exec.LookPath, Run: runPingCommand}
}

func (c *PingChecker) Check(ctx context.Context, monitor service.Monitor) service.CreateMonitorResultInput {
	started := time.Now()
	target, err := ParsePingTarget(monitor.Target)
	if err != nil {
		return pingFailure(monitor.ID, started, err.Error())
	}

	lookup := c.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	executable, err := lookup("ping")
	if err != nil {
		return pingFailure(monitor.ID, started, "ping executable not found")
	}

	timeout := time.Duration(monitor.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	runner := c.Run
	if runner == nil {
		runner = runPingCommand
	}
	stdout, stderr, err := runner(checkCtx, executable, "-n", "-c", "1", "-W", strconv.FormatFloat(timeout.Seconds(), 'f', -1, 64), target)
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		if errors.Is(checkCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return pingFailureWithElapsed(monitor.ID, elapsed, "ping timeout")
		}
		message := classifyPingFailure(stdout, stderr)
		return pingFailureWithElapsed(monitor.ID, elapsed, message)
	}

	latency, ok := ParsePingLatency(stdout)
	if !ok {
		return pingFailureWithElapsed(monitor.ID, elapsed, "ping reply latency unavailable")
	}
	return service.CreateMonitorResultInput{
		MonitorID: monitor.ID, Status: "up", ResponseTimeMS: latency,
		Message: fmt.Sprintf("ping reply from %s: %s ms", target, latencyText(stdout)),
	}
}

func ParsePingTarget(value string) (string, error) {
	target := strings.TrimSpace(value)
	if target == "" {
		return "", fmt.Errorf("invalid ping target: host or IP is required")
	}
	if strings.HasPrefix(target, "-") || strings.ContainsAny(target, "/?#") {
		return "", fmt.Errorf("invalid ping target: host or IP only")
	}
	for _, r := range target {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", fmt.Errorf("invalid ping target: host or IP only")
		}
	}
	if strings.Contains(target, ":") {
		if netIP := parsePingIP(target); netIP {
			return target, nil
		}
		return "", fmt.Errorf("invalid ping target: ports are not supported")
	}
	if parsePingIP(target) || validPingHostname(target) {
		return target, nil
	}
	return "", fmt.Errorf("invalid ping target: host or IP only")
}

func parsePingIP(value string) bool {
	return net.ParseIP(value) != nil
}

func validPingHostname(value string) bool {
	if len(value) > 253 || strings.HasSuffix(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return true
}

func ParsePingLatency(output string) (int64, bool) {
	match := pingTimePattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return int64(math.Round(value)), true
}

func latencyText(output string) string {
	match := pingTimePattern.FindStringSubmatch(output)
	if len(match) == 2 {
		return match[1]
	}
	return "0"
}

func classifyPingFailure(stdout, stderr string) string {
	text := strings.ToLower(stdout + "\n" + stderr)
	switch {
	case strings.Contains(text, "name or service not known"), strings.Contains(text, "unknown host"), strings.Contains(text, "temporary failure in name resolution"):
		return "name or service not known"
	case strings.Contains(text, "unreachable"):
		return "host unreachable"
	case strings.Contains(text, "timed out"), strings.Contains(text, "timeout"):
		return "ping timeout"
	default:
		return "ping command failed"
	}
}

func pingFailure(id int64, started time.Time, message string) service.CreateMonitorResultInput {
	return pingFailureWithElapsed(id, time.Since(started).Milliseconds(), message)
}

func pingFailureWithElapsed(id, elapsed int64, message string) service.CreateMonitorResultInput {
	return service.CreateMonitorResultInput{MonitorID: id, Status: "down", ResponseTimeMS: elapsed, Message: message}
}

func runPingCommand(ctx context.Context, executable string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	return string(stdout), stderr.String(), err
}

type limitedBuffer struct{ data []byte }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	const max = 16 * 1024
	written := len(p)
	if len(b.data) < max {
		remaining := max - len(b.data)
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	return written, nil
}

func (b *limitedBuffer) String() string { return string(b.data) }

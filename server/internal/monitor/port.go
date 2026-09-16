package monitor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

const (
	defaultPortTimeout = 10 * time.Second

	// MaxPortMonitorPorts bounds how many TCP ports one port monitor may watch.
	// Multi-port monitoring is monitoring, not scanning: only explicitly
	// configured ports are ever dialed.
	MaxPortMonitorPorts = 20

	// maxConcurrentPortChecks bounds simultaneous TCP connection attempts so a
	// fully configured monitor never opens 20 sockets at once.
	maxConcurrentPortChecks = 5
)

// PortChecker checks several explicitly configured TCP ports on a single host.
type PortChecker struct {
	Dialer tcpDialer
}

func NewPortChecker() *PortChecker {
	return &PortChecker{Dialer: &net.Dialer{}}
}

// PortCheckResult is the per-port outcome of one multi-port check.
type PortCheckResult struct {
	Port           int    `json:"port"`
	Status         string `json:"status"`
	ResponseTimeMS int64  `json:"response_time_ms"`
	Message        string `json:"message"`
}

// MultiPortCheckResult is the structured outcome of one multi-port check.
//
// Result.ResponseTimeMS is the overall wall-clock duration of the complete
// check, never the sum of the individual port latencies.
type MultiPortCheckResult struct {
	Result    service.CreateMonitorResultInput
	Target    string
	Ports     []PortCheckResult
	Reachable int
	Total     int
}

// Check dials every configured port of the monitor target with bounded
// concurrency and reports UP only when all of them accept a connection.
func (c *PortChecker) Check(ctx context.Context, monitor service.Monitor) MultiPortCheckResult {
	started := time.Now()

	target, err := ParsePortTarget(monitor.Target)
	if err != nil {
		return portFailure(monitor.ID, started, err.Error())
	}

	var configured []int
	if monitor.Config != nil {
		configured = monitor.Config.Ports
	}
	ports, err := NormalizePorts(configured)
	if err != nil {
		return portFailure(monitor.ID, started, err.Error())
	}

	timeout := time.Duration(monitor.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultPortTimeout
	}

	// One deadline covers the whole multi-port check, so the monitor timeout is
	// the overall execution budget instead of a per-port budget and it
	// cancels every outstanding connection attempt at once.
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results := make([]PortCheckResult, len(ports))
	semaphore := make(chan struct{}, maxConcurrentPortChecks)

	var wg sync.WaitGroup

	for index, port := range ports {
		wg.Add(1)

		go func(index, port int) {
			defer wg.Done()

			if err := checkCtx.Err(); err != nil {
				results[index] = interruptedPortResult(port, checkCtx)
				return
			}

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()

			case <-checkCtx.Done():
				results[index] = interruptedPortResult(port, checkCtx)
				return
			}

			results[index] = c.checkPort(checkCtx, target, port)
		}(index, port)
	}

	wg.Wait()

	reachable := 0

	unreachable := make([]int, 0, len(results))

	for _, result := range results {
		if result.Status == "up" {
			reachable++
			continue
		}

		unreachable = append(unreachable, result.Port)
	}

	status := "down"
	if reachable == len(results) {
		status = "up"
	}

	return MultiPortCheckResult{
		Result: service.CreateMonitorResultInput{
			MonitorID:      monitor.ID,
			Status:         status,
			ResponseTimeMS: time.Since(started).Milliseconds(),
			Message:        summarizePortResults(reachable, len(results), unreachable),
		},
		Target:    target,
		Ports:     results,
		Reachable: reachable,
		Total:     len(results),
	}
}

func (c *PortChecker) checkPort(ctx context.Context, host string, port int) PortCheckResult {
	started := time.Now()

	address := net.JoinHostPort(host, strconv.Itoa(port))

	conn, err := dialTCP(ctx, c.dialer(), address)
	elapsed := time.Since(started).Milliseconds()

	if err != nil {
		return PortCheckResult{
			Port:           port,
			Status:         "down",
			ResponseTimeMS: elapsed,
			Message:        classifyPortError(ctx, err),
		}
	}

	_ = conn.Close()

	return PortCheckResult{
		Port:           port,
		Status:         "up",
		ResponseTimeMS: elapsed,
		Message:        "reachable",
	}
}

func (c *PortChecker) dialer() tcpDialer {
	if c.Dialer == nil {
		return &net.Dialer{}
	}

	return c.Dialer
}

// ParsePortTarget validates a port monitor target: exactly one host, with no
// protocol, path, query, fragment, or embedded port. Raw IPv6 addresses are
// accepted with or without brackets.
func ParsePortTarget(value string) (string, error) {
	if value == "" {
		return "", errors.New("invalid port target: host or IP is required")
	}

	if strings.TrimSpace(value) != value {
		return "", errors.New("invalid port target: host or IP only")
	}

	target := value

	// Accept a bracket-wrapped IPv6 address for convenience: ports are
	// configured separately, so the brackets are not needed to disambiguate a
	// port. Dialing re-adds them through net.JoinHostPort.
	if strings.HasPrefix(target, "[") && strings.HasSuffix(target, "]") {
		if inner := strings.TrimSuffix(strings.TrimPrefix(target, "["), "]"); net.ParseIP(inner) != nil {
			target = inner
		}
	}

	if strings.HasPrefix(target, "-") || strings.ContainsAny(target, "/?#") || strings.Contains(target, "://") {
		return "", errors.New("invalid port target: host or IP only, without protocol, path, or port")
	}

	for _, character := range target {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return "", errors.New("invalid port target: host or IP only")
		}
	}

	if strings.Contains(target, ":") {
		// Colons are only valid as part of a raw IPv6 address. An embedded
		// port is configured through the ports field instead.
		if net.ParseIP(target) != nil {
			return target, nil
		}

		return "", errors.New("invalid port target: ports are configured separately")
	}

	if net.ParseIP(target) != nil {
		return target, nil
	}

	if isNumericAddress(target) {
		return "", errors.New("invalid port target: invalid IP address")
	}

	if validPortHostname(target) {
		return target, nil
	}

	return "", errors.New("invalid port target: host or IP only")
}

// NormalizePorts validates explicitly configured ports and returns them sorted
// ascending with duplicates removed. Port ranges are never accepted: every
// monitored port must be configured individually.
func NormalizePorts(ports []int) ([]int, error) {
	if len(ports) == 0 {
		return nil, errors.New("at least one port is required")
	}

	seen := make(map[int]struct{}, len(ports))
	normalized := make([]int, 0, len(ports))

	for _, port := range ports {
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port %d is invalid: ports must be between 1 and 65535", port)
		}

		if _, duplicate := seen[port]; duplicate {
			continue
		}

		seen[port] = struct{}{}
		normalized = append(normalized, port)
	}

	if len(normalized) > MaxPortMonitorPorts {
		return nil, fmt.Errorf("too many ports: a port monitor supports at most %d ports", MaxPortMonitorPorts)
	}

	sort.Ints(normalized)

	return normalized, nil
}

// summarizePortResults keeps stored result messages short and user-facing.
func summarizePortResults(reachable, total int, unreachable []int) string {
	if len(unreachable) == 0 {
		return fmt.Sprintf("%d/%d ports reachable", reachable, total)
	}

	if len(unreachable) == 1 {
		return fmt.Sprintf("%d/%d ports reachable; port %d unreachable", reachable, total, unreachable[0])
	}

	values := make([]string, 0, len(unreachable))
	for _, port := range unreachable {
		values = append(values, strconv.Itoa(port))
	}

	return fmt.Sprintf("%d/%d ports reachable; ports %s unreachable", reachable, total, strings.Join(values, ", "))
}

// classifyPortError maps Go networking errors to short user-facing states so
// stored results never expose low-level networking internals.
func classifyPortError(ctx context.Context, err error) string {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled):
		return "check canceled"
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "DNS resolution failed"
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		return "host unreachable"
	}

	return "unreachable"
}

func interruptedPortResult(port int, ctx context.Context) PortCheckResult {
	return PortCheckResult{Port: port, Status: "down", Message: classifyPortError(ctx, ctx.Err())}
}

func portFailure(id int64, started time.Time, message string) MultiPortCheckResult {
	return MultiPortCheckResult{
		Result: service.CreateMonitorResultInput{
			MonitorID:      id,
			Status:         "down",
			ResponseTimeMS: time.Since(started).Milliseconds(),
			Message:        message,
		},
		Ports: []PortCheckResult{},
	}
}

// validPortHostname mirrors the hostname rules used by the Ping and DNS
// monitors: dot-separated labels of letters, digits, and hyphens.
func validPortHostname(value string) bool {
	if len(value) > 253 || strings.HasSuffix(value, ".") {
		return false
	}

	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, character := range label {
			if !(character == '-' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
				return false
			}
		}
	}

	return true
}

// isNumericAddress reports whether a target looks like a dotted IPv4 address so
// invalid literals such as 999.999.999.999 are rejected instead of being
// treated as a hostname.
func isNumericAddress(value string) bool {
	if !strings.Contains(value, ".") {
		return false
	}

	for _, character := range value {
		if character != '.' && (character < '0' || character > '9') {
			return false
		}
	}

	return true
}

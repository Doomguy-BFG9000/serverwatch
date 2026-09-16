package monitor

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

const defaultTCPTimeout = 10 * time.Second

// tcpDialer is the minimal dialing surface shared by the TCP and multi-port
// checkers. It is satisfied by *net.Dialer, so both monitors keep using the
// same standard-library connection logic while staying testable.
type tcpDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// dialTCP opens one TCP connection and returns it to the caller. The caller
// owns the returned connection and must close it.
func dialTCP(ctx context.Context, dialer tcpDialer, address string) (net.Conn, error) {
	if dialer == nil {
		dialer = &net.Dialer{}
	}

	return dialer.DialContext(ctx, "tcp", address)
}

type TCPChecker struct {
	Dialer net.Dialer
}

func NewTCPChecker() *TCPChecker {
	return &TCPChecker{}
}

func (c *TCPChecker) Check(
	ctx context.Context,
	monitor service.Monitor,
) service.CreateMonitorResultInput {
	started := time.Now()

	target, err := parseTCPAddress(monitor.Target)
	if err != nil {
		return service.CreateMonitorResultInput{
			MonitorID:      monitor.ID,
			Status:         "down",
			ResponseTimeMS: time.Since(started).Milliseconds(),
			Message:        err.Error(),
		}
	}

	timeout := time.Duration(monitor.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultTCPTimeout
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := dialTCP(checkCtx, &c.Dialer, target)
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		return service.CreateMonitorResultInput{
			MonitorID:      monitor.ID,
			Status:         "down",
			ResponseTimeMS: elapsed,
			Message:        err.Error(),
		}
	}

	_ = conn.Close()

	return service.CreateMonitorResultInput{
		MonitorID:      monitor.ID,
		Status:         "up",
		ResponseTimeMS: elapsed,
		Message:        "TCP connection successful",
	}
}

func parseTCPAddress(target string) (string, error) {
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return "", fmt.Errorf("invalid TCP target: expected host:port")
	}

	if host == "" || portText == "" {
		return "", fmt.Errorf("invalid TCP target: host and port are required")
	}

	for _, character := range portText {
		if character < '0' || character > '9' {
			return "", fmt.Errorf("invalid TCP target: port must be numeric")
		}
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid TCP target: port must be between 1 and 65535")
	}

	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

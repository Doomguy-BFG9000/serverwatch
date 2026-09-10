package monitor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

const defaultSSLPort = "443"

type SSLCheckResult struct {
	Result           service.CreateMonitorResultInput
	Subject          string    `json:"subject"`
	Issuer           string    `json:"issuer"`
	NotBefore        time.Time `json:"not_before"`
	NotAfter         time.Time `json:"not_after"`
	DaysRemaining    int       `json:"days_remaining"`
	CertificateValid bool      `json:"certificate_valid"`
}

type SSLChecker struct {
	Dialer  net.Dialer
	RootCAs *x509.CertPool
	Now     func() time.Time
}

func NewSSLChecker() *SSLChecker {
	return &SSLChecker{}
}

func (c *SSLChecker) Check(ctx context.Context, monitor service.Monitor) service.CreateMonitorResultInput {
	return c.CheckCertificate(ctx, monitor).Result
}

func (c *SSLChecker) CheckCertificate(ctx context.Context, monitor service.Monitor) SSLCheckResult {
	started := time.Now()
	address, host, err := parseSSLAddress(monitor.Target)
	if err != nil {
		return SSLCheckResult{Result: sslFailure(monitor.ID, started, err)}
	}

	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}

	timeout := time.Duration(monitor.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	config := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
		RootCAs:    c.RootCAs,
	}
	dialer := c.Dialer
	tlsDialer := tls.Dialer{NetDialer: &dialer, Config: config}
	conn, err := tlsDialer.DialContext(checkCtx, "tcp", address)
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		return SSLCheckResult{Result: service.CreateMonitorResultInput{
			MonitorID: monitor.ID, Status: "down", ResponseTimeMS: elapsed,
			Message: fmt.Sprintf("TLS certificate verification failed: %v", err),
		}}
	}
	defer conn.Close()

	state := conn.(*tls.Conn).ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return SSLCheckResult{Result: service.CreateMonitorResultInput{
			MonitorID: monitor.ID, Status: "down", ResponseTimeMS: elapsed,
			Message: "TLS connection succeeded but no peer certificate was presented",
		}}
	}

	certificate := state.PeerCertificates[0]
	daysRemaining := int((certificate.NotAfter.Sub(now)).Hours() / 24)
	message := fmt.Sprintf("Certificate valid; expires %s; %d days remaining", certificate.NotAfter.Format("2006-01-02"), daysRemaining)
	if daysRemaining <= 30 {
		message = fmt.Sprintf("Certificate expires in %d days", daysRemaining)
	}

	return SSLCheckResult{
		Result: service.CreateMonitorResultInput{
			MonitorID: monitor.ID, Status: "up", ResponseTimeMS: elapsed, Message: message,
		},
		Subject: certificate.Subject.String(), Issuer: certificate.Issuer.String(),
		NotBefore: certificate.NotBefore, NotAfter: certificate.NotAfter,
		DaysRemaining: daysRemaining, CertificateValid: true,
	}
}

func sslFailure(id int64, started time.Time, err error) service.CreateMonitorResultInput {
	return service.CreateMonitorResultInput{MonitorID: id, Status: "down", ResponseTimeMS: time.Since(started).Milliseconds(), Message: err.Error()}
}

func ParseSSLAddress(target string) (string, error) {
	address, _, err := parseSSLAddress(target)
	return address, err
}

func parseSSLAddress(target string) (string, string, error) {
	target = strings.TrimSpace(target)
	if target == "" || strings.ContainsAny(target, "/?#") {
		return "", "", fmt.Errorf("invalid SSL target")
	}

	host := target
	port := defaultSSLPort
	if strings.HasPrefix(target, "[") {
		if strings.HasSuffix(target, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(target, "["), "]")
		} else {
			var err error
			host, port, err = net.SplitHostPort(target)
			if err != nil {
				return "", "", fmt.Errorf("invalid SSL target: expected host[:port]")
			}
		}
	} else if strings.Contains(target, ":") {
		var err error
		host, port, err = net.SplitHostPort(target)
		if err != nil {
			return "", "", fmt.Errorf("invalid SSL target: expected host[:port]")
		}
	}

	if host == "" || port == "" {
		return "", "", fmt.Errorf("invalid SSL target: host and port are required")
	}
	if net.ParseIP(host) == nil && strings.ContainsAny(host, " \t\r\n") {
		return "", "", fmt.Errorf("invalid SSL target: invalid hostname")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", "", fmt.Errorf("invalid SSL target: port must be between 1 and 65535")
	}
	return net.JoinHostPort(host, strconv.Itoa(portNumber)), host, nil
}

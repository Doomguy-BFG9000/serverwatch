package monitor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

const (
	DNSRecordA     = "A"
	DNSRecordAAAA  = "AAAA"
	DNSRecordCNAME = "CNAME"
	DNSRecordMX    = "MX"
	DNSRecordNS    = "NS"
	DNSRecordTXT   = "TXT"
)

var supportedDNSRecordTypes = map[string]struct{}{
	DNSRecordA: {}, DNSRecordAAAA: {}, DNSRecordCNAME: {},
	DNSRecordMX: {}, DNSRecordNS: {}, DNSRecordTXT: {},
}

// DNSResolver is intentionally small so DNS checks can be tested without a
// network or a dependency on public DNS.
type DNSResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
	LookupCNAME(context.Context, string) (string, error)
	LookupMX(context.Context, string) ([]*net.MX, error)
	LookupNS(context.Context, string) ([]*net.NS, error)
	LookupTXT(context.Context, string) ([]string, error)
}

type DNSChecker struct{ Resolver DNSResolver }

type DNSCheckResult struct {
	Result     service.CreateMonitorResultInput
	RecordType string
	Records    []string
	Expected   []string
	Matched    bool
}

type systemDNSResolver struct{ resolver *net.Resolver }

func (r systemDNSResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return r.resolver.LookupIPAddr(ctx, host)
}
func (r systemDNSResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	return r.resolver.LookupCNAME(ctx, host)
}
func (r systemDNSResolver) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	return r.resolver.LookupMX(ctx, host)
}
func (r systemDNSResolver) LookupNS(ctx context.Context, host string) ([]*net.NS, error) {
	return r.resolver.LookupNS(ctx, host)
}
func (r systemDNSResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, host)
}

func NewDNSChecker() *DNSChecker {
	return &DNSChecker{Resolver: systemDNSResolver{resolver: net.DefaultResolver}}
}

func (c *DNSChecker) Check(ctx context.Context, monitor service.Monitor) DNSCheckResult {
	started := time.Now()
	target, err := ParseDNSTarget(monitor.Target)
	if err != nil {
		return dnsFailure(monitor.ID, started, err.Error())
	}

	config := monitor.Config
	if config == nil {
		config = &service.MonitorConfig{}
	}
	recordType := strings.ToUpper(strings.TrimSpace(config.RecordType))
	if !IsSupportedDNSRecordType(recordType) {
		return dnsFailure(monitor.ID, started, "invalid DNS record type")
	}

	timeout := time.Duration(monitor.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resolver := c.Resolver
	if resolver == nil {
		resolver = systemDNSResolver{resolver: net.DefaultResolver}
	}
	records, err := lookupDNS(checkCtx, resolver, target, recordType)
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		return DNSCheckResult{Result: service.CreateMonitorResultInput{
			MonitorID: monitor.ID, Status: "down", ResponseTimeMS: elapsed,
			Message: classifyDNSError(checkCtx, err),
		}, RecordType: recordType, Expected: config.ExpectedValues}
	}
	if len(records) == 0 {
		return DNSCheckResult{Result: service.CreateMonitorResultInput{
			MonitorID: monitor.ID, Status: "down", ResponseTimeMS: elapsed,
			Message: fmt.Sprintf("no %s records returned", recordType),
		}, RecordType: recordType, Records: records, Expected: config.ExpectedValues}
	}

	matched := matchesDNSExpected(recordType, records, config.ExpectedValues)
	if len(config.ExpectedValues) > 0 && !matched {
		return DNSCheckResult{Result: service.CreateMonitorResultInput{
			MonitorID: monitor.ID, Status: "down", ResponseTimeMS: elapsed,
			Message: "expected DNS value not found",
		}, RecordType: recordType, Records: records, Expected: config.ExpectedValues}
	}
	message := fmt.Sprintf("resolved %d %s records", len(records), recordType)
	if len(config.ExpectedValues) > 0 {
		message = fmt.Sprintf("DNS %s record matched expected value", recordType)
	}
	return DNSCheckResult{Result: service.CreateMonitorResultInput{
		MonitorID: monitor.ID, Status: "up", ResponseTimeMS: elapsed, Message: message,
	}, RecordType: recordType, Records: records, Expected: config.ExpectedValues, Matched: true}
}

func lookupDNS(ctx context.Context, resolver DNSResolver, target, recordType string) ([]string, error) {
	switch recordType {
	case DNSRecordA, DNSRecordAAAA:
		addresses, err := resolver.LookupIPAddr(ctx, target)
		if err != nil {
			return nil, err
		}
		result := make([]string, 0, len(addresses))
		for _, address := range addresses {
			if recordType == DNSRecordA && address.IP.To4() != nil {
				result = append(result, address.IP.To4().String())
			}
			if recordType == DNSRecordAAAA && address.IP.To4() == nil && address.IP.To16() != nil {
				result = append(result, address.IP.To16().String())
			}
		}
		return result, nil
	case DNSRecordCNAME:
		value, err := resolver.LookupCNAME(ctx, target)
		if err != nil {
			return nil, err
		}
		value = normalizeDNSHostname(value)
		if value == "" {
			return nil, nil
		}
		return []string{value}, nil
	case DNSRecordMX:
		values, err := resolver.LookupMX(ctx, target)
		if err != nil {
			return nil, err
		}
		result := make([]string, 0, len(values))
		for _, value := range values {
			if value != nil {
				result = append(result, normalizeDNSHostname(value.Host))
			}
		}
		return result, nil
	case DNSRecordNS:
		values, err := resolver.LookupNS(ctx, target)
		if err != nil {
			return nil, err
		}
		result := make([]string, 0, len(values))
		for _, value := range values {
			if value != nil {
				result = append(result, normalizeDNSHostname(value.Host))
			}
		}
		return result, nil
	case DNSRecordTXT:
		return resolver.LookupTXT(ctx, target)
	default:
		return nil, errors.New("invalid DNS record type")
	}
}

func ParseDNSTarget(value string) (string, error) {
	if strings.TrimSpace(value) != value {
		return "", errors.New("invalid DNS target: DNS name only")
	}
	target := value
	if target == "" {
		return "", errors.New("invalid DNS target: name is required")
	}
	if strings.HasSuffix(target, ".") {
		target = strings.TrimSuffix(target, ".")
	}
	if target == "" || net.ParseIP(target) != nil || len(target) > 253 || strings.HasPrefix(target, "-") {
		return "", errors.New("invalid DNS target: DNS name only")
	}
	for _, r := range target {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", errors.New("invalid DNS target: DNS name only")
		}
	}
	for _, label := range strings.Split(target, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid DNS target: invalid hostname label")
		}
		for _, r := range label {
			if !(r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return "", errors.New("invalid DNS target: DNS name only")
			}
		}
	}
	return target, nil
}

func IsSupportedDNSRecordType(value string) bool {
	_, ok := supportedDNSRecordTypes[strings.ToUpper(strings.TrimSpace(value))]
	return ok
}
func normalizeDNSHostname(value string) string { return strings.TrimSuffix(value, ".") }

func matchesDNSExpected(recordType string, records, expected []string) bool {
	for _, want := range expected {
		want = strings.TrimSpace(want)
		for _, got := range records {
			switch recordType {
			case DNSRecordA, DNSRecordAAAA:
				wantIP, gotIP := net.ParseIP(want), net.ParseIP(got)
				if wantIP != nil && gotIP != nil && ((recordType == DNSRecordA && wantIP.To4() != nil && gotIP.To4() != nil && wantIP.To4().Equal(gotIP.To4())) || (recordType == DNSRecordAAAA && wantIP.To4() == nil && gotIP.To4() == nil && wantIP.To16().Equal(gotIP.To16()))) {
					return true
				}
			case DNSRecordCNAME, DNSRecordMX, DNSRecordNS:
				if strings.EqualFold(normalizeDNSHostname(want), normalizeDNSHostname(got)) {
					return true
				}
			case DNSRecordTXT:
				if want == got {
					return true
				}
			}
		}
	}
	return false
}

func classifyDNSError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return "DNS lookup timed out"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsTimeout {
			return "DNS lookup timed out"
		}
		if dnsErr.IsNotFound {
			return "DNS name not found"
		}
		if dnsErr.IsTemporary {
			return "temporary DNS resolver failure"
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return "DNS lookup canceled"
	}
	return "DNS lookup failed: resolver error"
}

func dnsFailure(id int64, started time.Time, message string) DNSCheckResult {
	return DNSCheckResult{Result: service.CreateMonitorResultInput{MonitorID: id, Status: "down", ResponseTimeMS: time.Since(started).Milliseconds(), Message: message}}
}

package monitor

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

type fakeDNSResolver struct {
	ips   []net.IPAddr
	cname string
	mx    []*net.MX
	ns    []*net.NS
	txt   []string
	err   error
	wait  bool
}

func (f fakeDNSResolver) LookupIPAddr(ctx context.Context, _ string) ([]net.IPAddr, error) {
	if err := f.block(ctx); err != nil {
		return nil, err
	}
	return f.ips, f.err
}
func (f fakeDNSResolver) LookupCNAME(ctx context.Context, _ string) (string, error) {
	if err := f.block(ctx); err != nil {
		return "", err
	}
	return f.cname, f.err
}
func (f fakeDNSResolver) LookupMX(ctx context.Context, _ string) ([]*net.MX, error) {
	if err := f.block(ctx); err != nil {
		return nil, err
	}
	return f.mx, f.err
}
func (f fakeDNSResolver) LookupNS(ctx context.Context, _ string) ([]*net.NS, error) {
	if err := f.block(ctx); err != nil {
		return nil, err
	}
	return f.ns, f.err
}
func (f fakeDNSResolver) LookupTXT(ctx context.Context, _ string) ([]string, error) {
	if err := f.block(ctx); err != nil {
		return nil, err
	}
	return f.txt, f.err
}
func (f fakeDNSResolver) block(ctx context.Context) error {
	if f.wait {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func dnsMonitor(recordType string, expected ...string) service.Monitor {
	return service.Monitor{ID: 8, Target: "unit.test", TimeoutSeconds: 1, Config: &service.MonitorConfig{RecordType: recordType, ExpectedValues: expected}}
}

func TestDNSCheckerRecordsAndMatching(t *testing.T) {
	tests := []struct {
		name, recordType string
		resolver         fakeDNSResolver
		expected         []string
		status, message  string
	}{
		{"A lookup", "A", fakeDNSResolver{ips: []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}}, nil, "up", "resolved 1 A records"},
		{"A expected match", "A", fakeDNSResolver{ips: []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}}, []string{"203.0.113.10"}, "up", "DNS A record matched expected value"},
		{"A expected mismatch", "A", fakeDNSResolver{ips: []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}}, []string{"203.0.113.11"}, "down", "expected DNS value not found"},
		{"no A records", "A", fakeDNSResolver{ips: []net.IPAddr{{IP: net.ParseIP("2001:db8::1")}}}, nil, "down", "no A records returned"},
		{"AAAA canonical match", "AAAA", fakeDNSResolver{ips: []net.IPAddr{{IP: net.ParseIP("2001:0db8:0:0:0:0:0:1")}}}, []string{"2001:db8::1"}, "up", "DNS AAAA record matched expected value"},
		{"CNAME trailing dot", "CNAME", fakeDNSResolver{cname: "Target.Example.TEST."}, []string{"target.example.test"}, "up", "DNS CNAME record matched expected value"},
		{"MX trailing dot", "MX", fakeDNSResolver{mx: []*net.MX{{Host: "Mail.Example.TEST.", Pref: 10}}}, []string{"mail.example.test"}, "up", "DNS MX record matched expected value"},
		{"NS trailing dot", "NS", fakeDNSResolver{ns: []*net.NS{{Host: "Ns.Example.TEST."}}}, []string{"ns.example.test"}, "up", "DNS NS record matched expected value"},
		{"TXT exact match", "TXT", fakeDNSResolver{txt: []string{"verification-string"}}, []string{"verification-string"}, "up", "DNS TXT record matched expected value"},
		{"TXT mismatch", "TXT", fakeDNSResolver{txt: []string{"verification-string"}}, []string{"other"}, "down", "expected DNS value not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checked := (&DNSChecker{Resolver: test.resolver}).Check(context.Background(), dnsMonitor(test.recordType, test.expected...))
			if checked.Result.Status != test.status || checked.Result.Message != test.message {
				t.Fatalf("got %s %q, want %s %q", checked.Result.Status, checked.Result.Message, test.status, test.message)
			}
			if checked.Result.ResponseTimeMS < 0 {
				t.Fatal("response time was not populated")
			}
		})
	}
}

func TestDNSCheckerErrorsAndValidation(t *testing.T) {
	notFound := &net.DNSError{Err: "no such host", Name: "unit.test", IsNotFound: true}
	tests := []struct {
		name     string
		monitor  service.Monitor
		resolver fakeDNSResolver
		want     string
	}{
		{"timeout", dnsMonitor("A"), fakeDNSResolver{wait: true}, "DNS lookup timed out"},
		{"NXDOMAIN", dnsMonitor("A"), fakeDNSResolver{err: notFound}, "DNS name not found"},
		{"resolver error", dnsMonitor("A"), fakeDNSResolver{err: errors.New("resolver unavailable")}, "DNS lookup failed: resolver error"},
		{"empty target", service.Monitor{Target: "", Config: &service.MonitorConfig{RecordType: "A"}}, fakeDNSResolver{}, "invalid DNS target: name is required"},
		{"invalid URL", service.Monitor{Target: "https://unit.test", Config: &service.MonitorConfig{RecordType: "A"}}, fakeDNSResolver{}, "invalid DNS target: DNS name only"},
		{"port", service.Monitor{Target: "unit.test:53", Config: &service.MonitorConfig{RecordType: "A"}}, fakeDNSResolver{}, "invalid DNS target: DNS name only"},
		{"invalid record type", dnsMonitor("PTR"), fakeDNSResolver{}, "invalid DNS record type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := (&DNSChecker{Resolver: test.resolver}).Check(context.Background(), test.monitor)
			if got.Result.Message != test.want {
				t.Fatalf("got %q, want %q", got.Result.Message, test.want)
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checked := (&DNSChecker{Resolver: fakeDNSResolver{wait: true}}).Check(ctx, dnsMonitor("A"))
	if checked.Result.Message != "DNS lookup canceled" {
		t.Fatalf("got %q, want cancellation", checked.Result.Message)
	}
}

func TestParseDNSTarget(t *testing.T) {
	if got, err := ParseDNSTarget("Example.TEST."); err != nil || got != "Example.TEST" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, value := range []string{"-whatever", "a..example.test", "a/example", "a example"} {
		if _, err := ParseDNSTarget(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

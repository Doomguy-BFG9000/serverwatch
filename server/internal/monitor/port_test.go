package monitor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

func portMonitor(target string, timeoutSeconds int, ports ...int) service.Monitor {
	return service.Monitor{
		ID:             9,
		Name:           "Production Ports",
		Target:         target,
		TimeoutSeconds: timeoutSeconds,
		Config:         &service.MonitorConfig{Ports: ports},
	}
}

// listenPort starts a local listener on 127.0.0.1 and returns its port. Tests
// never depend on the public internet or external services.
func listenPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on 127.0.0.1:0: %v", err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	return listener.Addr().(*net.TCPAddr).Port
}

// closedPort returns a 127.0.0.1 port that nothing is listening on.
func closedPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on 127.0.0.1:0: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	return port
}

func TestPortCheckerSingleReachablePort(t *testing.T) {
	port := listenPort(t)

	checked := NewPortChecker().Check(context.Background(), portMonitor("127.0.0.1", 1, port))

	if checked.Result.Status != "up" {
		t.Fatalf("status = %q, want up", checked.Result.Status)
	}
	if checked.Result.Message != "1/1 ports reachable" {
		t.Fatalf("message = %q, want %q", checked.Result.Message, "1/1 ports reachable")
	}
	if checked.Reachable != 1 || checked.Total != 1 {
		t.Fatalf("reachable/total = %d/%d, want 1/1", checked.Reachable, checked.Total)
	}
	if len(checked.Ports) != 1 {
		t.Fatalf("port results = %d, want 1", len(checked.Ports))
	}

	result := checked.Ports[0]
	if result.Port != port {
		t.Fatalf("port = %d, want %d", result.Port, port)
	}
	if result.Status != "up" || result.Message != "reachable" {
		t.Fatalf("per-port result = %+v, want up/reachable", result)
	}
	if result.ResponseTimeMS < 0 || checked.Result.ResponseTimeMS < 0 {
		t.Fatalf("response times must be non-negative: %+v", checked)
	}
	if checked.Result.StatusCode != nil {
		t.Fatal("status code should be nil for port checks")
	}
}

func TestPortCheckerMultipleReachablePorts(t *testing.T) {
	first := listenPort(t)
	second := listenPort(t)

	checked := NewPortChecker().Check(context.Background(), portMonitor("127.0.0.1", 1, second, first))

	if checked.Result.Status != "up" {
		t.Fatalf("status = %q, want up", checked.Result.Status)
	}
	if checked.Result.Message != "2/2 ports reachable" {
		t.Fatalf("message = %q, want %q", checked.Result.Message, "2/2 ports reachable")
	}
	if checked.Reachable != 2 || checked.Total != 2 {
		t.Fatalf("reachable/total = %d/%d, want 2/2", checked.Reachable, checked.Total)
	}

	// Results are normalized (sorted ascending) so stored detail rows are
	// deterministic.
	want := []int{first, second}
	sort.Ints(want)
	for index, port := range want {
		if checked.Ports[index].Port != port {
			t.Fatalf("port results = %+v, want ascending %v", checked.Ports, want)
		}
		if checked.Ports[index].Status != "up" {
			t.Fatalf("port %d status = %q, want up", port, checked.Ports[index].Status)
		}
	}
}

func TestPortCheckerPartialFailureIsDown(t *testing.T) {
	reachable := listenPort(t)
	refused := closedPort(t)

	checked := NewPortChecker().Check(context.Background(), portMonitor("127.0.0.1", 1, reachable, refused))

	if checked.Result.Status != "down" {
		t.Fatalf("status = %q, want down", checked.Result.Status)
	}
	if checked.Reachable != 1 || checked.Total != 2 {
		t.Fatalf("reachable/total = %d/%d, want 1/2", checked.Reachable, checked.Total)
	}

	wantMessage := fmt.Sprintf("1/2 ports reachable; port %d unreachable", refused)
	if checked.Result.Message != wantMessage {
		t.Fatalf("message = %q, want %q", checked.Result.Message, wantMessage)
	}

	for _, result := range checked.Ports {
		if result.Port != refused {
			continue
		}
		if result.Message != "connection refused" {
			t.Fatalf("failed port message = %q, want %q", result.Message, "connection refused")
		}
		if result.ResponseTimeMS < 0 {
			t.Fatalf("failed port response time = %d, want non-negative", result.ResponseTimeMS)
		}
	}
}

func TestPortCheckerAllPortsFailedIsDown(t *testing.T) {
	first := closedPort(t)
	second := closedPort(t)

	checked := NewPortChecker().Check(context.Background(), portMonitor("127.0.0.1", 1, first, second))

	if checked.Result.Status != "down" {
		t.Fatalf("status = %q, want down", checked.Result.Status)
	}
	if checked.Reachable != 0 || checked.Total != 2 {
		t.Fatalf("reachable/total = %d/%d, want 0/2", checked.Reachable, checked.Total)
	}

	lower, upper := first, second
	if lower > upper {
		lower, upper = upper, lower
	}

	wantMessage := fmt.Sprintf("0/2 ports reachable; ports %d, %d unreachable", lower, upper)
	if checked.Result.Message != wantMessage {
		t.Fatalf("message = %q, want %q", checked.Result.Message, wantMessage)
	}
}

func TestPortCheckerReportsOverallWallClockDuration(t *testing.T) {
	ports := []int{listenPort(t), listenPort(t), listenPort(t)}

	checked := NewPortChecker().Check(context.Background(), portMonitor("127.0.0.1", 2, ports...))

	if checked.Result.Status != "up" {
		t.Fatalf("status = %q, want up", checked.Result.Status)
	}

	var summed int64
	for _, result := range checked.Ports {
		if result.ResponseTimeMS < 0 {
			t.Fatalf("port %d response time = %d, want non-negative", result.Port, result.ResponseTimeMS)
		}
		summed += result.ResponseTimeMS
	}

	// Overall response time is the wall-clock duration of the whole check, not
	// the sum of per-port latencies.
	if checked.Result.ResponseTimeMS > summed+50 {
		t.Fatalf("overall response time = %d, want wall-clock duration (per-port sum %d)", checked.Result.ResponseTimeMS, summed)
	}
}

func TestPortCheckerInvalidTargetIsDown(t *testing.T) {
	for _, target := range []string{"https://example.com", "example.com:443", "example.com/path"} {
		checked := NewPortChecker().Check(context.Background(), portMonitor(target, 1, 80))

		if checked.Result.Status != "down" {
			t.Fatalf("target %q status = %q, want down", target, checked.Result.Status)
		}
		if checked.Result.Message == "" {
			t.Fatalf("target %q message should not be empty", target)
		}
		if len(checked.Ports) != 0 || checked.Total != 0 {
			t.Fatalf("target %q ports = %+v, want no port results", target, checked.Ports)
		}
	}
}

func TestPortCheckerRequiresValidPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports []int
	}{
		{"missing config", nil},
		{"empty ports", []int{}},
		{"port below range", []int{0}},
		{"port above range", []int{65536}},
		{"negative port", []int{-1}},
	}

	for _, test := range tests {
		monitor := portMonitor("127.0.0.1", 1)
		if test.ports != nil {
			monitor.Config = &service.MonitorConfig{Ports: test.ports}
		} else {
			monitor.Config = nil
		}

		checked := NewPortChecker().Check(context.Background(), monitor)

		if checked.Result.Status != "down" {
			t.Fatalf("%s: status = %q, want down", test.name, checked.Result.Status)
		}
		if checked.Result.Message == "" {
			t.Fatalf("%s: message should not be empty", test.name)
		}
	}
}

// blockingDialer waits for the overall check deadline, mimicking a host that
// drops packets.
type blockingDialer struct{}

func (blockingDialer) DialContext(ctx context.Context, _ string, _ string) (net.Conn, error) {
	<-ctx.Done()

	return nil, ctx.Err()
}

func TestPortCheckerTimeout(t *testing.T) {
	started := time.Now()

	checked := (&PortChecker{Dialer: blockingDialer{}}).Check(
		context.Background(),
		portMonitor("192.0.2.1", 1, 80),
	)

	elapsed := time.Since(started)

	if checked.Result.Status != "down" {
		t.Fatalf("status = %q, want down", checked.Result.Status)
	}
	if checked.Ports[0].Message != "timeout" {
		t.Fatalf("per-port message = %q, want timeout", checked.Ports[0].Message)
	}
	if checked.Result.Message != "0/1 ports reachable; port 80 unreachable" {
		t.Fatalf("message = %q", checked.Result.Message)
	}
	if elapsed < 900*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("timeout took %s, want approximately the configured 1s deadline", elapsed)
	}
}

func TestPortCheckerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()

	checked := (&PortChecker{Dialer: blockingDialer{}}).Check(ctx, portMonitor("127.0.0.1", 30, 80, 443))

	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("canceled check took %s, want immediate return", elapsed)
	}
	if checked.Result.Status != "down" {
		t.Fatalf("status = %q, want down", checked.Result.Status)
	}
	if checked.Total != 2 {
		t.Fatalf("total = %d, want 2", checked.Total)
	}
	for _, result := range checked.Ports {
		if result.Status != "down" || result.Message != "check canceled" {
			t.Fatalf("canceled port result = %+v, want down/check canceled", result)
		}
	}
}

// concurrentDialer counts simultaneous dial attempts and succeeds after a
// short delay.
type concurrentDialer struct {
	current int32
	peak    int32
}

func (d *concurrentDialer) DialContext(ctx context.Context, _ string, _ string) (net.Conn, error) {
	active := atomic.AddInt32(&d.current, 1)

	for {
		peak := atomic.LoadInt32(&d.peak)
		if active <= peak || atomic.CompareAndSwapInt32(&d.peak, peak, active) {
			break
		}
	}

	defer atomic.AddInt32(&d.current, -1)

	select {
	case <-time.After(20 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	server, client := net.Pipe()
	_ = server.Close()

	return client, nil
}

func TestPortCheckerBoundsConcurrentPortChecks(t *testing.T) {
	dialer := &concurrentDialer{}

	reachable := make([]int, 0, MaxPortMonitorPorts)
	for port := 1; port <= MaxPortMonitorPorts; port++ {
		reachable = append(reachable, port)
	}

	checked := (&PortChecker{Dialer: dialer}).Check(
		context.Background(),
		portMonitor("192.0.2.1", 5, reachable...),
	)

	if checked.Result.Status != "up" {
		t.Fatalf("status = %q, want up", checked.Result.Status)
	}
	if checked.Reachable != MaxPortMonitorPorts {
		t.Fatalf("reachable = %d, want %d", checked.Reachable, MaxPortMonitorPorts)
	}

	peak := atomic.LoadInt32(&dialer.peak)
	if peak > maxConcurrentPortChecks {
		t.Fatalf("peak concurrent dials = %d, want at most %d", peak, maxConcurrentPortChecks)
	}
	if peak < 2 {
		t.Fatalf("peak concurrent dials = %d, want concurrent checks", peak)
	}
}

// recordingDialer records dialed addresses without opening a connection.
type recordingDialer struct {
	mutex     sync.Mutex
	addresses []string
}

func (d *recordingDialer) DialContext(_ context.Context, network string, address string) (net.Conn, error) {
	d.mutex.Lock()
	d.addresses = append(d.addresses, network+" "+address)
	d.mutex.Unlock()

	return nil, errors.New("no listener in test")
}

func TestPortCheckerDialsIPv6WithJoinedHostPort(t *testing.T) {
	dialer := &recordingDialer{}

	checked := (&PortChecker{Dialer: dialer}).Check(
		context.Background(),
		portMonitor("2001:db8::10", 1, 443),
	)

	if checked.Result.Status != "down" {
		t.Fatalf("status = %q, want down", checked.Result.Status)
	}

	dialer.mutex.Lock()
	defer dialer.mutex.Unlock()

	if len(dialer.addresses) != 1 {
		t.Fatalf("addresses = %v, want one dial", dialer.addresses)
	}
	if dialer.addresses[0] != "tcp [2001:db8::10]:443" {
		t.Fatalf("address = %q, want tcp [2001:db8::10]:443", dialer.addresses[0])
	}
}

func TestClassifyPortError(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want string
	}{
		{"deadline exceeded", context.Background(), context.DeadlineExceeded, "timeout"},
		{"context canceled", context.Background(), context.Canceled, "check canceled"},
		{"dns failure", context.Background(), &net.DNSError{Err: "no such host", Name: "unit.test"}, "DNS resolution failed"},
		{"connection refused", context.Background(), syscall.ECONNREFUSED, "connection refused"},
		{"host unreachable", context.Background(), syscall.EHOSTUNREACH, "host unreachable"},
		{"network unreachable", context.Background(), syscall.ENETUNREACH, "host unreachable"},
		{"generic failure", context.Background(), errors.New("boom"), "unreachable"},
		{"canceled ctx wins", canceledContext(), errors.New("boom"), "check canceled"},
	}

	for _, test := range tests {
		if got := classifyPortError(test.ctx, test.err); got != test.want {
			t.Fatalf("%s: classifyPortError = %q, want %q", test.name, got, test.want)
		}
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}

func TestNormalizePorts(t *testing.T) {
	twenty := make([]int, 0, MaxPortMonitorPorts)
	for port := 1; port <= MaxPortMonitorPorts; port++ {
		twenty = append(twenty, port)
	}

	twentyOne := append(append([]int{}, twenty...), MaxPortMonitorPorts+1)

	tests := []struct {
		name    string
		input   []int
		want    []int
		wantErr bool
	}{
		{"one valid port", []int{443}, []int{443}, false},
		{"multiple valid ports", []int{22, 80, 443}, []int{22, 80, 443}, false},
		{"duplicates collapse", []int{443, 80, 443, 22}, []int{22, 80, 443}, false},
		{"unsorted input is sorted", []int{443, 22, 80}, []int{22, 80, 443}, false},
		{"port 1 accepted", []int{1}, []int{1}, false},
		{"port 65535 accepted", []int{65535}, []int{65535}, false},
		{"twenty ports accepted", twenty, twenty, false},
		{"port 0 rejected", []int{0}, nil, true},
		{"port 65536 rejected", []int{65536}, nil, true},
		{"negative port rejected", []int{-1}, nil, true},
		{"mixed invalid port rejected", []int{22, 70000}, nil, true},
		{"no ports rejected", nil, nil, true},
		{"empty ports rejected", []int{}, nil, true},
		{"too many ports rejected", twentyOne, nil, true},
	}

	for _, test := range tests {
		got, err := NormalizePorts(test.input)

		if test.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error, got %v", test.name, got)
			}
			if got != nil {
				t.Fatalf("%s: ports = %v, want nil", test.name, got)
			}
			continue
		}

		if err != nil {
			t.Fatalf("%s: unexpected error: %v", test.name, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s: ports = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestParsePortTargetAcceptsHostsAndAddresses(t *testing.T) {
	valid := []string{
		"example.com",
		"server.example.com",
		"192.168.1.10",
		"2001:db8::10",
		"::1",
		"localhost",
	}

	for _, target := range valid {
		got, err := ParsePortTarget(target)
		if err != nil {
			t.Fatalf("target %q: unexpected error: %v", target, err)
		}
		if got != target {
			t.Fatalf("target %q: parsed = %q", target, got)
		}
	}

	// Bracket-wrapped IPv6 targets are normalized so dialing can add brackets
	// through net.JoinHostPort.
	got, err := ParsePortTarget("[2001:db8::10]")
	if err != nil {
		t.Fatalf("bracketed IPv6: unexpected error: %v", err)
	}
	if got != "2001:db8::10" {
		t.Fatalf("bracketed IPv6 parsed = %q, want 2001:db8::10", got)
	}
}

func TestParsePortTargetRejectsInvalidTargets(t *testing.T) {
	invalid := []string{
		"",
		" example.com",
		"example.com ",
		"exa mple.com",
		"exam\tple.com",
		"\nexample.com",
		"example\x00.com",
		"https://example.com",
		"http://example.com",
		"example.com/path",
		"example.com?x=1",
		"example.com#fragment",
		"example.com:443",
		"localhost:22",
		"-example.com",
		"bad_label.example.com",
		"example..com",
		"example.com.",
		"*",
		"999.999.999.999",
		"1.2.3",
	}

	for _, target := range invalid {
		if got, err := ParsePortTarget(target); err == nil {
			t.Fatalf("target %q: expected error, got %q", target, got)
		}
	}
}

func TestSummarizePortResults(t *testing.T) {
	tests := []struct {
		reachable   int
		total       int
		unreachable []int
		want        string
	}{
		{3, 3, nil, "3/3 ports reachable"},
		{2, 3, []int{443}, "2/3 ports reachable; port 443 unreachable"},
		{1, 3, []int{80, 443}, "1/3 ports reachable; ports 80, 443 unreachable"},
		{0, 1, []int{22}, "0/1 ports reachable; port 22 unreachable"},
	}

	for _, test := range tests {
		if got := summarizePortResults(test.reachable, test.total, test.unreachable); got != test.want {
			t.Fatalf("summarizePortResults = %q, want %q", got, test.want)
		}
	}
}

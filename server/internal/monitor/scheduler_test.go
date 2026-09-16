package monitor

import (
	"context"
	"testing"
)

// TestSchedulerDispatchesPortMonitorToPortChecker guards the Phase 0.36
// integration: the repository makes "port" monitors due, so the scheduler must
// dispatch them instead of reporting "unsupported monitor type".
func TestSchedulerDispatchesPortMonitorToPortChecker(t *testing.T) {
	dialer := &recordingDialer{}
	scheduler := &Scheduler{PortChecker: &PortChecker{Dialer: dialer}}

	monitorItem := portMonitor("127.0.0.1", 1, 8443)
	monitorItem.MonitorType = "port"

	result, sslResult, supported := scheduler.check(context.Background(), monitorItem)

	if !supported {
		t.Fatal(`monitor type "port" fell through to "unsupported monitor type"`)
	}
	if sslResult != nil {
		t.Fatalf("sslResult = %+v, want nil for a port monitor", sslResult)
	}
	if result.MonitorID != monitorItem.ID {
		t.Fatalf("result monitor id = %d, want %d", result.MonitorID, monitorItem.ID)
	}

	dialer.mutex.Lock()
	defer dialer.mutex.Unlock()

	if len(dialer.addresses) != 1 {
		t.Fatalf("dialed addresses = %v, want one dial from PortChecker", dialer.addresses)
	}
	if dialer.addresses[0] != "tcp 127.0.0.1:8443" {
		t.Fatalf("address = %q, want tcp 127.0.0.1:8443", dialer.addresses[0])
	}
}

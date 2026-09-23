package repository

import (
	"strings"
	"testing"
)

// TestListDueQueryExcludesMaintenanceMonitors guards the Phase 0.38 scheduler
// integration: due-monitor selection must stay enabled-only and additionally
// skip monitors that are in manual maintenance mode.
func TestListDueQueryExcludesMaintenanceMonitors(t *testing.T) {
	query := strings.ToLower(listDueQuery)

	if !strings.Contains(query, "m.enabled = true") {
		t.Fatal("ListDue query must keep the enabled-only filter")
	}
	if !strings.Contains(query, "m.maintenance = false") {
		t.Fatal("ListDue query must exclude monitors in maintenance")
	}

	for _, monitorType := range []string{"http", "https", "tcp", "ssl", "ping", "dns", "port"} {
		if !strings.Contains(query, "'"+monitorType+"'") {
			t.Fatalf("ListDue query does not include monitor type %q", monitorType)
		}
	}
}

// TestSetMaintenanceQueryIsIsolated verifies the maintenance toggle only writes
// monitors.maintenance. It must not touch check history or alert state, so it
// cannot create a synthetic result or trigger an alert transition.
func TestSetMaintenanceQueryIsIsolated(t *testing.T) {
	query := strings.ToLower(setMaintenanceQuery)

	if !strings.Contains(query, "update monitors") {
		t.Fatal("SetMaintenance must update the monitors table")
	}
	if !strings.Contains(query, "maintenance = $2") {
		t.Fatal("SetMaintenance must write the maintenance column")
	}

	for _, forbidden := range []string{"monitor_alert_state", "monitor_results", "insert", "delete"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("SetMaintenance query must not reference %q", forbidden)
		}
	}
}

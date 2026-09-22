package repository

import (
	"strings"
	"testing"
)

func TestListDueQueryDoesNotFilterByGroup(t *testing.T) {
	query := strings.ToLower(listDueQuery)

	if strings.Contains(query, "group") {
		t.Fatal("scheduler ListDue query must not filter or join on monitor groups")
	}

	for _, monitorType := range []string{"http", "https", "tcp", "ssl", "ping", "dns", "port"} {
		if !strings.Contains(query, "'"+monitorType+"'") {
			t.Fatalf("scheduler ListDue query does not include monitor type %q", monitorType)
		}
	}
}

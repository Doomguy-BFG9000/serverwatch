package storage

import (
	"strings"
	"testing"
)

func TestMonitorGroupsMigration(t *testing.T) {
	contents, err := migrationFiles.ReadFile("migrations/004_monitor_groups.sql")
	if err != nil {
		t.Fatalf("read monitor groups migration: %v", err)
	}
	migration := string(contents)

	required := []string{
		"CREATE TABLE IF NOT EXISTS monitor_groups",
		"name VARCHAR(100) NOT NULL",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_groups_name_lower",
		"ON monitor_groups (LOWER(name))",
		"group_id BIGINT",
		"ON DELETE SET NULL",
	}

	for _, fragment := range required {
		if !strings.Contains(migration, fragment) {
			t.Fatalf("migration does not contain %q", fragment)
		}
	}
}

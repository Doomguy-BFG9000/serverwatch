package storage

import (
	"os"
	"path/filepath"
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

func TestMonitorMaintenanceMigration(t *testing.T) {
	contents, err := migrationFiles.ReadFile("migrations/005_monitor_maintenance.sql")
	if err != nil {
		t.Fatalf("read monitor maintenance migration: %v", err)
	}
	migration := string(contents)

	required := []string{
		"ALTER TABLE monitors",
		"ADD COLUMN IF NOT EXISTS maintenance BOOLEAN NOT NULL DEFAULT FALSE",
	}

	for _, fragment := range required {
		if !strings.Contains(migration, fragment) {
			t.Fatalf("migration does not contain %q", fragment)
		}
	}
}

func TestMonitorFavoritesMigration(t *testing.T) {
	contents, err := migrationFiles.ReadFile("migrations/006_monitor_favorites.sql")
	if err != nil {
		t.Fatalf("read monitor favorites migration: %v", err)
	}
	migration := string(contents)

	required := []string{
		"ALTER TABLE monitors",
		"ADD COLUMN IF NOT EXISTS favorite BOOLEAN NOT NULL DEFAULT FALSE",
	}

	for _, fragment := range required {
		if !strings.Contains(migration, fragment) {
			t.Fatalf("migration does not contain %q", fragment)
		}
	}
}

func TestAuthMigration(t *testing.T) {
	contents, err := migrationFiles.ReadFile("migrations/007_auth.sql")
	if err != nil {
		t.Fatalf("read auth migration: %v", err)
	}
	migration := string(contents)

	required := []string{
		"CREATE TABLE IF NOT EXISTS sessions",
		"token_hash TEXT PRIMARY KEY",
		"user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE",
		"expires_at TIMESTAMPTZ NOT NULL",
	}

	for _, fragment := range required {
		if !strings.Contains(migration, fragment) {
			t.Fatalf("migration does not contain %q", fragment)
		}
	}
}

// TestMigrationsMirrored guards the project invariant that the two migration
// directories stay byte-for-byte identical. The embedded copy is what actually
// runs; the top-level database/migrations copy is the human-facing source. If
// they drift, operators reading one directory would apply different SQL than the
// server does.
func TestMigrationsMirrored(t *testing.T) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		embedded, err := migrationFiles.ReadFile(filepath.Join("migrations", entry.Name()))
		if err != nil {
			t.Fatalf("read embedded %s: %v", entry.Name(), err)
		}
		onDisk, err := os.ReadFile(filepath.Join("..", "..", "..", "database", "migrations", entry.Name()))
		if err != nil {
			t.Fatalf("read database/migrations/%s: %v (the two migration directories must stay in sync)", entry.Name(), err)
		}
		if string(embedded) != string(onDisk) {
			t.Fatalf("migration %s differs between server/internal/storage/migrations and database/migrations", entry.Name())
		}
	}
}

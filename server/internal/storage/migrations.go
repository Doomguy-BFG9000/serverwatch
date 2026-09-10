package storage

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration assets are compiled into the server so startup does not depend
// on the process working directory.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version string
	path    string
	sql     string
}

func RunMigrations(ctx context.Context, db *pgxpool.Pool) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('serverwatch:migrations'))`); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(100) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedMigrations(ctx, tx)
	if err != nil {
		return err
	}
	if _, ok := applied["001"]; !ok {
		legacy, err := legacySchemaState(ctx, tx)
		if err != nil {
			return err
		}
		switch legacy {
		case legacyComplete:
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ('001')`); err != nil {
				return fmt.Errorf("record legacy migration 001: %w", err)
			}
			applied["001"] = struct{}{}
		case legacyPartial:
			return fmt.Errorf("legacy database has an incomplete initial schema; refusing to run migrations")
		}
	}

	for _, item := range migrations {
		if _, ok := applied[item.version]; ok {
			continue
		}
		if _, err := tx.Exec(ctx, item.sql); err != nil {
			return fmt.Errorf("apply migration %s (%s): %w", item.version, item.path, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, item.version); err != nil {
			return fmt.Errorf("record migration %s: %w", item.version, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	items := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		contents, err := migrationFiles.ReadFile(filepath.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		items = append(items, migration{version: parts[0], path: entry.Name(), sql: string(contents)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	for i := 1; i < len(items); i++ {
		if items[i-1].version == items[i].version {
			return nil, fmt.Errorf("duplicate migration version %s", items[i].version)
		}
	}
	return items, nil
}

func appliedMigrations(ctx context.Context, tx pgx.Tx) (map[string]struct{}, error) {
	rows, err := tx.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	return applied, nil
}

type legacySchema int

const (
	legacyFresh legacySchema = iota
	legacyComplete
	legacyPartial
)

func legacySchemaState(ctx context.Context, tx pgx.Tx) (legacySchema, error) {
	const query = `SELECT to_regclass('public.users') IS NOT NULL,
		to_regclass('public.monitors') IS NOT NULL,
		to_regclass('public.monitor_results') IS NOT NULL`
	var users, monitors, results bool
	if err := tx.QueryRow(ctx, query).Scan(&users, &monitors, &results); err != nil {
		return legacyFresh, fmt.Errorf("inspect legacy schema: %w", err)
	}
	if !users && !monitors && !results {
		return legacyFresh, nil
	}
	if users && monitors && results {
		return legacyComplete, nil
	}
	return legacyPartial, nil
}

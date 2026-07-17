// Package migrations runs embedded PostgreSQL schema migrations.
package migrations

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var migrationFS embed.FS

// Run applies all embedded migrations that have not yet been recorded.
func Run(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrationFS.ReadDir("sql")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && !strings.HasPrefix(name, ".") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := runOne(ctx, db, name); err != nil {
			return fmt.Errorf("run migration %s: %w", name, err)
		}
	}

	return nil
}

func runOne(ctx context.Context, db *pgxpool.Pool, name string) error {
	var applied bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied); err != nil {
		return fmt.Errorf("check migration status: %w", err)
	}
	if applied {
		return nil
	}

	sql, err := migrationFS.ReadFile("sql/" + name)
	if err != nil {
		return fmt.Errorf("read migration SQL: %w", err)
	}

	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		// Migration files may contain multiple statements; simple protocol lets
		// PostgreSQL parse the file as a single script.
		if _, err := tx.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
			return fmt.Errorf("execute migration SQL: %w", err)
		}
		_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name)
		if err != nil {
			return fmt.Errorf("record migration: %w", err)
		}
		return nil
	})
}

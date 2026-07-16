// Package migrations runs embedded PostgreSQL schema migrations.
package migrations

import (
	"context"
	"embed"
	"sort"

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
		return err
	}

	entries, err := migrationFS.ReadDir("sql")
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := runOne(ctx, db, name); err != nil {
			return err
		}
	}

	return nil
}

func runOne(ctx context.Context, db *pgxpool.Pool, name string) error {
	var applied bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied); err != nil {
		return err
	}
	if applied {
		return nil
	}

	sql, err := migrationFS.ReadFile("sql/" + name)
	if err != nil {
		return err
	}

	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name)
		return err
	})
}

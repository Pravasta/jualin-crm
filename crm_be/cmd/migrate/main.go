// Command migrate runs schema migrations against DATABASE_URL.
//
// Usage:
//
//	migrate up      apply all pending migrations
//	migrate down    roll back one migration
//	migrate status  print applied/pending migrations
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/config"
	"github.com/Pravasta/jualin-crm/crm_be/migrations"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down|status>")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate: failed to open database:", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "migrate: failed to close database:", closeErr)
		}
	}()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}

	if err := run(sqlDB, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(db *sql.DB, cmd string) error {
	switch cmd {
	case "up":
		return goose.Up(db, ".")
	case "down":
		return goose.Down(db, ".")
	case "status":
		return goose.Status(db, ".")
	default:
		return fmt.Errorf("unknown command %q (want up|down|status)", cmd)
	}
}

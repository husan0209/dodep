package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/opus?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN NOT NULL DEFAULT FALSE,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`); err != nil {
		fmt.Fprintf(os.Stderr, "create migrations table: %v\n", err)
		os.Exit(1)
	}

	migrationsDir := "../../../libs/migrations/postgresql"
	if len(os.Args) > 1 {
		migrationsDir = os.Args[1]
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read dir %s: %v\n", migrationsDir, err)
		os.Exit(1)
	}

	type mig struct {
		version int64
		path    string
		name    string
	}

	var migs []mig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".up.sql")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) < 1 {
			continue
		}
		v, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		migs = append(migs, mig{version: v, path: filepath.Join(migrationsDir, e.Name()), name: base})
	}

	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	for _, m := range migs {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, m.version).Scan(&exists); err != nil {
			fmt.Fprintf(os.Stderr, "check version %d: %v\n", m.version, err)
			os.Exit(1)
		}
		if exists {
			fmt.Printf("SKIP   %s\n", m.name)
			continue
		}

		sqlBytes, err := os.ReadFile(m.path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", m.path, err)
			os.Exit(1)
		}
		sql := string(sqlBytes)

		tx, err := pool.Begin(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "begin tx for %d: %v\n", m.version, err)
			os.Exit(1)
		}

		if _, err := tx.Exec(ctx, sql); err != nil {
			tx.Rollback(ctx)
			fmt.Fprintf(os.Stderr, "apply %d: %v\n", m.version, err)
			os.Exit(1)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES ($1, FALSE)`, m.version); err != nil {
			tx.Rollback(ctx)
			fmt.Fprintf(os.Stderr, "record %d: %v\n", m.version, err)
			os.Exit(1)
		}

		if err := tx.Commit(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "commit %d: %v\n", m.version, err)
			os.Exit(1)
		}

		fmt.Printf("APPLY  %s\n", m.name)
	}

	fmt.Println("Done")
}

// Package db owns the Postgres connection pool, schema migrations, and every
// SQL query in the system (exposed as methods on *Store).
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aji/pulse/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver, used for goose
	"github.com/pressly/goose/v3"
)

// Store wraps a pgx connection pool and exposes typed query methods.
type Store struct {
	Pool *pgxpool.Pool
}

// Connect opens a pooled connection to Postgres and verifies it is reachable.
func Connect(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{Pool: pool}, nil
}

// Close releases the underlying pool.
func (s *Store) Close() { s.Pool.Close() }

// Migrate applies all embedded goose migrations. It opens its own database/sql
// handle (goose requires one) over the same connection string.
func Migrate(url string) error {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open sql: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrations.FS)
	goose.SetTableName("goose_db_version")
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

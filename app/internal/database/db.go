package database

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Connect establishes a connection pool to the PostgreSQL database and verifies it with a ping.
func Connect(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("database connection error: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping error: %w", err)
	}
	return db, nil
}

// Migrate runs database migrations from the given directory.
// Ignores ErrNoChange if there are no new migrations to apply.
func Migrate(migrationDir string, dbURL string) error {
	m, err := migrate.New("file://"+migrationDir, dbURL)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

package db

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB connection with SQLite-specific setup.
type DB struct {
	db *sql.DB
}

// NewDB opens (or creates) the SQLite database at path and returns a ready DB.
func NewDB(path string) (*DB, error) {
	d := &DB{}
	if err := d.Open(path); err != nil {
		return nil, err
	}
	return d, nil
}

// Open establishes the SQLite connection and applies connection pragmas.
func (d *DB) Open(path string) error {
	slog.Debug("opening db", "path", path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("sql open: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		if err := db.Close(); err != nil {
			slog.Error("failed to close db", "err", err)
		}
		return fmt.Errorf("enable WAL: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		if err := db.Close(); err != nil {
			slog.Error("failed to close db", "err", err)
		}
		return fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		if err := db.Close(); err != nil {
			slog.Error("failed to close db", "err", err)
		}
		return fmt.Errorf("set busy timeout: %w", err)
	}

	db.SetMaxOpenConns(1)

	d.db = db
	return nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// SQLDB returns the underlying *sql.DB for use by other packages.
func (d *DB) SQLDB() *sql.DB {
	return d.db
}

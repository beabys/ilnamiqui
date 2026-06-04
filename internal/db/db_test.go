package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if d.SQLDB() == nil {
		t.Fatal("SQLDB() returned nil")
	}

	// Verify the file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("db file was not created")
	}

	if err := d.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestWALMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal_test.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	defer d.Close()

	var journalMode string
	row := d.SQLDB().QueryRow("PRAGMA journal_mode")
	if err := row.Scan(&journalMode); err != nil {
		t.Fatalf("scan journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected journal_mode 'wal', got %q", journalMode)
	}
}

func TestForeignKeyEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fk_test.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	defer d.Close()

	var fkEnabled int
	row := d.SQLDB().QueryRow("PRAGMA foreign_keys")
	if err := row.Scan(&fkEnabled); err != nil {
		t.Fatalf("scan foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Fatalf("expected foreign_keys=1, got %d", fkEnabled)
	}
}

func TestDoubleClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "double_close.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("second close should be no-op: %v", err)
	}
}

func TestNewDB_InvalidPath(t *testing.T) {
	_, err := NewDB("/nonexistent/path/db.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestOpen_DirPath(t *testing.T) {
	// Using a directory path causes the first PRAGMA Exec to fail
	d := &DB{}
	err := d.Open(t.TempDir())
	if err == nil {
		t.Fatal("expected error opening directory as db path")
	}
}

func TestClose_NilDB(t *testing.T) {
	d := &DB{}
	if err := d.Close(); err != nil {
		t.Fatalf("Close on zero DB: %v", err)
	}
}

func TestSQLDB_NilOnUnopened(t *testing.T) {
	d := &DB{}
	if db := d.SQLDB(); db != nil {
		t.Fatal("SQLDB on unopened DB should return nil")
	}
}

func TestSQLDB_AfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sql_after_close.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	_ = d.Close()
	if db := d.SQLDB(); db == nil {
		t.Fatal("SQLDB after Close should still return non-nil *sql.DB")
	}
}

func TestRunMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migrate.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	defer d.Close()

	if err := RunMigrations(d.SQLDB()); err != nil {
		t.Fatalf("RunMigrations error: %v", err)
	}

	var count int
	row := d.SQLDB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('sessions', 'memory_entries', 'schema_versions')")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan table count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 tables, got %d", count)
	}

	// idempotent
	if err := RunMigrations(d.SQLDB()); err != nil {
		t.Fatalf("second RunMigrations should succeed: %v", err)
	}
}

func TestRunMigrations_ClosedDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "closed_migrate.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	d.Close()

	if err := RunMigrations(d.SQLDB()); err == nil {
		t.Fatal("expected error running migrations on closed DB")
	}
}

func TestRunMigrations_InvalidConn(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	db.Close()

	err = RunMigrations(db)
	if err == nil {
		t.Fatal("expected error with closed *sql.DB connection")
	}
}

func TestRunMigrations_Versioning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("NewDB error: %v", err)
	}
	defer d.Close()

	// First run: should apply v1
	if err := RunMigrations(d.SQLDB()); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	// Verify schema_versions has version 1
	var version int
	var desc string
	err = d.SQLDB().QueryRow("SELECT version, description FROM schema_versions ORDER BY version DESC LIMIT 1").Scan(&version, &desc)
	if err != nil {
		t.Fatalf("query schema_versions: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}

	// Second run: should skip (version already applied)
	if err := RunMigrations(d.SQLDB()); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	// Verify only one version record
	var count int
	_ = d.SQLDB().QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 version record, got %d", count)
	}
}

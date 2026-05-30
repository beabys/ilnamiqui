package db

import (
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
	// Try opening in a non-existent directory
	_, err := NewDB("/nonexistent/path/db.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

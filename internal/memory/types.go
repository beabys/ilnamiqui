package memory

import (
	"database/sql"
	"time"
)

// Store provides CRUD operations for memory entries backed by SQLite.
type Store struct {
	db *sql.DB
}

// Session represents a single opencode session within a project.
type Session struct {
	ID        string     `json:"id"`
	Project   string     `json:"project"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Summary   string     `json:"summary"`
}

// MemoryEntry is a single key-value memory belonging to a session.
type MemoryEntry struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

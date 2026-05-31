package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// NewStore creates a new Store backed by the given *sql.DB.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// SaveEntry inserts a new memory entry and returns it with the assigned ID and timestamp.
func (s *Store) SaveEntry(ctx context.Context, sessionID, key, value string) (*MemoryEntry, error) {
	const query = `INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, query, sessionID, key, value, now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("save entry: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("save entry last insert id: %w", err)
	}

	return &MemoryEntry{
		ID:        id,
		SessionID: sessionID,
		Key:       key,
		Value:     value,
		CreatedAt: now,
	}, nil
}

// LoadEntries returns entries for a session ordered by created_at DESC.
// If limit > 0, at most limit entries are returned.
func (s *Store) LoadEntries(ctx context.Context, sessionID string, limit int) ([]MemoryEntry, error) {
	query := `SELECT id, session_id, key, value, created_at FROM memory_entries WHERE session_id = ? ORDER BY created_at DESC, id DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		return s.queryEntries(ctx, query, sessionID, limit)
	}
	return s.queryEntries(ctx, query, sessionID)
}

// LoadAllEntries returns all entries for the project ordered by created_at DESC.
// If limit > 0, at most limit entries are returned.
func (s *Store) LoadAllEntries(ctx context.Context, limit int) ([]MemoryEntry, error) {
	query := `SELECT id, session_id, key, value, created_at FROM memory_entries ORDER BY created_at DESC, id DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		return s.queryEntries(ctx, query, limit)
	}
	return s.queryEntries(ctx, query)
}

// SearchEntries searches entries by key, value, and/or date range.
// queryStr can be empty (date-only search). after/before are optional date filters.
// If limit > 0, at most limit entries are returned.
func (s *Store) SearchEntries(ctx context.Context, queryStr string, limit int, after, before *time.Time) ([]MemoryEntry, error) {
	var conditions []string
	var args []any

	if queryStr != "" {
		conditions = append(conditions, "(key LIKE ? OR value LIKE ?)")
		pattern := "%" + queryStr + "%"
		args = append(args, pattern, pattern)
	}

	if after != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, after.UTC().Format(time.RFC3339))
	}

	if before != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, before.UTC().Format(time.RFC3339))
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := "SELECT id, session_id, key, value, created_at FROM memory_entries" + where + " ORDER BY created_at DESC, id DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	return s.queryEntries(ctx, query, args...)
}

// DeleteEntry deletes a memory entry by its ID.
func (s *Store) DeleteEntry(ctx context.Context, id int64) error {
	const query = `DELETE FROM memory_entries WHERE id = ?`

	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete entry %d: %w", id, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete entry rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("delete entry %d: not found", id)
	}
	return nil
}

// queryEntries is a helper to scan rows into MemoryEntry slices.
func (s *Store) queryEntries(ctx context.Context, query string, args ...any) ([]MemoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var createdAtStr string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Key, &e.Value, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse created_at %q: %w", createdAtStr, err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if entries == nil {
		entries = []MemoryEntry{}
	}
	return entries, nil
}

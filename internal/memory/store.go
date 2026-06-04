package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SearchMode controls which field(s) the search targets.
type SearchMode string

const (
	SearchModeKey     SearchMode = "key"     // default — key prefix LIKE (uses idx_memory_key index)
	SearchModeContent SearchMode = "content" // FTS5 MATCH on value (indexed full-text)
	SearchModeBoth    SearchMode = "both"    // key prefix LIKE OR FTS5 MATCH
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

// SearchEntries searches entries by key (default, uses idx_memory_key index),
// content (FTS5 MATCH), or both. queryStr can be empty (date-only search).
// after/before are optional date filters. If limit > 0, at most limit entries are returned.
func (s *Store) SearchEntries(ctx context.Context, queryStr string, mode SearchMode, limit int, after, before *time.Time) ([]MemoryEntry, error) {
	var conditions []string
	var args []any

	if queryStr != "" {
		switch mode {
		case SearchModeContent:
			conditions = append(conditions, `id IN (SELECT rowid FROM memory_fts WHERE memory_fts MATCH ?)`)
			args = append(args, toFTSQuery(queryStr))
		case SearchModeBoth:
			conditions = append(conditions, `(key LIKE ? OR id IN (SELECT rowid FROM memory_fts WHERE memory_fts MATCH ?))`)
			args = append(args, queryStr+"%", toFTSQuery(queryStr))
		default: // SearchModeKey
			conditions = append(conditions, "key LIKE ?")
			args = append(args, queryStr+"%")
		}
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

// ListKeys returns all distinct keys ordered by critical DESC, last_used_at DESC.
// If limit > 0, at most limit keys are returned.
func (s *Store) ListKeys(ctx context.Context, limit int) ([]KeyInfo, error) {
	query := `SELECT key, last_used_at, critical FROM memory_keys ORDER BY critical DESC, last_used_at DESC`
	if limit > 0 {
		query += " LIMIT ?"
		return s.queryKeyInfo(ctx, query, limit)
	}
	return s.queryKeyInfo(ctx, query)
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
	defer rows.Close() //nolint:errcheck

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

// queryKeyInfo is a helper to scan rows into KeyInfo slices.
func (s *Store) queryKeyInfo(ctx context.Context, query string, args ...any) ([]KeyInfo, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query key info: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var keys []KeyInfo
	for rows.Next() {
		var k KeyInfo
		var lastUsedStr string
		var criticalInt int
		if err := rows.Scan(&k.Key, &lastUsedStr, &criticalInt); err != nil {
			return nil, fmt.Errorf("scan key info: %w", err)
		}
		k.LastUsedAt, err = time.Parse(time.RFC3339, lastUsedStr)
		if err != nil {
			return nil, fmt.Errorf("parse last_used_at %q: %w", lastUsedStr, err)
		}
		k.Critical = criticalInt != 0
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	if keys == nil {
		keys = []KeyInfo{}
	}
	return keys, nil
}

// toFTSQuery converts user query to FTS5 prefix query syntax.
// Each word gets "*" appended so "arch design" matches tokens "architecture" "design".
func toFTSQuery(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	for i, w := range words {
		words[i] = w + "*"
	}
	return strings.Join(words, " ")
}

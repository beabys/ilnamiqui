package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/beabys/ilnamiqui/internal/memory"
)

// Agent constants for session identification.
const (
	AgentOpencode   = "opencode"
	AgentClaudeCode = "claude-code"
	AgentSystem     = "_system"
)

// Manager handles session lifecycle operations.
type Manager struct {
	db *sql.DB
}

// NewManager creates a new session Manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// StartSession creates a new session with a UUID and returns it.
func (m *Manager) StartSession(ctx context.Context, project string, agent string) (*memory.Session, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	if agent == "" {
		agent = AgentOpencode
	}

	const query = `INSERT INTO sessions (id, project, agent, started_at, created_at) VALUES (?, ?, ?, ?, ?)`

	if _, err := m.db.ExecContext(ctx, query, id, project, agent, now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	return &memory.Session{
		ID:        id,
		Project:   project,
		Agent:     agent,
		StartedAt: now,
	}, nil
}

// EndSession sets ended_at and summary on an existing session.
func (m *Manager) EndSession(ctx context.Context, id string, summary string) error {
	now := time.Now().UTC()

	const query = `UPDATE sessions SET ended_at = ?, summary = ? WHERE id = ?`

	res, err := m.db.ExecContext(ctx, query, now.Format(time.RFC3339), summary, id)
	if err != nil {
		return fmt.Errorf("end session %s: %w", id, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("end session rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("end session %s: not found", id)
	}
	return nil
}

// CloseSessionsByAgent ends all active sessions for a project and agent.
func (m *Manager) CloseSessionsByAgent(ctx context.Context, project, agent string) error {
	if agent == "" {
		agent = AgentOpencode
	}
	now := time.Now().UTC()
	const query = `UPDATE sessions SET ended_at = ?, summary = ? WHERE project = ? AND agent = ? AND ended_at IS NULL`
	_, err := m.db.ExecContext(ctx, query, now.Format(time.RFC3339), "auto-closed by new session", project, agent)
	if err != nil {
		return fmt.Errorf("close sessions by agent: %w", err)
	}
	return nil
}

// GetActiveSession returns the active session (NULL ended_at) for a project and agent.
// If none exists, a new session is automatically created and returned.
func (m *Manager) GetActiveSession(ctx context.Context, project, agent string) (*memory.Session, error) {
	if agent == "" {
		agent = AgentOpencode
	}
	const query = `SELECT id, project, agent, started_at, ended_at, summary FROM sessions WHERE project = ? AND agent = ? AND ended_at IS NULL ORDER BY started_at DESC LIMIT 1`

	row := m.db.QueryRowContext(ctx, query, project, agent)

	var s memory.Session
	var startedAtStr string
	var endedAtStr *string
	var summary string

	err := row.Scan(&s.ID, &s.Project, &s.Agent, &startedAtStr, &endedAtStr, &summary)
	if err == nil {
		s.StartedAt, err = time.Parse(time.RFC3339, startedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse started_at: %w", err)
		}
		if endedAtStr != nil {
			t, err := time.Parse(time.RFC3339, *endedAtStr)
			if err != nil {
				return nil, fmt.Errorf("parse ended_at: %w", err)
			}
			s.EndedAt = &t
		}
		s.Summary = summary
		return &s, nil
	}
	if err == sql.ErrNoRows {
		// No active session found – auto-create one
		return m.StartSession(ctx, project, agent)
	}
	return nil, fmt.Errorf("get active session: %w", err)
}

// ListSessions returns recent sessions for a project, ordered by started_at DESC.
func (m *Manager) ListSessions(ctx context.Context, project string, limit int) ([]memory.Session, error) {
	if limit <= 0 {
		limit = 10
	}

	const query = `SELECT id, project, agent, started_at, ended_at, summary FROM sessions WHERE project = ? ORDER BY started_at DESC LIMIT ?`

	rows, err := m.db.QueryContext(ctx, query, project, limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var sessions []memory.Session
	for rows.Next() {
		var s memory.Session
		var startedAtStr string
		var endedAtStr *string
		var summary string

		if err := rows.Scan(&s.ID, &s.Project, &s.Agent, &startedAtStr, &endedAtStr, &summary); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		s.StartedAt, err = time.Parse(time.RFC3339, startedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse started_at: %w", err)
		}
		if endedAtStr != nil {
			t, err := time.Parse(time.RFC3339, *endedAtStr)
			if err != nil {
				return nil, fmt.Errorf("parse ended_at: %w", err)
			}
			s.EndedAt = &t
		}
		s.Summary = summary
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if sessions == nil {
		sessions = []memory.Session{}
	}
	return sessions, nil
}

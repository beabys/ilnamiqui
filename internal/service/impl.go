package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/beabys/ilnamiqui/internal/config"
	"github.com/beabys/ilnamiqui/internal/memory"
	"github.com/beabys/ilnamiqui/internal/session"
)

type serviceImpl struct {
	config   Config
	dbOpener DBOpener
	mu       sync.Mutex
	store    *memory.Store
	mgr      *session.Manager
	database DB
	project  string
}

// New creates a Service with the given dependencies.
func New(cfg Config, opener DBOpener) Service {
	return &serviceImpl{config: cfg, dbOpener: opener}
}

// ensureDB opens the database if not already open. Caller must hold mu.
func (s *serviceImpl) ensureDB() error {
	if s.database != nil {
		return nil
	}

	// Lazy migration: if .ilnamiqui/ sentinel missing, attempt legacy migration.
	cwd, wdErr := os.Getwd()
	if wdErr == nil && !config.IsInitialized(cwd) && config.NeedsMigration(cwd) {
		if err := config.MigrateLegacy(cwd); err != nil {
			return fmt.Errorf("auto-migrate: %w", err)
		}
	}

	dbPath, err := s.config.DBPath()
	if err != nil {
		return fmt.Errorf("find db path: %w\n\nRun 'ilnamiqui init' first", err)
	}
	database, err := s.dbOpener.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	projectSlug, err := s.config.ProjectSlug()
	if err != nil {
		_ = database.Close()
		return err
	}
	s.database = database
	s.project = projectSlug
	s.store = memory.NewStore(database.SQLDB())
	s.mgr = session.NewManager(database.SQLDB())
	return nil
}

// Init creates .ilnamiqui directory and runs database migrations.
func (s *serviceImpl) Init(ctx context.Context, _ *InitRequest) (*InitResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close any existing connection
	if s.database != nil {
		_ = s.database.Close()
		s.database = nil
		s.store = nil
		s.mgr = nil
		s.project = ""
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}

	// Attempt migration from legacy .opencode/ storage first.
	if config.NeedsMigration(cwd) {
		if err := config.MigrateLegacy(cwd); err != nil {
			return nil, fmt.Errorf("migrate legacy: %w", err)
		}
	}

	ilnamiquiDir := cwd + "/.ilnamiqui"
	if err := os.MkdirAll(ilnamiquiDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .ilnamiqui directory: %w", err)
	}
	dbPath := ilnamiquiDir + "/ilnamiqui.db"
	database, err := s.dbOpener.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := s.dbOpener.RunMigrations(database.SQLDB()); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	// Write sentinel file so future calls skip migration checks
	if err := config.WriteSentinel(cwd); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("write sentinel: %w", err)
	}

	// Create project-path system entry if not exists.
	const systemSessionID = "00000000-0000-0000-0000-000000000000"
	var pathCount int
	_ = database.SQLDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_entries WHERE key = 'project-path'").Scan(&pathCount)
	if pathCount == 0 {
		if _, err := database.SQLDB().ExecContext(ctx,
			`INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`,
			systemSessionID, "project-path", cwd, time.Now().UTC().Format(time.RFC3339),
		); err == nil {
			// Mark the key as critical
			_, _ = database.SQLDB().Exec(
				`UPDATE memory_keys SET critical = 1 WHERE key = 'project-path'`,
			)
		}
	}

	// Re-read project slug
	s.database = database
	projectSlug, err := s.config.ProjectSlug()
	if err != nil {
		s.database = nil
		_ = database.Close()
		return nil, fmt.Errorf("project slug: %w", err)
	}
	s.project = projectSlug
	s.store = memory.NewStore(database.SQLDB())
	s.mgr = session.NewManager(database.SQLDB())

	return &InitResponse{DBPath: dbPath}, nil
}

// Save saves a memory entry for the active session.
func (s *serviceImpl) Save(ctx context.Context, req *SaveRequest) (*SaveResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetActiveSession(ctx, s.project)
	if err != nil {
		return nil, fmt.Errorf("get active session: %w", err)
	}
	entry, err := s.store.SaveEntry(ctx, sess.ID, req.Key, req.Value)
	if err != nil {
		return nil, fmt.Errorf("save entry: %w", err)
	}
	return &SaveResponse{Entry: entry}, nil
}

// Load loads memory entries, optionally filtered to the active session.
func (s *serviceImpl) Load(ctx context.Context, req *LoadRequest) (*LoadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	// Auto-reinit: ensure all schema tables exist (idempotent, version-gated)
	if s.database != nil {
		if err := s.dbOpener.RunMigrations(s.database.SQLDB()); err != nil {
			return nil, fmt.Errorf("auto-reinit: %w", err)
		}
	}

	if req.SessionOnly {
		sess, err := s.mgr.GetActiveSession(ctx, s.project)
		if err != nil {
			return nil, fmt.Errorf("get active session: %w", err)
		}
		entries, err := s.store.LoadEntries(ctx, sess.ID, req.Limit)
		if err != nil {
			return nil, fmt.Errorf("load entries: %w", err)
		}
		return &LoadResponse{Entries: entries}, nil
	}
	entries, err := s.store.LoadAllEntries(ctx, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("load all entries: %w", err)
	}
	return &LoadResponse{Entries: entries}, nil
}

// Search searches memory entries by key, content (FTS5), or both.
func (s *serviceImpl) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	mode := req.Mode
	if mode == "" {
		mode = memory.SearchModeKey
	}
	entries, err := s.store.SearchEntries(ctx, req.Query, mode, req.Limit, req.After, req.Before)
	if err != nil {
		return nil, fmt.Errorf("search entries: %w", err)
	}
	return &SearchResponse{Entries: entries}, nil
}

// ListSessions lists recent sessions for the project.
func (s *serviceImpl) ListSessions(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	sessions, err := s.mgr.ListSessions(ctx, s.project, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return &ListSessionsResponse{Sessions: sessions}, nil
}

// ListKeys returns all distinct memory keys.
func (s *serviceImpl) ListKeys(ctx context.Context, req *ListKeysRequest) (*ListKeysResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	keys, err := s.store.ListKeys(ctx, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	return &ListKeysResponse{Keys: keys}, nil
}

// Delete deletes a memory entry by ID.
func (s *serviceImpl) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	if err := s.store.DeleteEntry(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("delete entry: %w", err)
	}
	return &DeleteResponse{}, nil
}

// StartSession starts a new session.
func (s *serviceImpl) StartSession(ctx context.Context, _ *StartSessionRequest) (*StartSessionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	sess, err := s.mgr.StartSession(ctx, s.project)
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}
	return &StartSessionResponse{Session: sess}, nil
}

// EndSession ends the active session with a summary.
func (s *serviceImpl) EndSession(ctx context.Context, req *EndSessionRequest) (*EndSessionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetActiveSession(ctx, s.project)
	if err != nil {
		return nil, fmt.Errorf("get active session: %w", err)
	}
	if err := s.mgr.EndSession(ctx, sess.ID, req.Summary); err != nil {
		return nil, fmt.Errorf("end session: %w", err)
	}
	return &EndSessionResponse{Session: sess}, nil
}

// Close closes the database connection.
func (s *serviceImpl) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.database != nil {
		return s.database.Close()
	}
	return nil
}

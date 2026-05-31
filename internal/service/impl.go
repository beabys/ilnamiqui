package service

import (
	"context"
	"fmt"
	"os"
	"sync"

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
		database.Close()
		return err
	}
	s.database = database
	s.project = projectSlug
	s.store = memory.NewStore(database.SQLDB())
	s.mgr = session.NewManager(database.SQLDB())
	return nil
}

// Init creates .opencode directory and runs database migrations.
func (s *serviceImpl) Init(ctx context.Context, _ *InitRequest) (*InitResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close any existing connection
	if s.database != nil {
		s.database.Close()
		s.database = nil
		s.store = nil
		s.mgr = nil
		s.project = ""
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	opencodeDir := cwd + "/.opencode"
	if err := os.MkdirAll(opencodeDir, 0o755); err != nil {
		return nil, fmt.Errorf("create .opencode directory: %w", err)
	}
	dbPath := opencodeDir + "/ilnamiqui.db"
	database, err := s.dbOpener.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := s.dbOpener.RunMigrations(database.SQLDB()); err != nil {
		database.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	// Re-read project slug
	s.database = database
	projectSlug, err := s.config.ProjectSlug()
	if err != nil {
		s.database = nil
		database.Close()
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

// Search searches memory entries by key or value.
func (s *serviceImpl) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	entries, err := s.store.SearchEntries(ctx, req.Query, req.Limit, req.After, req.Before)
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

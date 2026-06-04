package service

import (
	"time"

	"github.com/beabys/ilnamiqui/internal/memory"
)

type InitRequest struct{}
type InitResponse struct{ DBPath string }

type SaveRequest struct {
	Key   string
	Value string
	Agent string
}
type SaveResponse struct {
	Entry *memory.MemoryEntry
}

type LoadRequest struct {
	Limit       int
	SessionOnly bool
}
type LoadResponse struct {
	Entries []memory.MemoryEntry
}

type SearchRequest struct {
	Query  string
	Mode   memory.SearchMode
	Limit  int
	After  *time.Time
	Before *time.Time
}
type SearchResponse struct {
	Entries []memory.MemoryEntry
}

type ListSessionsRequest struct {
	Limit int
}
type ListSessionsResponse struct {
	Sessions []memory.Session
}

type DeleteRequest struct {
	ID int64
}
type DeleteResponse struct{}

type StartSessionRequest struct {
	Agent string
}
type StartSessionResponse struct {
	Session *memory.Session
}

type EndSessionRequest struct {
	Summary string
	Agent   string
}
type EndSessionResponse struct {
	Session *memory.Session
}

type ListKeysRequest struct {
	Limit int
}
type ListKeysResponse struct {
	Keys []memory.KeyInfo
}

type KeyUpdateRequest struct {
	Key      string
	Critical bool
}
type KeyUpdateResponse struct{}

type PruneRequest struct {
	Before time.Time
	Key    string // "*" or empty = all non-critical keys
}
type PruneResponse struct {
	Deleted        int
	OrphansCleaned int
}

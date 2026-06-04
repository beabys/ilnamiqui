package service

import "context"

// Service is the canonical interface for memory operations.
// Adapters (CLI, MCP) depend on this interface.
type Service interface {
	Init(ctx context.Context, req *InitRequest) (*InitResponse, error)
	Save(ctx context.Context, req *SaveRequest) (*SaveResponse, error)
	Load(ctx context.Context, req *LoadRequest) (*LoadResponse, error)
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
	ListSessions(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error)
	Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error)
	ListKeys(ctx context.Context, req *ListKeysRequest) (*ListKeysResponse, error)
	StartSession(ctx context.Context, req *StartSessionRequest) (*StartSessionResponse, error)
	EndSession(ctx context.Context, req *EndSessionRequest) (*EndSessionResponse, error)
	Close() error
}

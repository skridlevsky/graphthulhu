package backend

import (
	"context"
	"encoding/json"

	"github.com/skridlevsky/graphthulhu/types"
)

// Backend is the interface every knowledge graph backend must implement.
// The Logseq client (client.Client) satisfies this interface.
// Future backends (e.g. Obsidian vault) implement the same contract.
type Backend interface {
	// Core read operations
	GetAllPages(ctx context.Context) ([]types.PageEntity, error)
	GetPage(ctx context.Context, nameOrID any) (*types.PageEntity, error)
	GetPageBlocksTree(ctx context.Context, nameOrID any) ([]types.BlockEntity, error)
	GetBlock(ctx context.Context, uuid string, opts ...map[string]any) (*types.BlockEntity, error)
	GetPageLinkedReferences(ctx context.Context, nameOrID any) (json.RawMessage, error)

	// Query operations
	DatascriptQuery(ctx context.Context, query string, inputs ...any) (json.RawMessage, error)

	// Write operations
	CreatePage(ctx context.Context, name string, properties map[string]any, opts map[string]any) (*types.PageEntity, error)
	AppendBlockInPage(ctx context.Context, page string, content string) (*types.BlockEntity, error)
	PrependBlockInPage(ctx context.Context, page string, content string) (*types.BlockEntity, error)
	InsertBlock(ctx context.Context, srcBlock any, content string, opts map[string]any) (*types.BlockEntity, error)
	UpdateBlock(ctx context.Context, uuid string, content string, opts ...map[string]any) error
	RemoveBlock(ctx context.Context, uuid string) error
	MoveBlock(ctx context.Context, uuid string, targetUUID string, opts map[string]any) error

	// Connectivity
	Ping(ctx context.Context) error
}

// HasDataScript is a marker interface for backends supporting DataScript queries.
// Logseq implements this; Obsidian does not.
type HasDataScript interface {
	HasDataScript()
}

// TagSearcher is implemented by backends that support tag search without DataScript.
type TagSearcher interface {
	FindBlocksByTag(ctx context.Context, tag string, includeChildren bool) ([]TagResult, error)
}

// PropertySearcher is implemented by backends that support property search without DataScript.
type PropertySearcher interface {
	FindByProperty(ctx context.Context, key, value, operator string) ([]PropertyResult, error)
}

// JournalSearcher is implemented by backends that support journal search without DataScript.
type JournalSearcher interface {
	SearchJournals(ctx context.Context, query string, from, to string) ([]JournalResult, error)
}

// TagResult holds a block found by tag search, grouped by page.
type TagResult struct {
	Page   string             `json:"page"`
	Blocks []types.BlockEntity `json:"blocks"`
}

// PropertyResult holds a page or block found by property search.
type PropertyResult struct {
	Type       string         `json:"type"` // "page" or "block"
	Name       string         `json:"name,omitempty"`
	UUID       string         `json:"uuid,omitempty"`
	Properties map[string]any `json:"properties"`
}

// JournalResult holds a journal entry found by search.
type JournalResult struct {
	Date    string              `json:"date"`
	Page    string              `json:"page"`
	Blocks  []types.BlockEntity `json:"blocks"`
}

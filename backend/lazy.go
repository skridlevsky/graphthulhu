package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/skridlevsky/graphthulhu/types"
)

// Compile-time check: *LazyBackend must always satisfy Backend.
var _ Backend = (*LazyBackend)(nil)

// IndexableBackend is a Backend that also serves the index-driven search
// capabilities. LazyBackend is built for backends of this shape — the
// "indexing" is exactly what takes time to start. Naming the precondition
// at the type level lets the wrapper forward capability calls directly,
// without runtime guards.
type IndexableBackend interface {
	Backend
	FullTextSearcher
	TagSearcher
	PropertySearcher
	JournalSearcher
}

// LazyBackend wraps an IndexableBackend that needs time to initialize.
// It responds to Ping immediately but blocks all other calls until the
// underlying backend signals readiness.
type LazyBackend struct {
	inner IndexableBackend
	ready chan struct{}
	err   error
	once  sync.Once // guards close(ready) so MarkReady/MarkFailed are idempotent
}

// NewLazyBackend returns a wrapper around inner. Call MarkReady or MarkFailed
// from a goroutine once initialization is complete.
func NewLazyBackend(inner IndexableBackend) *LazyBackend {
	return &LazyBackend{
		inner: inner,
		ready: make(chan struct{}),
	}
}

// MarkReady signals that the underlying backend is fully initialized.
// Safe to call multiple times; only the first call wins.
func (lb *LazyBackend) MarkReady() {
	lb.once.Do(func() { close(lb.ready) })
}

// MarkFailed signals that initialization failed. All subsequent calls
// will return this error. Safe to call multiple times; only the first
// call wins. err must be non-nil.
func (lb *LazyBackend) MarkFailed(err error) {
	lb.once.Do(func() {
		lb.err = err
		close(lb.ready)
	})
}

func (lb *LazyBackend) wait(ctx context.Context) error {
	select {
	case <-lb.ready:
		return lb.err
	case <-ctx.Done():
		return fmt.Errorf("vault is still loading: %w", ctx.Err())
	}
}

// Ping responds immediately — the server is alive even while loading.
func (lb *LazyBackend) Ping(ctx context.Context) error {
	return nil
}

// --- Core read operations ---

func (lb *LazyBackend) GetAllPages(ctx context.Context) ([]types.PageEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.GetAllPages(ctx)
}

func (lb *LazyBackend) GetPage(ctx context.Context, nameOrID any) (*types.PageEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.GetPage(ctx, nameOrID)
}

func (lb *LazyBackend) GetPageBlocksTree(ctx context.Context, nameOrID any) ([]types.BlockEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.GetPageBlocksTree(ctx, nameOrID)
}

func (lb *LazyBackend) GetBlock(ctx context.Context, uuid string, opts ...map[string]any) (*types.BlockEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.GetBlock(ctx, uuid, opts...)
}

func (lb *LazyBackend) GetPageLinkedReferences(ctx context.Context, nameOrID any) (json.RawMessage, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.GetPageLinkedReferences(ctx, nameOrID)
}

// --- Query operations ---

func (lb *LazyBackend) DatascriptQuery(ctx context.Context, query string, inputs ...any) (json.RawMessage, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.DatascriptQuery(ctx, query, inputs...)
}

// --- Write operations ---

func (lb *LazyBackend) CreatePage(ctx context.Context, name string, properties map[string]any, opts map[string]any) (*types.PageEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.CreatePage(ctx, name, properties, opts)
}

func (lb *LazyBackend) AppendBlockInPage(ctx context.Context, page string, content string) (*types.BlockEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.AppendBlockInPage(ctx, page, content)
}

func (lb *LazyBackend) PrependBlockInPage(ctx context.Context, page string, content string) (*types.BlockEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.PrependBlockInPage(ctx, page, content)
}

func (lb *LazyBackend) InsertBlock(ctx context.Context, srcBlock any, content string, opts map[string]any) (*types.BlockEntity, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.InsertBlock(ctx, srcBlock, content, opts)
}

func (lb *LazyBackend) UpdateBlock(ctx context.Context, uuid string, content string, opts ...map[string]any) error {
	if err := lb.wait(ctx); err != nil {
		return err
	}
	return lb.inner.UpdateBlock(ctx, uuid, content, opts...)
}

func (lb *LazyBackend) RemoveBlock(ctx context.Context, uuid string) error {
	if err := lb.wait(ctx); err != nil {
		return err
	}
	return lb.inner.RemoveBlock(ctx, uuid)
}

func (lb *LazyBackend) MoveBlock(ctx context.Context, uuid string, targetUUID string, opts map[string]any) error {
	if err := lb.wait(ctx); err != nil {
		return err
	}
	return lb.inner.MoveBlock(ctx, uuid, targetUUID, opts)
}

// --- Page management ---

func (lb *LazyBackend) DeletePage(ctx context.Context, name string) error {
	if err := lb.wait(ctx); err != nil {
		return err
	}
	return lb.inner.DeletePage(ctx, name)
}

func (lb *LazyBackend) RenamePage(ctx context.Context, oldName, newName string) error {
	if err := lb.wait(ctx); err != nil {
		return err
	}
	return lb.inner.RenamePage(ctx, oldName, newName)
}

// --- Capability interfaces (typed precisely on inner; no fallback needed) ---

func (lb *LazyBackend) FullTextSearch(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.FullTextSearch(ctx, query, limit)
}

func (lb *LazyBackend) FindBlocksByTag(ctx context.Context, tag string, includeChildren bool) ([]TagResult, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.FindBlocksByTag(ctx, tag, includeChildren)
}

func (lb *LazyBackend) FindByProperty(ctx context.Context, key, value, operator string) ([]PropertyResult, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.FindByProperty(ctx, key, value, operator)
}

func (lb *LazyBackend) SearchJournals(ctx context.Context, query string, from, to string) ([]JournalResult, error) {
	if err := lb.wait(ctx); err != nil {
		return nil, err
	}
	return lb.inner.SearchJournals(ctx, query, from, to)
}

package backend_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/skridlevsky/graphthulhu/backend"
	"github.com/skridlevsky/graphthulhu/types"
)

// stubBackend satisfies backend.IndexableBackend with benign canned return
// values, so tests can verify wrapper behavior (blocking, error paths,
// capability forwarding) without standing up a real vault.
type stubBackend struct{}

func (stubBackend) GetAllPages(context.Context) ([]types.PageEntity, error) { return nil, nil }
func (stubBackend) GetPage(context.Context, any) (*types.PageEntity, error) {
	return &types.PageEntity{Name: "ok"}, nil
}
func (stubBackend) GetPageBlocksTree(context.Context, any) ([]types.BlockEntity, error) {
	return nil, nil
}
func (stubBackend) GetBlock(context.Context, string, ...map[string]any) (*types.BlockEntity, error) {
	return nil, nil
}
func (stubBackend) GetPageLinkedReferences(context.Context, any) (json.RawMessage, error) {
	return nil, nil
}
func (stubBackend) DatascriptQuery(context.Context, string, ...any) (json.RawMessage, error) {
	return nil, nil
}
func (stubBackend) CreatePage(context.Context, string, map[string]any, map[string]any) (*types.PageEntity, error) {
	return nil, nil
}
func (stubBackend) AppendBlockInPage(context.Context, string, string) (*types.BlockEntity, error) {
	return nil, nil
}
func (stubBackend) PrependBlockInPage(context.Context, string, string) (*types.BlockEntity, error) {
	return nil, nil
}
func (stubBackend) InsertBlock(context.Context, any, string, map[string]any) (*types.BlockEntity, error) {
	return nil, nil
}
func (stubBackend) UpdateBlock(context.Context, string, string, ...map[string]any) error {
	return nil
}
func (stubBackend) RemoveBlock(context.Context, string) error                       { return nil }
func (stubBackend) MoveBlock(context.Context, string, string, map[string]any) error { return nil }
func (stubBackend) DeletePage(context.Context, string) error                        { return nil }
func (stubBackend) RenamePage(context.Context, string, string) error                { return nil }
func (stubBackend) Ping(context.Context) error                                      { return nil }

func (stubBackend) FullTextSearch(context.Context, string, int) ([]backend.SearchHit, error) {
	return []backend.SearchHit{{PageName: "p", UUID: "u", Content: "c"}}, nil
}
func (stubBackend) FindBlocksByTag(context.Context, string, bool) ([]backend.TagResult, error) {
	return []backend.TagResult{{Page: "p"}}, nil
}
func (stubBackend) FindByProperty(context.Context, string, string, string) ([]backend.PropertyResult, error) {
	return []backend.PropertyResult{{Type: "page", Name: "p"}}, nil
}
func (stubBackend) SearchJournals(context.Context, string, string, string) ([]backend.JournalResult, error) {
	return []backend.JournalResult{{Page: "j"}}, nil
}

func TestLazyBackend_PingRespondsBeforeReady(t *testing.T) {
	lb := backend.NewLazyBackend(stubBackend{})
	// Never call MarkReady — Ping must still succeed immediately.
	if err := lb.Ping(context.Background()); err != nil {
		t.Fatalf("Ping should respond immediately while loading, got %v", err)
	}
}

func TestLazyBackend_CallsBlockUntilReady(t *testing.T) {
	lb := backend.NewLazyBackend(stubBackend{})

	done := make(chan error, 1)
	go func() {
		_, err := lb.GetPage(context.Background(), "anything")
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("expected GetPage to block before MarkReady, got immediate return err=%v", err)
	case <-time.After(50 * time.Millisecond):
		// expected: still blocked
	}

	lb.MarkReady()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after MarkReady, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("GetPage did not return within 1s after MarkReady")
	}
}

func TestLazyBackend_MarkFailedPropagates(t *testing.T) {
	sentinel := errors.New("load broke")
	lb := backend.NewLazyBackend(stubBackend{})
	lb.MarkFailed(sentinel)

	_, err := lb.GetPage(context.Background(), "x")
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error from GetPage, got %v", err)
	}
}

func TestLazyBackend_ContextCancelDuringWait(t *testing.T) {
	lb := backend.NewLazyBackend(stubBackend{})
	// Never call MarkReady — every call should time out.

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := lb.GetPage(ctx, "x")
	if err == nil {
		t.Fatal("expected error from cancelled ctx, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected wrapped DeadlineExceeded, got %v", err)
	}
	if !strings.Contains(err.Error(), "still loading") {
		t.Errorf("expected 'still loading' message, got %v", err)
	}
}

func TestLazyBackend_MarkReadyIdempotent(t *testing.T) {
	lb := backend.NewLazyBackend(stubBackend{})
	lb.MarkReady()
	lb.MarkReady()                        // must not panic on close-of-closed-channel
	lb.MarkFailed(errors.New("too late")) // must not overwrite the success state

	if _, err := lb.GetPage(context.Background(), "x"); err != nil {
		t.Errorf("first MarkReady should win, got %v", err)
	}
}

func TestLazyBackend_FirstFailWins(t *testing.T) {
	sentinel := errors.New("first fail")
	lb := backend.NewLazyBackend(stubBackend{})
	lb.MarkFailed(sentinel)
	lb.MarkReady() // must not clear the error

	_, err := lb.GetPage(context.Background(), "x")
	if !errors.Is(err, sentinel) {
		t.Errorf("first MarkFailed should win, got %v", err)
	}
}

func TestLazyBackend_CapabilityCallsForwarded(t *testing.T) {
	lb := backend.NewLazyBackend(stubBackend{})
	lb.MarkReady()

	hits, err := lb.FullTextSearch(context.Background(), "q", 10)
	if err != nil {
		t.Fatalf("expected forwarded result, got %v", err)
	}
	if len(hits) != 1 || hits[0].PageName != "p" {
		t.Errorf("unexpected hits: %+v", hits)
	}

	tags, err := lb.FindBlocksByTag(context.Background(), "t", false)
	if err != nil || len(tags) != 1 {
		t.Errorf("FindBlocksByTag forwarding broken: tags=%v err=%v", tags, err)
	}

	props, err := lb.FindByProperty(context.Background(), "k", "v", "eq")
	if err != nil || len(props) != 1 {
		t.Errorf("FindByProperty forwarding broken: props=%v err=%v", props, err)
	}

	jrs, err := lb.SearchJournals(context.Background(), "q", "", "")
	if err != nil || len(jrs) != 1 {
		t.Errorf("SearchJournals forwarding broken: jrs=%v err=%v", jrs, err)
	}
}

package vault

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/skridlevsky/graphthulhu/backend"
)

// Compile-time check: *Client satisfies backend.Backend.
var _ backend.Backend = (*Client)(nil)

func testVault(t *testing.T) *Client {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(thisFile), "testdata")

	c := New(testdata)
	if err := c.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	c.BuildBacklinks()
	return c
}

func TestLoad(t *testing.T) {
	c := testVault(t)

	// Should have pages for: index, projects/graphthulhu, projects/openchaos,
	// people/Hanna, daily notes/2026-01-31, daily notes/2026-02-01.
	// Plus the alias "graphthulhu-mcp" pointing to projects/graphthulhu.
	pages, err := c.GetAllPages(context.Background())
	if err != nil {
		t.Fatalf("GetAllPages: %v", err)
	}

	// Should NOT include .obsidian directory contents.
	for _, p := range pages {
		if strings.Contains(p.Name, ".obsidian") {
			t.Errorf("hidden directory leaked: %s", p.Name)
		}
	}

	if len(pages) < 6 {
		t.Errorf("expected at least 6 pages, got %d", len(pages))
		for _, p := range pages {
			t.Logf("  page: %s", p.Name)
		}
	}
}

func TestGetPage(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	t.Run("exact name", func(t *testing.T) {
		page, err := c.GetPage(ctx, "projects/graphthulhu")
		if err != nil {
			t.Fatalf("GetPage: %v", err)
		}
		if page == nil {
			t.Fatal("page not found")
		}
		if page.Properties == nil {
			t.Fatal("expected properties")
		}
		if page.Properties["type"] != "project" {
			t.Errorf("type = %v, want project", page.Properties["type"])
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		page, err := c.GetPage(ctx, "Projects/Graphthulhu")
		if err != nil {
			t.Fatalf("GetPage: %v", err)
		}
		if page == nil {
			t.Fatal("page not found with different case")
		}
	})

	t.Run("alias lookup", func(t *testing.T) {
		page, err := c.GetPage(ctx, "graphthulhu-mcp")
		if err != nil {
			t.Fatalf("GetPage: %v", err)
		}
		if page == nil {
			t.Fatal("alias lookup failed")
		}
		if page.Name != "projects/graphthulhu" {
			t.Errorf("alias resolved to %q, want projects/graphthulhu", page.Name)
		}
	})

	t.Run("nonexistent", func(t *testing.T) {
		page, err := c.GetPage(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("GetPage: %v", err)
		}
		if page != nil {
			t.Error("expected nil for nonexistent page")
		}
	})
}

func TestGetPageBlocksTree(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	blocks, err := c.GetPageBlocksTree(ctx, "projects/graphthulhu")
	if err != nil {
		t.Fatalf("GetPageBlocksTree: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("expected blocks")
	}

	// First block should be the H1 "# graphthulhu".
	if !strings.Contains(blocks[0].Content, "graphthulhu") {
		t.Errorf("first block = %q, expected to contain graphthulhu", blocks[0].Content)
	}

	// Should have children (## Architecture, ## Features).
	if len(blocks[0].Children) < 2 {
		t.Errorf("expected at least 2 children, got %d", len(blocks[0].Children))
	}

	// All blocks should have UUIDs.
	for _, b := range blocks {
		if b.UUID == "" {
			t.Error("block has empty UUID")
		}
	}
}

func TestGetBlock(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	// Get a page's blocks first to find a UUID.
	blocks, _ := c.GetPageBlocksTree(ctx, "projects/graphthulhu")
	if len(blocks) == 0 {
		t.Fatal("no blocks to test")
	}

	uuid := blocks[0].UUID
	block, err := c.GetBlock(ctx, uuid)
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
	if block == nil {
		t.Fatal("block not found")
	}
	if block.UUID != uuid {
		t.Errorf("UUID = %q, want %q", block.UUID, uuid)
	}
	if block.Page == nil || block.Page.Name == "" {
		t.Error("expected page reference on block")
	}
}

func TestBacklinks(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	// projects/graphthulhu is linked from: projects/openchaos, people/Hanna, daily notes, index.
	raw, err := c.GetPageLinkedReferences(ctx, "projects/graphthulhu")
	if err != nil {
		t.Fatalf("GetPageLinkedReferences: %v", err)
	}

	var refs []any
	if err := json.Unmarshal(raw, &refs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(refs) < 2 {
		t.Errorf("expected at least 2 pages linking to graphthulhu, got %d", len(refs))
	}
}

func TestJournalPages(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	pages, _ := c.GetAllPages(ctx)
	journalCount := 0
	for _, p := range pages {
		if p.Journal {
			journalCount++
		}
	}
	if journalCount != 2 {
		t.Errorf("expected 2 journal pages, got %d", journalCount)
	}
}

func TestPing(t *testing.T) {
	c := testVault(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}

	bad := New("/nonexistent/path")
	if err := bad.Ping(context.Background()); err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestReadOnlyErrors(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	if _, err := c.CreatePage(ctx, "test", nil, nil); err != ErrReadOnly {
		t.Errorf("CreatePage: %v, want ErrReadOnly", err)
	}
	if _, err := c.AppendBlockInPage(ctx, "test", "content"); err != ErrReadOnly {
		t.Errorf("AppendBlockInPage: %v, want ErrReadOnly", err)
	}
	if err := c.UpdateBlock(ctx, "uuid", "content"); err != ErrReadOnly {
		t.Errorf("UpdateBlock: %v, want ErrReadOnly", err)
	}
	if err := c.RemoveBlock(ctx, "uuid"); err != ErrReadOnly {
		t.Errorf("RemoveBlock: %v, want ErrReadOnly", err)
	}
}

func TestDatascriptQueryNotSupported(t *testing.T) {
	c := testVault(t)
	_, err := c.DatascriptQuery(context.Background(), "[:find ...]")
	if err != ErrNotSupported {
		t.Errorf("DatascriptQuery: %v, want ErrNotSupported", err)
	}
}

func TestFindBlocksByTag(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	results, err := c.FindBlocksByTag(ctx, "decision", false)
	if err != nil {
		t.Fatalf("FindBlocksByTag: %v", err)
	}

	// The daily notes/2026-01-31 page has a block with #decision.
	found := false
	for _, r := range results {
		if strings.Contains(r.Page, "2026-01-31") {
			found = true
		}
	}
	if !found {
		t.Error("expected to find #decision in daily notes/2026-01-31")
	}
}

func TestFindByProperty(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	results, err := c.FindByProperty(ctx, "type", "project", "eq")
	if err != nil {
		t.Fatalf("FindByProperty: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 project pages, got %d", len(results))
		for _, r := range results {
			t.Logf("  %s", r.Name)
		}
	}
}

func TestSearchJournals(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	results, err := c.SearchJournals(ctx, "backend", "", "")
	if err != nil {
		t.Fatalf("SearchJournals: %v", err)
	}

	found := false
	for _, r := range results {
		if strings.Contains(r.Date, "2026-02-01") {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 'backend' in 2026-02-01 journal")
	}
}

func TestGraphBuildFromVault(t *testing.T) {
	c := testVault(t)
	ctx := context.Background()

	// Test that graph.Build works with the vault client.
	// This is the key integration test: the vault client satisfies the
	// interface that graph.Build needs.
	pages, err := c.GetAllPages(ctx)
	if err != nil {
		t.Fatalf("GetAllPages: %v", err)
	}

	// Verify we can get blocks for every page (graph.Build does this).
	for _, p := range pages {
		blocks, err := c.GetPageBlocksTree(ctx, p.Name)
		if err != nil {
			t.Errorf("GetPageBlocksTree(%s): %v", p.Name, err)
		}
		// blocks can be nil for empty pages, that's fine.
		_ = blocks
	}
}

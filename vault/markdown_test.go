package vault

import (
	"testing"

	"github.com/skridlevsky/graphthulhu/types"
)

func TestHeadingLevel(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"# Heading 1", 1},
		{"## Heading 2", 2},
		{"### Heading 3", 3},
		{"#### Heading 4", 4},
		{"##### Heading 5", 5},
		{"###### Heading 6", 6},
		{"####### Seven hashes", 0},
		{"Not a heading", 0},
		{"#NoSpace", 0},
		{"##NoSpace", 0},
		{"  ## Indented heading", 2},
		{"", 0},
		{"# ", 1},
		{"##", 2},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := headingLevel(tt.line)
			if got != tt.want {
				t.Errorf("headingLevel(%q) = %d, want %d", tt.line, got, tt.want)
			}
		})
	}
}

func TestDeterministicUUID(t *testing.T) {
	// Same inputs produce same output.
	a := deterministicUUID("test.md", 5)
	b := deterministicUUID("test.md", 5)
	if a != b {
		t.Errorf("deterministic UUID not stable: %q != %q", a, b)
	}

	// Different inputs produce different output.
	c := deterministicUUID("test.md", 6)
	if a == c {
		t.Errorf("different inputs produced same UUID: %q", a)
	}

	// UUID format: 8-4-4-4-12.
	if len(a) != 36 {
		t.Errorf("UUID length = %d, want 36", len(a))
	}
	if a[8] != '-' || a[13] != '-' || a[18] != '-' || a[23] != '-' {
		t.Errorf("UUID format wrong: %q", a)
	}
}

func TestParseMarkdownBlocks(t *testing.T) {
	t.Run("empty body", func(t *testing.T) {
		blocks := parseMarkdownBlocks("test.md", "")
		if len(blocks) != 0 {
			t.Errorf("expected 0 blocks, got %d", len(blocks))
		}
	})

	t.Run("whitespace only", func(t *testing.T) {
		blocks := parseMarkdownBlocks("test.md", "   \n\n  ")
		if len(blocks) != 0 {
			t.Errorf("expected 0 blocks, got %d", len(blocks))
		}
	})

	t.Run("pre-heading content only", func(t *testing.T) {
		body := "Some intro text\nwith multiple lines"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
		if blocks[0].Content != body {
			t.Errorf("content = %q, want %q", blocks[0].Content, body)
		}
		if blocks[0].UUID == "" {
			t.Error("UUID should not be empty")
		}
	})

	t.Run("single heading with content", func(t *testing.T) {
		body := "# Title\nSome content here"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
		if blocks[0].Content != body {
			t.Errorf("content = %q, want %q", blocks[0].Content, body)
		}
	})

	t.Run("two headings at same level", func(t *testing.T) {
		body := "## Section A\nContent A\n## Section B\nContent B"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(blocks))
		}
		if blocks[0].Content != "## Section A\nContent A" {
			t.Errorf("block[0] content = %q", blocks[0].Content)
		}
		if blocks[1].Content != "## Section B\nContent B" {
			t.Errorf("block[1] content = %q", blocks[1].Content)
		}
	})

	t.Run("nested headings", func(t *testing.T) {
		body := "# Parent\nIntro\n## Child 1\nContent 1\n## Child 2\nContent 2"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 root block, got %d", len(blocks))
		}
		parent := blocks[0]
		if parent.Content != "# Parent\nIntro" {
			t.Errorf("parent content = %q", parent.Content)
		}
		if len(parent.Children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(parent.Children))
		}
		if parent.Children[0].Content != "## Child 1\nContent 1" {
			t.Errorf("child[0] content = %q", parent.Children[0].Content)
		}
		if parent.Children[1].Content != "## Child 2\nContent 2" {
			t.Errorf("child[1] content = %q", parent.Children[1].Content)
		}
	})

	t.Run("deeply nested headings", func(t *testing.T) {
		body := "# H1\n## H2\n### H3\nDeep content"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 root, got %d", len(blocks))
		}
		if len(blocks[0].Children) != 1 {
			t.Fatalf("H1 should have 1 child (H2), got %d", len(blocks[0].Children))
		}
		h2 := blocks[0].Children[0]
		if len(h2.Children) != 1 {
			t.Fatalf("H2 should have 1 child (H3), got %d", len(h2.Children))
		}
		h3 := h2.Children[0]
		if h3.Content != "### H3\nDeep content" {
			t.Errorf("H3 content = %q", h3.Content)
		}
	})

	t.Run("pre-heading then heading", func(t *testing.T) {
		body := "Preamble text\n# Section 1\nContent"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(blocks))
		}
		if blocks[0].Content != "Preamble text" {
			t.Errorf("preamble = %q", blocks[0].Content)
		}
		if blocks[1].Content != "# Section 1\nContent" {
			t.Errorf("section = %q", blocks[1].Content)
		}
	})

	t.Run("heading level jump H1 then H3", func(t *testing.T) {
		body := "# Top\n### Skipped H2\nContent"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 root, got %d", len(blocks))
		}
		// H3 should be child of H1 (skipping H2 is fine).
		if len(blocks[0].Children) != 1 {
			t.Fatalf("H1 should have 1 child, got %d", len(blocks[0].Children))
		}
		if blocks[0].Children[0].Content != "### Skipped H2\nContent" {
			t.Errorf("child content = %q", blocks[0].Children[0].Content)
		}
	})

	t.Run("heading then back to higher level", func(t *testing.T) {
		body := "# Section 1\n## Sub\nNested\n# Section 2\nTop again"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 2 {
			t.Fatalf("expected 2 roots, got %d", len(blocks))
		}
		if len(blocks[0].Children) != 1 {
			t.Fatalf("section 1 should have 1 child, got %d", len(blocks[0].Children))
		}
		if blocks[1].Content != "# Section 2\nTop again" {
			t.Errorf("section 2 = %q", blocks[1].Content)
		}
	})

	t.Run("blocks have unique UUIDs", func(t *testing.T) {
		body := "# A\n## B\n## C\n# D"
		blocks := parseMarkdownBlocks("test.md", body)
		uuids := collectUUIDs(blocks)
		seen := make(map[string]bool)
		for _, uuid := range uuids {
			if seen[uuid] {
				t.Errorf("duplicate UUID: %s", uuid)
			}
			seen[uuid] = true
		}
	})

	t.Run("content with wikilinks preserved", func(t *testing.T) {
		body := "# Project\nSee [[graphthulhu]] and [[OpenChaos]] for details"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
		want := "# Project\nSee [[graphthulhu]] and [[OpenChaos]] for details"
		if blocks[0].Content != want {
			t.Errorf("content = %q, want %q", blocks[0].Content, want)
		}
	})

	t.Run("trailing newlines stripped", func(t *testing.T) {
		body := "# Section\nContent\n\n\n"
		blocks := parseMarkdownBlocks("test.md", body)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
		if blocks[0].Content != "# Section\nContent" {
			t.Errorf("content = %q", blocks[0].Content)
		}
	})
}

func collectUUIDs(blocks []types.BlockEntity) []string {
	var uuids []string
	for _, b := range blocks {
		uuids = append(uuids, b.UUID)
		uuids = append(uuids, collectUUIDs(b.Children)...)
	}
	return uuids
}

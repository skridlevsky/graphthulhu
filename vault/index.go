package vault

import (
	"github.com/skridlevsky/graphthulhu/parser"
	"github.com/skridlevsky/graphthulhu/types"
)

// backlink records a reference from one page to another.
type backlink struct {
	fromPage string // source page name (lowercase)
	block    types.BlockSummary
}

// buildBacklinks scans all pages' block trees and builds a reverse link index.
// Returns: map[lowercase target page name] → []backlink
func buildBacklinks(pages map[string]*cachedPage) map[string][]backlink {
	index := make(map[string][]backlink)

	for _, page := range pages {
		scanBlocksForLinks(page.lowerName, page.blocks, index)
	}

	return index
}

// scanBlocksForLinks recursively extracts [[links]] from blocks and records backlinks.
func scanBlocksForLinks(sourcePage string, blocks []types.BlockEntity, index map[string][]backlink) {
	for _, b := range blocks {
		parsed := parser.Parse(b.Content)
		for _, link := range parsed.Links {
			targetKey := toLower(link)
			index[targetKey] = append(index[targetKey], backlink{
				fromPage: sourcePage,
				block: types.BlockSummary{
					UUID:    b.UUID,
					Content: b.Content,
				},
			})
		}
		if len(b.Children) > 0 {
			scanBlocksForLinks(sourcePage, b.Children, index)
		}
	}
}

// toLower is a helper that matches the vault's key normalization.
func toLower(s string) string {
	// strings.ToLower is imported in vault.go; avoid duplicate import.
	// Use a simple ASCII-aware lowering for page names.
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

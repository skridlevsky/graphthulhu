package vault

import (
	"strings"

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

// toLower normalizes page names to lowercase using full Unicode support.
func toLower(s string) string {
	return strings.ToLower(s)
}

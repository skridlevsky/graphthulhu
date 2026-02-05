package vault

import (
	"sort"
	"strings"
	"sync"

	"github.com/skridlevsky/graphthulhu/parser"
	"github.com/skridlevsky/graphthulhu/types"
)

// SearchIndex is a simple inverted index for full-text search.
// Maps lowercase terms to the blocks containing them.
type SearchIndex struct {
	mu      sync.RWMutex
	// term → list of block references
	index   map[string][]blockRef
	// page lowercase → set of terms (for efficient removal on reindex)
	pageTerms map[string]map[string]bool
}

// blockRef identifies a block within a page.
type blockRef struct {
	pageName string // original case page name
	uuid     string
	content  string
}

// NewSearchIndex creates an empty search index.
func NewSearchIndex() *SearchIndex {
	return &SearchIndex{
		index:     make(map[string][]blockRef),
		pageTerms: make(map[string]map[string]bool),
	}
}

// BuildFrom indexes all pages in the vault.
func (si *SearchIndex) BuildFrom(pages map[string]*cachedPage) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.index = make(map[string][]blockRef)
	si.pageTerms = make(map[string]map[string]bool)

	seen := make(map[string]bool)
	for _, page := range pages {
		if seen[page.lowerName] {
			continue
		}
		seen[page.lowerName] = true
		si.indexPageLocked(page)
	}
}

// ReindexPage removes and re-adds a single page to the index.
func (si *SearchIndex) ReindexPage(page *cachedPage) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.removePageLocked(page.lowerName)
	si.indexPageLocked(page)
}

// RemovePage removes a page from the index.
func (si *SearchIndex) RemovePage(lowerName string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.removePageLocked(lowerName)
}

// Search finds blocks matching all terms in the query (AND semantics).
// Returns results sorted by relevance (number of term hits).
func (si *SearchIndex) Search(query string, limit int) []SearchResult {
	si.mu.RLock()
	defer si.mu.RUnlock()

	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}

	if limit <= 0 {
		limit = 20
	}

	// Find blocks that contain ALL terms.
	// Start with the rarest term for efficiency.
	var rarest string
	rarestCount := int(^uint(0) >> 1) // max int
	for _, t := range terms {
		if count := len(si.index[t]); count < rarestCount {
			rarestCount = count
			rarest = t
		}
	}

	// Candidates from rarest term.
	candidates := si.index[rarest]
	if len(candidates) == 0 {
		return nil
	}

	// Build a set of UUIDs for each other term.
	otherTermSets := make([]map[string]bool, 0, len(terms)-1)
	for _, t := range terms {
		if t == rarest {
			continue
		}
		set := make(map[string]bool)
		for _, ref := range si.index[t] {
			set[ref.uuid] = true
		}
		otherTermSets = append(otherTermSets, set)
	}

	// Filter candidates that appear in ALL other term sets.
	var results []SearchResult
	for _, ref := range candidates {
		inAll := true
		for _, set := range otherTermSets {
			if !set[ref.uuid] {
				inAll = false
				break
			}
		}
		if !inAll {
			continue
		}

		results = append(results, SearchResult{
			PageName: ref.pageName,
			UUID:     ref.uuid,
			Content:  ref.content,
		})

		if len(results) >= limit {
			break
		}
	}

	return results
}

// SearchResult is a block that matched a search query.
type SearchResult struct {
	PageName string `json:"page"`
	UUID     string `json:"uuid"`
	Content  string `json:"content"`
}

// --- Internal ---

func (si *SearchIndex) indexPageLocked(page *cachedPage) {
	terms := make(map[string]bool)
	si.indexBlocksLocked(page.blocks, page.entity.OriginalName, terms)
	si.pageTerms[page.lowerName] = terms
}

func (si *SearchIndex) indexBlocksLocked(blocks []types.BlockEntity, pageName string, terms map[string]bool) {
	for _, b := range blocks {
		blockTerms := tokenize(b.Content)

		// Also index parsed links and tags as terms.
		parsed := parser.Parse(b.Content)
		for _, link := range parsed.Links {
			for _, t := range tokenize(link) {
				blockTerms = append(blockTerms, t)
			}
		}
		for _, tag := range parsed.Tags {
			for _, t := range tokenize(tag) {
				blockTerms = append(blockTerms, t)
			}
		}

		ref := blockRef{
			pageName: pageName,
			uuid:     b.UUID,
			content:  b.Content,
		}

		seen := make(map[string]bool)
		for _, term := range blockTerms {
			if seen[term] {
				continue
			}
			seen[term] = true
			terms[term] = true
			si.index[term] = append(si.index[term], ref)
		}

		if len(b.Children) > 0 {
			si.indexBlocksLocked(b.Children, pageName, terms)
		}
	}
}

func (si *SearchIndex) removePageLocked(lowerName string) {
	terms, ok := si.pageTerms[lowerName]
	if !ok {
		return
	}

	for term := range terms {
		refs := si.index[term]
		filtered := refs[:0]
		for _, ref := range refs {
			if strings.ToLower(ref.pageName) != lowerName {
				filtered = append(filtered, ref)
			}
		}
		if len(filtered) == 0 {
			delete(si.index, term)
		} else {
			si.index[term] = filtered
		}
	}

	delete(si.pageTerms, lowerName)
}

// tokenize splits text into lowercase terms for indexing.
// Strips common markdown syntax and splits on whitespace + punctuation.
func tokenize(text string) []string {
	// Remove markdown syntax.
	text = strings.ReplaceAll(text, "[[", " ")
	text = strings.ReplaceAll(text, "]]", " ")
	text = strings.ReplaceAll(text, "((", " ")
	text = strings.ReplaceAll(text, "))", " ")
	text = strings.ReplaceAll(text, "#", " ")
	text = strings.ReplaceAll(text, "::", " ")
	text = strings.ReplaceAll(text, "**", " ")
	text = strings.ReplaceAll(text, "__", " ")
	text = strings.ReplaceAll(text, "`", " ")

	text = strings.ToLower(text)

	// Split on non-alphanumeric (Unicode-aware).
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !isWordChar(r)
	})

	// Filter very short terms and deduplicate.
	var terms []string
	for _, w := range words {
		if len(w) >= 2 { // skip single chars
			terms = append(terms, w)
		}
	}

	return terms
}

// isWordChar returns true for letters and digits (Unicode-aware).
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r > 127
}

// SortByRelevance sorts search results by term frequency.
// Not currently used but available for future ranking improvements.
func SortByRelevance(results []SearchResult, query string) {
	terms := tokenize(query)
	sort.Slice(results, func(i, j int) bool {
		iScore := countTermHits(results[i].Content, terms)
		jScore := countTermHits(results[j].Content, terms)
		return iScore > jScore
	})
}

func countTermHits(content string, terms []string) int {
	lower := strings.ToLower(content)
	count := 0
	for _, t := range terms {
		count += strings.Count(lower, t)
	}
	return count
}

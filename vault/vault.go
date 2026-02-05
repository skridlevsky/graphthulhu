package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/skridlevsky/graphthulhu/backend"
	"github.com/skridlevsky/graphthulhu/parser"
	"github.com/skridlevsky/graphthulhu/types"
)

// ErrReadOnly is returned for write operations on the read-only Obsidian backend.
var ErrReadOnly = fmt.Errorf("obsidian backend is read-only")

// ErrNotSupported is returned for Logseq-specific operations (DataScript queries).
var ErrNotSupported = fmt.Errorf("operation not supported by obsidian backend")

// cachedPage holds a parsed markdown file in memory.
type cachedPage struct {
	entity    types.PageEntity
	lowerName string
	filePath  string
	blocks    []types.BlockEntity
}

// Client implements backend.Backend for an Obsidian vault on disk.
// It reads all .md files on initialization and serves queries from memory.
type Client struct {
	vaultPath   string
	dailyFolder string // e.g. "daily notes"
	pages       map[string]*cachedPage // lowercase name → page
	backlinks   map[string][]backlink  // lowercase target → backlinks
	blockIndex  map[string]*blockLookup // uuid → block + page
}

// blockLookup stores a block and its page for UUID-based retrieval.
type blockLookup struct {
	block *types.BlockEntity
	page  string
}

// Option configures a vault Client.
type Option func(*Client)

// WithDailyFolder sets the daily notes subfolder (default: "daily notes").
func WithDailyFolder(folder string) Option {
	return func(c *Client) { c.dailyFolder = folder }
}

// New creates a new Obsidian vault client. Call Load() to index the vault.
func New(vaultPath string, opts ...Option) *Client {
	c := &Client{
		vaultPath:   vaultPath,
		dailyFolder: "daily notes",
		pages:       make(map[string]*cachedPage),
		backlinks:   make(map[string][]backlink),
		blockIndex:  make(map[string]*blockLookup),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Load reads all .md files in the vault and builds the in-memory index.
func (c *Client) Load() error {
	return filepath.Walk(c.vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip hidden directories (.obsidian, .git, etc).
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		relPath, _ := filepath.Rel(c.vaultPath, path)
		c.indexFile(relPath, string(content), info)
		return nil
	})
}

// indexFile parses a single markdown file and adds it to the index.
func (c *Client) indexFile(relPath, content string, info os.FileInfo) {
	// Page name: strip .md extension, use relative path with / separators.
	name := strings.TrimSuffix(filepath.ToSlash(relPath), ".md")
	lowerName := strings.ToLower(name)

	// Parse frontmatter.
	props, body := parseFrontmatter(content)

	// Determine if this is a journal page (lives in daily notes folder).
	isJournal := false
	if c.dailyFolder != "" {
		prefix := strings.ToLower(c.dailyFolder) + "/"
		isJournal = strings.HasPrefix(lowerName, prefix)
	}

	// Build page entity.
	entity := types.PageEntity{
		Name:         name,
		OriginalName: name,
		Properties:   props,
		Journal:      isJournal,
		CreatedAt:    info.ModTime().UnixMilli(),
		UpdatedAt:    info.ModTime().UnixMilli(),
	}

	// Handle aliases from frontmatter.
	if props != nil {
		if aliases, ok := props["aliases"]; ok {
			if aliasList, ok := aliases.([]any); ok {
				for _, a := range aliasList {
					if s, ok := a.(string); ok {
						aliasLower := strings.ToLower(s)
						// Register alias as an additional lookup key.
						c.pages[aliasLower] = &cachedPage{
							entity:    entity,
							lowerName: lowerName,
							filePath:  relPath,
						}
					}
				}
			}
		}
	}

	// Parse blocks.
	blocks := parseMarkdownBlocks(relPath, body)

	page := &cachedPage{
		entity:    entity,
		lowerName: lowerName,
		filePath:  relPath,
		blocks:    blocks,
	}
	c.pages[lowerName] = page

	// Index blocks by UUID.
	c.indexBlocks(blocks, name)
}

// indexBlocks recursively adds blocks to the UUID index.
func (c *Client) indexBlocks(blocks []types.BlockEntity, pageName string) {
	for i := range blocks {
		c.blockIndex[blocks[i].UUID] = &blockLookup{
			block: &blocks[i],
			page:  pageName,
		}
		if len(blocks[i].Children) > 0 {
			c.indexBlocks(blocks[i].Children, pageName)
		}
	}
}

// BuildBacklinks must be called after Load() to build the reverse link index.
func (c *Client) BuildBacklinks() {
	c.backlinks = buildBacklinks(c.pages)
}

// --- backend.Backend implementation ---

func (c *Client) Ping(_ context.Context) error {
	info, err := os.Stat(c.vaultPath)
	if err != nil {
		return fmt.Errorf("vault path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vault path is not a directory: %s", c.vaultPath)
	}
	return nil
}

func (c *Client) GetAllPages(_ context.Context) ([]types.PageEntity, error) {
	seen := make(map[string]bool)
	var pages []types.PageEntity
	for _, page := range c.pages {
		if seen[page.lowerName] {
			continue // skip alias duplicates
		}
		seen[page.lowerName] = true
		pages = append(pages, page.entity)
	}
	return pages, nil
}

func (c *Client) GetPage(_ context.Context, nameOrID any) (*types.PageEntity, error) {
	name := fmt.Sprint(nameOrID)
	page, ok := c.pages[strings.ToLower(name)]
	if !ok {
		return nil, nil
	}
	return &page.entity, nil
}

func (c *Client) GetPageBlocksTree(_ context.Context, nameOrID any) ([]types.BlockEntity, error) {
	name := fmt.Sprint(nameOrID)
	page, ok := c.pages[strings.ToLower(name)]
	if !ok {
		return nil, nil
	}
	return page.blocks, nil
}

func (c *Client) GetBlock(_ context.Context, uuid string, opts ...map[string]any) (*types.BlockEntity, error) {
	lookup, ok := c.blockIndex[uuid]
	if !ok {
		return nil, nil
	}

	// Build a copy with page reference.
	block := *lookup.block
	block.Page = &types.PageRef{Name: lookup.page}
	return &block, nil
}

func (c *Client) GetPageLinkedReferences(_ context.Context, nameOrID any) (json.RawMessage, error) {
	name := fmt.Sprint(nameOrID)
	key := strings.ToLower(name)

	links, ok := c.backlinks[key]
	if !ok || len(links) == 0 {
		return json.Marshal([]any{})
	}

	// Group by source page.
	grouped := make(map[string][]types.BlockSummary)
	for _, bl := range links {
		grouped[bl.fromPage] = append(grouped[bl.fromPage], bl.block)
	}

	// Format as array of [pageName, blocks] pairs (Logseq format).
	var result []any
	for page, blocks := range grouped {
		result = append(result, []any{page, blocks})
	}

	return json.Marshal(result)
}

func (c *Client) DatascriptQuery(_ context.Context, query string, inputs ...any) (json.RawMessage, error) {
	return nil, ErrNotSupported
}

// --- Write operations (read-only for v0.1) ---

func (c *Client) CreatePage(_ context.Context, name string, properties map[string]any, opts map[string]any) (*types.PageEntity, error) {
	return nil, ErrReadOnly
}

func (c *Client) AppendBlockInPage(_ context.Context, page string, content string) (*types.BlockEntity, error) {
	return nil, ErrReadOnly
}

func (c *Client) PrependBlockInPage(_ context.Context, page string, content string) (*types.BlockEntity, error) {
	return nil, ErrReadOnly
}

func (c *Client) InsertBlock(_ context.Context, srcBlock any, content string, opts map[string]any) (*types.BlockEntity, error) {
	return nil, ErrReadOnly
}

func (c *Client) UpdateBlock(_ context.Context, uuid string, content string, opts ...map[string]any) error {
	return ErrReadOnly
}

func (c *Client) RemoveBlock(_ context.Context, uuid string) error {
	return ErrReadOnly
}

func (c *Client) MoveBlock(_ context.Context, uuid string, targetUUID string, opts map[string]any) error {
	return ErrReadOnly
}

// --- Optional search interfaces ---

// FindBlocksByTag scans all pages for blocks containing the given #tag.
// Implements backend.TagSearcher.
func (c *Client) FindBlocksByTag(_ context.Context, tag string, includeChildren bool) ([]backend.TagResult, error) {
	tagLower := strings.ToLower(tag)
	var results []backend.TagResult

	seen := make(map[string]bool)
	for _, page := range c.pages {
		if seen[page.lowerName] {
			continue
		}
		seen[page.lowerName] = true

		var matches []types.BlockEntity
		findTagInBlocks(page.blocks, tagLower, &matches)
		if len(matches) > 0 {
			results = append(results, backend.TagResult{
				Page:   page.entity.Name,
				Blocks: matches,
			})
		}
	}

	return results, nil
}

// findTagInBlocks recursively searches blocks for a tag.
func findTagInBlocks(blocks []types.BlockEntity, tagLower string, matches *[]types.BlockEntity) {
	for _, b := range blocks {
		parsed := parser.Parse(b.Content)
		for _, t := range parsed.Tags {
			if strings.ToLower(t) == tagLower {
				*matches = append(*matches, b)
				break
			}
		}
		if len(b.Children) > 0 {
			findTagInBlocks(b.Children, tagLower, matches)
		}
	}
}

// FindByProperty scans all pages for matching frontmatter properties.
// Implements backend.PropertySearcher.
func (c *Client) FindByProperty(_ context.Context, key, value, operator string) ([]backend.PropertyResult, error) {
	var results []backend.PropertyResult

	seen := make(map[string]bool)
	for _, page := range c.pages {
		if seen[page.lowerName] {
			continue
		}
		seen[page.lowerName] = true

		if page.entity.Properties == nil {
			continue
		}

		propVal, ok := page.entity.Properties[key]
		if !ok {
			continue
		}

		if value == "" {
			// Just checking if property exists.
			results = append(results, backend.PropertyResult{
				Type:       "page",
				Name:       page.entity.Name,
				Properties: page.entity.Properties,
			})
			continue
		}

		propStr := fmt.Sprint(propVal)
		match := false
		switch operator {
		case "eq", "":
			match = strings.EqualFold(propStr, value)
		case "contains":
			match = strings.Contains(strings.ToLower(propStr), strings.ToLower(value))
		case "gt":
			match = propStr > value
		case "lt":
			match = propStr < value
		}

		if match {
			results = append(results, backend.PropertyResult{
				Type:       "page",
				Name:       page.entity.Name,
				Properties: page.entity.Properties,
			})
		}
	}

	return results, nil
}

// SearchJournals scans daily notes for matching content.
// Implements backend.JournalSearcher.
func (c *Client) SearchJournals(_ context.Context, query, from, to string) ([]backend.JournalResult, error) {
	queryLower := strings.ToLower(query)
	var results []backend.JournalResult

	seen := make(map[string]bool)
	for _, page := range c.pages {
		if seen[page.lowerName] {
			continue
		}
		seen[page.lowerName] = true

		if !page.entity.Journal {
			continue
		}

		// Extract date from page name (last path segment).
		parts := strings.Split(page.entity.Name, "/")
		date := parts[len(parts)-1]

		// Filter by date range.
		if from != "" && date < from {
			continue
		}
		if to != "" && date > to {
			continue
		}

		// Search blocks for query.
		var matches []types.BlockEntity
		searchBlocksForText(page.blocks, queryLower, &matches)
		if len(matches) > 0 {
			results = append(results, backend.JournalResult{
				Date:   date,
				Page:   page.entity.Name,
				Blocks: matches,
			})
		}
	}

	return results, nil
}

// searchBlocksForText recursively searches blocks for text content.
func searchBlocksForText(blocks []types.BlockEntity, queryLower string, matches *[]types.BlockEntity) {
	for _, b := range blocks {
		if strings.Contains(strings.ToLower(b.Content), queryLower) {
			*matches = append(*matches, b)
		}
		if len(b.Children) > 0 {
			searchBlocksForText(b.Children, queryLower, matches)
		}
	}
}

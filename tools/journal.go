package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/backend"
	"github.com/skridlevsky/graphthulhu/parser"
	"github.com/skridlevsky/graphthulhu/types"
)

// Journal implements journal MCP tools.
type Journal struct {
	client backend.Backend
}

// NewJournal creates a new Journal tool handler.
func NewJournal(c backend.Backend) *Journal {
	return &Journal{client: c}
}

// JournalRange returns journal entries across a date range.
func (j *Journal) JournalRange(ctx context.Context, req *mcp.CallToolRequest, input types.JournalRangeInput) (*mcp.CallToolResult, any, error) {
	from, err := time.Parse("2006-01-02", input.From)
	if err != nil {
		return errorResult(fmt.Sprintf("invalid from date '%s': use YYYY-MM-DD format", input.From)), nil, nil
	}

	to, err := time.Parse("2006-01-02", input.To)
	if err != nil {
		return errorResult(fmt.Sprintf("invalid to date '%s': use YYYY-MM-DD format", input.To)), nil, nil
	}

	if to.Before(from) {
		return errorResult("'to' date must be after 'from' date"), nil, nil
	}

	var journals []map[string]any

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		// Try common Logseq journal page name formats
		pageNames := []string{
			d.Format("Jan 2nd, 2006"),
			d.Format("2006-01-02"),
			d.Format("January 2, 2006"),
		}

		var foundPage *types.PageEntity
		var foundName string

		for _, name := range pageNames {
			page, err := j.client.GetPage(ctx, name)
			if err == nil && page != nil {
				foundPage = page
				foundName = name
				break
			}
		}

		if foundPage == nil {
			continue
		}

		entry := map[string]any{
			"date":     d.Format("2006-01-02"),
			"pageName": foundName,
		}

		if input.IncludeBlocks {
			blocks, err := j.client.GetPageBlocksTree(ctx, foundName)
			if err == nil {
				enriched := enrichBlockTree(blocks, -1, 0)
				entry["blocks"] = enriched
				entry["blockCount"] = countBlocks(enriched)
			}
		}

		journals = append(journals, entry)
	}

	res, err := jsonTextResult(map[string]any{
		"from":         input.From,
		"to":           input.To,
		"entriesFound": len(journals),
		"journals":     journals,
	})
	return res, nil, err
}

// JournalSearch searches within journal entries.
func (j *Journal) JournalSearch(ctx context.Context, req *mcp.CallToolRequest, input types.JournalSearchInput) (*mcp.CallToolResult, any, error) {
	// Use native journal search if the backend supports it (e.g. Obsidian).
	if searcher, ok := j.client.(backend.JournalSearcher); ok {
		results, err := searcher.SearchJournals(ctx, input.Query, input.From, input.To)
		if err != nil {
			return errorResult(fmt.Sprintf("journal search failed: %v", err)), nil, nil
		}

		var matches []map[string]any
		for _, r := range results {
			for _, block := range r.Blocks {
				parsed := parser.Parse(block.Content)
				matches = append(matches, map[string]any{
					"content": block.Content,
					"parsed":  parsed,
					"page":    r.Page,
					"date":    r.Date,
				})
			}
		}

		res, err := jsonTextResult(map[string]any{
			"query":   input.Query,
			"count":   len(matches),
			"results": matches,
		})
		return res, nil, err
	}

	// Fall back to DataScript (Logseq).
	query := `[:find (pull ?b [:block/uuid :block/content {:block/page [:block/name :block/original-name :block/journal-day]}])
		:where
		[?b :block/content ?content]
		[?b :block/page ?p]
		[?p :block/journal? true]]`

	raw, err := j.client.DatascriptQuery(ctx, query)
	if err != nil {
		return errorResult(fmt.Sprintf("journal search failed: %v", err)), nil, nil
	}

	var allResults [][]map[string]any
	if err := json.Unmarshal(raw, &allResults); err != nil {
		return errorResult(fmt.Sprintf("failed to parse results: %v", err)), nil, nil
	}

	searchLower := strings.ToLower(input.Query)
	var matches []map[string]any

	for _, r := range allResults {
		if len(r) == 0 {
			continue
		}
		block := r[0]
		content, _ := block["content"].(string)
		if !strings.Contains(strings.ToLower(content), searchLower) {
			continue
		}

		parsed := parser.Parse(content)
		match := map[string]any{
			"content": content,
			"parsed":  parsed,
		}

		if page, ok := block["page"].(map[string]any); ok {
			match["page"] = page["original-name"]
			if match["page"] == nil {
				match["page"] = page["name"]
			}
		}

		matches = append(matches, match)
	}

	res, err := jsonTextResult(map[string]any{
		"query":   input.Query,
		"count":   len(matches),
		"results": matches,
	})
	return res, nil, err
}

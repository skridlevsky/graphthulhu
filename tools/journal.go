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
		// Try common Logseq journal page name formats.
		pageNames := journalPageNames(d)

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

// journalPageNames returns the candidate Logseq journal page names for a date,
// ordered most-common-first. Logseq lets users pick a date format, so we try
// the defaults first, then "day-first" variants that several locales prefer
// (e.g. "16th Apr 2026"), then formats without ordinals.
func journalPageNames(d time.Time) []string {
	day := d.Day()
	suffix := ordinalSuffix(day)
	year := d.Year()
	monShort := d.Format("Jan")
	monLong := d.Format("January")

	return []string{
		// Default Logseq formats: "Apr 16th, 2026", "April 16th, 2026"
		fmt.Sprintf("%s %d%s, %d", monShort, day, suffix, year),
		fmt.Sprintf("%s %d%s, %d", monLong, day, suffix, year),
		// ISO and "April 16, 2026"
		d.Format("2006-01-02"),
		d.Format("January 2, 2006"),
		// Day-first with ordinal: "16th Apr 2026", "16th April 2026"
		fmt.Sprintf("%d%s %s %d", day, suffix, monShort, year),
		fmt.Sprintf("%d%s %s %d", day, suffix, monLong, year),
		// Day-first without ordinal: "16 Apr 2026", "16 April 2026"
		fmt.Sprintf("%d %s %d", day, monShort, year),
		fmt.Sprintf("%d %s %d", day, monLong, year),
	}
}

func ordinalSuffix(day int) string {
	if day%100 >= 11 && day%100 <= 13 {
		return "th"
	}

	switch day % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
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

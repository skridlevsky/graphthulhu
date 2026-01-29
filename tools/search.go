package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/client"
	"github.com/skridlevsky/graphthulhu/parser"
	"github.com/skridlevsky/graphthulhu/types"
)

// Search implements search and query MCP tools.
type Search struct {
	client *client.Client
}

// NewSearch creates a new Search tool handler.
func NewSearch(c *client.Client) *Search {
	return &Search{client: c}
}

// Search performs full-text search across all blocks with context.
func (s *Search) Search(ctx context.Context, req *mcp.CallToolRequest, input types.SearchInput) (*mcp.CallToolResult, any, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	query := strings.ToLower(input.Query)

	pages, err := s.client.GetAllPages(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list pages: %v", err)), nil, nil
	}

	var results []map[string]any
	for _, page := range pages {
		if len(results) >= limit {
			break
		}
		if page.Name == "" {
			continue
		}

		blocks, err := s.client.GetPageBlocksTree(ctx, page.Name)
		if err != nil {
			continue
		}

		matches := searchBlockTree(blocks, query, page.OriginalName)
		for _, m := range matches {
			if len(results) >= limit {
				break
			}
			results = append(results, m)
		}
	}

	if len(results) == 0 {
		return textResult(fmt.Sprintf("No results found for '%s'.", input.Query)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"query":   input.Query,
		"count":   len(results),
		"results": results,
	})
	return res, nil, err
}

// QueryProperties finds blocks/pages by property values.
func (s *Search) QueryProperties(ctx context.Context, req *mcp.CallToolRequest, input types.QueryPropertiesInput) (*mcp.CallToolResult, any, error) {
	operator := input.Operator
	if operator == "" {
		operator = "eq"
	}
	_ = operator // used for future operator support

	var query string
	if input.Value == "" {
		query = fmt.Sprintf(`[:find (pull ?b [:block/uuid :block/content :block/properties {:block/page [:block/name :block/original-name]}])
			:where
			[?b :block/properties ?props]
			[(get ?props :%s)]]`, input.Property)
	} else {
		query = fmt.Sprintf(`[:find (pull ?b [:block/uuid :block/content :block/properties {:block/page [:block/name :block/original-name]}])
			:where
			[?b :block/properties ?props]
			[(get ?props :%s) ?v]
			[(str ?v) ?vs]
			[(clojure.string/includes? ?vs "%s")]]`, input.Property, input.Value)
	}

	raw, err := s.client.DatascriptQuery(ctx, query)
	if err != nil {
		return errorResult(fmt.Sprintf("property query failed: %v", err)), nil, nil
	}

	res, err := jsonRawTextResult(raw)
	return res, nil, err
}

// QueryDatalog executes raw DataScript queries.
func (s *Search) QueryDatalog(ctx context.Context, req *mcp.CallToolRequest, input types.QueryDatalogInput) (*mcp.CallToolResult, any, error) {
	raw, err := s.client.DatascriptQuery(ctx, input.Query, input.Inputs...)
	if err != nil {
		return errorResult(fmt.Sprintf("Datalog query failed: %v", err)), nil, nil
	}

	res, err := jsonRawTextResult(raw)
	return res, nil, err
}

// FindByTag finds content by tag, including child tags.
func (s *Search) FindByTag(ctx context.Context, req *mcp.CallToolRequest, input types.FindByTagInput) (*mcp.CallToolResult, any, error) {
	query := fmt.Sprintf(`[:find (pull ?b [:block/uuid :block/content {:block/page [:block/name :block/original-name]}])
		:where
		[?b :block/refs ?ref]
		[?ref :block/name "%s"]]`, strings.ToLower(input.Tag))

	raw, err := s.client.DatascriptQuery(ctx, query)
	if err != nil {
		return errorResult(fmt.Sprintf("tag query failed: %v", err)), nil, nil
	}

	var results [][]json.RawMessage
	if err := json.Unmarshal(raw, &results); err != nil {
		res, err := jsonRawTextResult(raw)
		return res, nil, err
	}

	var enriched []map[string]any
	for _, r := range results {
		if len(r) == 0 {
			continue
		}
		var block types.BlockEntity
		if err := json.Unmarshal(r[0], &block); err != nil {
			continue
		}
		parsed := parser.Parse(block.Content)
		entry := map[string]any{
			"uuid":    block.UUID,
			"content": block.Content,
			"parsed":  parsed,
		}
		if block.Page != nil {
			entry["page"] = block.Page.Name
		}
		enriched = append(enriched, entry)
	}

	res, err := jsonTextResult(map[string]any{
		"tag":     input.Tag,
		"count":   len(enriched),
		"results": enriched,
	})
	return res, nil, err
}

// --- Internal helpers ---

func searchBlockTree(blocks []types.BlockEntity, query, pageName string) []map[string]any {
	var results []map[string]any
	searchBlocksRecursive(blocks, query, pageName, nil, &results)
	return results
}

func searchBlocksRecursive(blocks []types.BlockEntity, query, pageName string, parentChain []types.BlockSummary, results *[]map[string]any) {
	for i, b := range blocks {
		if strings.Contains(strings.ToLower(b.Content), query) {
			var siblings []types.BlockSummary
			start := i - 1
			if start < 0 {
				start = 0
			}
			end := i + 2
			if end > len(blocks) {
				end = len(blocks)
			}
			for j := start; j < end; j++ {
				if j != i {
					siblings = append(siblings, types.BlockSummary{
						UUID:    blocks[j].UUID,
						Content: blocks[j].Content,
					})
				}
			}

			parsed := parser.Parse(b.Content)
			match := map[string]any{
				"page":        pageName,
				"uuid":        b.UUID,
				"content":     b.Content,
				"parsed":      parsed,
				"parentChain": parentChain,
				"siblings":    siblings,
			}
			*results = append(*results, match)
		}

		if len(b.Children) > 0 {
			chain := append(append([]types.BlockSummary{}, parentChain...), types.BlockSummary{
				UUID:    b.UUID,
				Content: b.Content,
			})
			searchBlocksRecursive(b.Children, query, pageName, chain, results)
		}
	}
}

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

// Whiteboard implements whiteboard MCP tools.
type Whiteboard struct {
	client *client.Client
}

// NewWhiteboard creates a new Whiteboard tool handler.
func NewWhiteboard(c *client.Client) *Whiteboard {
	return &Whiteboard{client: c}
}

// ListWhiteboards returns all whiteboards in the graph.
func (w *Whiteboard) ListWhiteboards(ctx context.Context, req *mcp.CallToolRequest, input types.ListWhiteboardsInput) (*mcp.CallToolResult, any, error) {
	// Whiteboards in Logseq are pages stored in the whiteboards/ directory.
	// Try DataScript query first for whiteboard-type pages.
	query := `[:find (pull ?p [:block/uuid :block/name :block/original-name
	                           :block/created-at :block/updated-at])
		:where
		[?p :block/name]
		[?p :block/type "whiteboard"]]`

	raw, err := w.client.DatascriptQuery(ctx, query)
	if err != nil {
		// Fallback: try to find whiteboards by file path pattern
		return w.listWhiteboardsFallback(ctx)
	}

	var results [][]json.RawMessage
	if err := json.Unmarshal(raw, &results); err != nil {
		return w.listWhiteboardsFallback(ctx)
	}

	if len(results) == 0 {
		// Try fallback before giving up
		return w.listWhiteboardsFallback(ctx)
	}

	var boards []map[string]any
	for _, r := range results {
		if len(r) == 0 {
			continue
		}
		var page struct {
			UUID         string `json:"uuid"`
			Name         string `json:"name"`
			OriginalName string `json:"original-name"`
			CreatedAt    int64  `json:"created-at"`
			UpdatedAt    int64  `json:"updated-at"`
		}
		if err := json.Unmarshal(r[0], &page); err != nil {
			continue
		}
		name := page.OriginalName
		if name == "" {
			name = page.Name
		}
		boards = append(boards, map[string]any{
			"uuid":      page.UUID,
			"name":      name,
			"createdAt": page.CreatedAt,
			"updatedAt": page.UpdatedAt,
		})
	}

	if len(boards) == 0 {
		return textResult("No whiteboards found in the graph."), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"count":       len(boards),
		"whiteboards": boards,
	})
	return res, nil, err
}

// GetWhiteboard retrieves a whiteboard's content including embedded pages and connections.
func (w *Whiteboard) GetWhiteboard(ctx context.Context, req *mcp.CallToolRequest, input types.GetWhiteboardInput) (*mcp.CallToolResult, any, error) {
	// Get the whiteboard page's block tree
	blocks, err := w.client.GetPageBlocksTree(ctx, input.Name)
	if err != nil {
		return errorResult(fmt.Sprintf("whiteboard not found: %s — %v", input.Name, err)), nil, nil
	}

	var elements []map[string]any
	var embeddedPages []string
	var connections []map[string]any
	seenPages := make(map[string]bool)

	for _, b := range blocks {
		element := map[string]any{
			"uuid":    b.UUID,
			"content": b.Content,
		}

		if b.Properties != nil {
			element["properties"] = b.Properties

			// Detect shape type
			if lsType, ok := b.Properties["ls-type"]; ok {
				element["shapeType"] = lsType
			}

			// Detect embedded page references
			if pageRef, ok := b.Properties["logseq.tldraw.page"].(string); ok {
				element["embeddedPage"] = pageRef
				if !seenPages[pageRef] {
					embeddedPages = append(embeddedPages, pageRef)
					seenPages[pageRef] = true
				}
			}

			// Detect connectors (lines between shapes)
			source, hasSource := b.Properties["logseq.tldraw.source"]
			target, hasTarget := b.Properties["logseq.tldraw.target"]
			if hasSource && hasTarget {
				connections = append(connections, map[string]any{
					"source": source,
					"target": target,
				})
			}
		}

		// Also extract links from content
		parsed := parser.Parse(b.Content)
		if len(parsed.Links) > 0 {
			element["links"] = parsed.Links
			for _, link := range parsed.Links {
				if !seenPages[link] {
					embeddedPages = append(embeddedPages, link)
					seenPages[link] = true
				}
			}
		}

		elements = append(elements, element)
	}

	res, err := jsonTextResult(map[string]any{
		"name":          input.Name,
		"elementCount":  len(elements),
		"elements":      elements,
		"embeddedPages": embeddedPages,
		"connections":   connections,
	})
	return res, nil, err
}

// --- Fallback ---

func (w *Whiteboard) listWhiteboardsFallback(ctx context.Context) (*mcp.CallToolResult, any, error) {
	// Fallback: scan all pages and look for whiteboard indicators
	pages, err := w.client.GetAllPages(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list pages: %v", err)), nil, nil
	}

	var boards []map[string]any
	for _, p := range pages {
		if p.Name == "" {
			continue
		}
		// Check if file path indicates whiteboard
		if p.File != nil && strings.Contains(p.File.Path, "whiteboards/") {
			name := p.OriginalName
			if name == "" {
				name = p.Name
			}
			boards = append(boards, map[string]any{
				"uuid":      p.UUID,
				"name":      name,
				"updatedAt": p.UpdatedAt,
			})
		}
	}

	if len(boards) == 0 {
		return textResult("No whiteboards found in the graph."), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"count":       len(boards),
		"whiteboards": boards,
	})
	return res, nil, err
}

// Ensure types.BlockEntity fields are accessible (uses existing Content, UUID, Properties, Children)
var _ = types.BlockEntity{}

package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/backend"
	"github.com/skridlevsky/graphthulhu/graph"
	"github.com/skridlevsky/graphthulhu/types"
)

// isNumericPageName returns true if the page name consists only of
// digits and common stray characters like parentheses, backticks, etc.
// These are typically artifacts from Logseq block references, not real pages.
func isNumericPageName(name string) bool {
	cleaned := strings.TrimRight(name, ")`,. ")
	if cleaned == "" {
		return true
	}
	for _, r := range cleaned {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// Analyze implements graph analysis MCP tools.
type Analyze struct {
	client backend.Backend
	cache  *graph.Cache
}

// NewAnalyze creates a new Analyze tool handler with a 30-second graph cache.
func NewAnalyze(c backend.Backend) *Analyze {
	return &Analyze{
		client: c,
		cache:  graph.NewCache(c, 30*time.Second),
	}
}

// GraphOverview returns global graph statistics.
func (a *Analyze) GraphOverview(ctx context.Context, req *mcp.CallToolRequest, input types.GraphOverviewInput) (*mcp.CallToolResult, any, error) {
	g, err := a.cache.Get(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build graph: %v", err)), nil, nil
	}

	stats := g.Overview()

	res, err := jsonTextResult(stats)
	return res, nil, err
}

// FindConnections finds how two pages are connected in the graph.
func (a *Analyze) FindConnections(ctx context.Context, req *mcp.CallToolRequest, input types.FindConnectionsInput) (*mcp.CallToolResult, any, error) {
	g, err := a.cache.Get(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build graph: %v", err)), nil, nil
	}

	result := g.FindConnections(input.From, input.To, input.MaxDepth)

	if !result.DirectlyLinked && len(result.Paths) == 0 && len(result.SharedConnections) == 0 {
		return textResult(fmt.Sprintf("No connections found between '%s' and '%s'.", input.From, input.To)), nil, nil
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// KnowledgeGaps finds sparse areas in the knowledge graph.
func (a *Analyze) KnowledgeGaps(ctx context.Context, req *mcp.CallToolRequest, input types.KnowledgeGapsInput) (*mcp.CallToolResult, any, error) {
	g, err := a.cache.Get(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build graph: %v", err)), nil, nil
	}

	gaps := g.KnowledgeGaps()

	// Apply optional filters to orphan pages.
	if input.MinBlockCount > 0 || input.ExcludeNumeric {
		filtered := gaps.OrphanPages[:0]
		for _, name := range gaps.OrphanPages {
			if input.MinBlockCount > 0 {
				key := strings.ToLower(name)
				if g.BlockCounts[key] < input.MinBlockCount {
					continue
				}
			}
			if input.ExcludeNumeric && isNumericPageName(name) {
				continue
			}
			filtered = append(filtered, name)
		}
		gaps.OrphanPages = filtered
	}

	res, err := jsonTextResult(gaps)
	return res, nil, err
}

// ListOrphans returns the actual orphan page names (not just a count).
func (a *Analyze) ListOrphans(ctx context.Context, req *mcp.CallToolRequest, input types.ListOrphansInput) (*mcp.CallToolResult, any, error) {
	g, err := a.cache.Get(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build graph: %v", err)), nil, nil
	}

	gaps := g.KnowledgeGaps()

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	total := len(gaps.OrphanPages)
	var filtered []map[string]any

	for _, name := range gaps.OrphanPages {
		key := strings.ToLower(name)
		blockCount := g.BlockCounts[key]

		if input.MinBlockCount > 0 && blockCount < input.MinBlockCount {
			continue
		}
		if input.ExcludeNumeric && isNumericPageName(name) {
			continue
		}

		hasProps := false
		if p, ok := g.Pages[key]; ok && len(p.Properties) > 0 {
			hasProps = true
		}

		filtered = append(filtered, map[string]any{
			"name":          name,
			"blockCount":    blockCount,
			"hasProperties": hasProps,
		})

		if len(filtered) >= limit {
			break
		}
	}

	res, err := jsonTextResult(map[string]any{
		"total":    total,
		"returned": len(filtered),
		"orphans":  filtered,
	})
	return res, nil, err
}

// TopicClusters finds community clusters in the knowledge graph.
func (a *Analyze) TopicClusters(ctx context.Context, req *mcp.CallToolRequest, input types.TopicClustersInput) (*mcp.CallToolResult, any, error) {
	g, err := a.cache.Get(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build graph: %v", err)), nil, nil
	}

	clusters := g.TopicClusters()

	if len(clusters) == 0 {
		return textResult("No topic clusters found — the graph may be too sparse or disconnected."), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"clusterCount": len(clusters),
		"clusters":     clusters,
	})
	return res, nil, err
}

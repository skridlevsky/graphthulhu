package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/client"
	"github.com/skridlevsky/graphthulhu/graph"
	"github.com/skridlevsky/graphthulhu/types"
)

// Analyze implements graph analysis MCP tools.
type Analyze struct {
	client *client.Client
}

// NewAnalyze creates a new Analyze tool handler.
func NewAnalyze(c *client.Client) *Analyze {
	return &Analyze{client: c}
}

// GraphOverview returns global graph statistics.
func (a *Analyze) GraphOverview(ctx context.Context, req *mcp.CallToolRequest, input types.GraphOverviewInput) (*mcp.CallToolResult, any, error) {
	g, err := graph.Build(ctx, a.client)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build graph: %v", err)), nil, nil
	}

	stats := g.Overview()

	res, err := jsonTextResult(stats)
	return res, nil, err
}

// FindConnections finds how two pages are connected in the graph.
func (a *Analyze) FindConnections(ctx context.Context, req *mcp.CallToolRequest, input types.FindConnectionsInput) (*mcp.CallToolResult, any, error) {
	g, err := graph.Build(ctx, a.client)
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
	g, err := graph.Build(ctx, a.client)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build graph: %v", err)), nil, nil
	}

	gaps := g.KnowledgeGaps()

	res, err := jsonTextResult(gaps)
	return res, nil, err
}

// TopicClusters finds community clusters in the knowledge graph.
func (a *Analyze) TopicClusters(ctx context.Context, req *mcp.CallToolRequest, input types.TopicClustersInput) (*mcp.CallToolResult, any, error) {
	g, err := graph.Build(ctx, a.client)
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

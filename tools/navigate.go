package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/backend"
	"github.com/skridlevsky/graphthulhu/parser"
	"github.com/skridlevsky/graphthulhu/types"
)

// Navigate implements navigation MCP tools.
type Navigate struct {
	client backend.Backend
}

// NewNavigate creates a new Navigate tool handler.
func NewNavigate(c backend.Backend) *Navigate {
	return &Navigate{client: c}
}

// GetPage retrieves a page with its full recursive block tree and parsed content.
func (n *Navigate) GetPage(ctx context.Context, req *mcp.CallToolRequest, input types.GetPageInput) (*mcp.CallToolResult, any, error) {
	page, err := n.client.GetPage(ctx, input.Name)
	if err != nil {
		return errorResult(fmt.Sprintf("page not found: %s — %v", input.Name, err)), nil, nil
	}
	if page == nil {
		return errorResult(fmt.Sprintf("page not found: %s", input.Name)), nil, nil
	}

	blocks, err := n.client.GetPageBlocksTree(ctx, input.Name)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get blocks for %s: %v", input.Name, err)), nil, nil
	}

	depth := input.Depth
	if depth == 0 {
		depth = -1 // unlimited by default
	}

	enrichedBlocks := enrichBlockTree(blocks, depth, 0)
	outgoing := collectOutgoingLinks(enrichedBlocks)
	backlinks := n.getBacklinks(ctx, input.Name)

	result := &types.EnrichedPage{
		PageEntity:    *page,
		Blocks:        enrichedBlocks,
		OutgoingLinks: outgoing,
		BackLinks:     backlinks,
		BlockCount:    countBlocks(enrichedBlocks),
		LinkCount:     len(outgoing) + len(backlinks),
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// GetBlock retrieves a block with ancestors, children, and optionally siblings.
func (n *Navigate) GetBlock(ctx context.Context, req *mcp.CallToolRequest, input types.GetBlockInput) (*mcp.CallToolResult, any, error) {
	opts := map[string]any{"includeChildren": true}
	block, err := n.client.GetBlock(ctx, input.UUID, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("block not found: %s — %v", input.UUID, err)), nil, nil
	}
	if block == nil {
		return errorResult(fmt.Sprintf("block not found: %s", input.UUID)), nil, nil
	}

	enriched := enrichBlock(*block)

	if input.IncludeAncestors {
		ancestors, err := n.getAncestors(ctx, input.UUID)
		if err == nil {
			enriched.Ancestors = ancestors
		}
	}

	res, err := jsonTextResult(&enriched)
	return res, nil, err
}

// ListPages lists pages with optional filtering.
func (n *Navigate) ListPages(ctx context.Context, req *mcp.CallToolRequest, input types.ListPagesInput) (*mcp.CallToolResult, any, error) {
	pages, err := n.client.GetAllPages(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list pages: %v", err)), nil, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	var filtered []types.PageEntity
	for _, p := range pages {
		if p.Name == "" {
			continue
		}
		if input.Namespace != "" {
			if !strings.HasPrefix(strings.ToLower(p.Name), strings.ToLower(input.Namespace)) {
				continue
			}
		}
		if input.HasProperty != "" {
			if p.Properties == nil {
				continue
			}
			if _, ok := p.Properties[input.HasProperty]; !ok {
				continue
			}
		}
		filtered = append(filtered, p)
	}

	sortBy := input.SortBy
	if sortBy == "" {
		sortBy = "name"
	}
	sortPages(filtered, sortBy)

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	summaries := make([]map[string]any, len(filtered))
	for i, p := range filtered {
		summaries[i] = map[string]any{
			"name":       p.OriginalName,
			"properties": p.Properties,
			"journal":    p.Journal,
		}
		if p.UpdatedAt > 0 {
			summaries[i]["updatedAt"] = p.UpdatedAt
		}
	}

	res, err := jsonTextResult(summaries)
	return res, nil, err
}

// GetLinks returns forward links and backlinks for a page.
func (n *Navigate) GetLinks(ctx context.Context, req *mcp.CallToolRequest, input types.GetLinksInput) (*mcp.CallToolResult, any, error) {
	direction := input.Direction
	if direction == "" {
		direction = "both"
	}

	result := map[string]any{
		"page": input.Name,
	}

	if direction == "forward" || direction == "both" {
		blocks, err := n.client.GetPageBlocksTree(ctx, input.Name)
		if err == nil {
			outgoing := collectAllLinks(blocks)
			result["outgoingLinks"] = outgoing
		}
	}

	if direction == "backward" || direction == "both" {
		backlinks := n.getBacklinks(ctx, input.Name)
		result["backlinks"] = backlinks
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// GetReferences finds all blocks referencing a specific block via ((uuid)).
func (n *Navigate) GetReferences(ctx context.Context, req *mcp.CallToolRequest, input types.GetReferencesInput) (*mcp.CallToolResult, any, error) {
	query := fmt.Sprintf(`[:find (pull ?b [:block/uuid :block/content {:block/page [:block/name]}])
		:where
		[?b :block/refs ?ref]
		[?ref :block/uuid #uuid "%s"]]`, input.UUID)

	raw, err := n.client.DatascriptQuery(ctx, query)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to query references: %v", err)), nil, nil
	}

	res, err := jsonRawTextResult(raw)
	return res, nil, err
}

// Traverse finds paths between two pages using BFS on the link graph.
func (n *Navigate) Traverse(ctx context.Context, req *mcp.CallToolRequest, input types.TraverseInput) (*mcp.CallToolResult, any, error) {
	maxHops := input.MaxHops
	if maxHops <= 0 {
		maxHops = 4
	}

	paths := n.bfs(ctx, input.From, input.To, maxHops)

	if len(paths) == 0 {
		return textResult(fmt.Sprintf("No path found between '%s' and '%s' within %d hops.", input.From, input.To, maxHops)), nil, nil
	}

	result := map[string]any{
		"from":       input.From,
		"to":         input.To,
		"pathsFound": len(paths),
		"paths":      paths,
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// --- Internal helpers ---

func (n *Navigate) bfs(ctx context.Context, from, to string, maxHops int) [][]string {
	fromLower := strings.ToLower(from)
	toLower := strings.ToLower(to)

	type node struct {
		name string
		path []string
	}

	queue := []node{{name: fromLower, path: []string{from}}}
	visited := map[string]bool{fromLower: true}
	var paths [][]string

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if len(current.path) > maxHops+1 {
			break
		}

		blocks, err := n.client.GetPageBlocksTree(ctx, current.name)
		if err != nil {
			continue
		}

		links := collectAllLinks(blocks)
		for _, link := range links {
			linkLower := strings.ToLower(link)
			if linkLower == toLower {
				path := append(append([]string{}, current.path...), link)
				paths = append(paths, path)
				continue
			}
			if !visited[linkLower] && len(current.path) < maxHops {
				visited[linkLower] = true
				newPath := append(append([]string{}, current.path...), link)
				queue = append(queue, node{name: linkLower, path: newPath})
			}
		}
	}

	return paths
}

func (n *Navigate) getBacklinks(ctx context.Context, name string) []types.BackLink {
	raw, err := n.client.GetPageLinkedReferences(ctx, name)
	if err != nil {
		return nil
	}

	var refs [][]json.RawMessage
	if err := json.Unmarshal(raw, &refs); err != nil {
		return nil
	}

	var backlinks []types.BackLink
	for _, ref := range refs {
		if len(ref) < 2 {
			continue
		}

		var page types.PageEntity
		if err := json.Unmarshal(ref[0], &page); err != nil {
			continue
		}

		var blocks []types.BlockEntity
		if err := json.Unmarshal(ref[1], &blocks); err != nil {
			continue
		}

		bl := types.BackLink{
			PageName: page.OriginalName,
		}
		if bl.PageName == "" {
			bl.PageName = page.Name
		}
		for _, b := range blocks {
			bl.Blocks = append(bl.Blocks, types.BlockSummary{
				UUID:    b.UUID,
				Content: b.Content,
			})
		}
		backlinks = append(backlinks, bl)
	}

	return backlinks
}

func (n *Navigate) getAncestors(ctx context.Context, uuid string) ([]types.BlockSummary, error) {
	query := fmt.Sprintf(`[:find (pull ?parent [:block/uuid :block/content])
		:where
		[?b :block/uuid #uuid "%s"]
		[?b :block/parent ?parent]]`, uuid)

	raw, err := n.client.DatascriptQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	var results [][]json.RawMessage
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}

	var ancestors []types.BlockSummary
	for _, r := range results {
		if len(r) == 0 {
			continue
		}
		var block struct {
			UUID    string `json:"uuid"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(r[0], &block); err != nil {
			continue
		}
		if block.UUID != "" {
			ancestors = append(ancestors, types.BlockSummary{
				UUID:    block.UUID,
				Content: block.Content,
			})
		}
	}

	return ancestors, nil
}

func enrichBlockTree(blocks []types.BlockEntity, maxDepth, currentDepth int) []types.EnrichedBlock {
	if maxDepth >= 0 && currentDepth > maxDepth {
		return nil
	}

	enriched := make([]types.EnrichedBlock, 0, len(blocks))
	for _, b := range blocks {
		eb := enrichBlock(b)
		if len(b.Children) > 0 {
			childEnriched := enrichBlockTree(b.Children, maxDepth, currentDepth+1)
			for _, ce := range childEnriched {
				eb.BlockEntity.Children = append(eb.BlockEntity.Children, ce.BlockEntity)
			}
		}
		enriched = append(enriched, eb)
	}
	return enriched
}

func enrichBlock(b types.BlockEntity) types.EnrichedBlock {
	return types.EnrichedBlock{
		BlockEntity: b,
		Parsed:      parser.Parse(b.Content),
	}
}

func collectOutgoingLinks(blocks []types.EnrichedBlock) []string {
	seen := make(map[string]bool)
	var links []string
	for _, b := range blocks {
		for _, link := range b.Parsed.Links {
			if !seen[link] {
				links = append(links, link)
				seen[link] = true
			}
		}
	}
	return links
}

func collectAllLinks(blocks []types.BlockEntity) []string {
	seen := make(map[string]bool)
	var links []string
	var walk func([]types.BlockEntity)
	walk = func(bs []types.BlockEntity) {
		for _, b := range bs {
			parsed := parser.Parse(b.Content)
			for _, link := range parsed.Links {
				if !seen[link] {
					links = append(links, link)
					seen[link] = true
				}
			}
			if len(b.Children) > 0 {
				walk(b.Children)
			}
		}
	}
	walk(blocks)
	return links
}

func countBlocks(blocks []types.EnrichedBlock) int {
	count := len(blocks)
	for _, b := range blocks {
		if len(b.BlockEntity.Children) > 0 {
			count += countBlocksRaw(b.BlockEntity.Children)
		}
	}
	return count
}

func countBlocksRaw(blocks []types.BlockEntity) int {
	count := len(blocks)
	for _, b := range blocks {
		if len(b.Children) > 0 {
			count += countBlocksRaw(b.Children)
		}
	}
	return count
}

func sortPages(pages []types.PageEntity, sortBy string) {
	switch sortBy {
	case "modified":
		sortByField(pages, func(p types.PageEntity) int64 { return -p.UpdatedAt })
	case "created":
		sortByField(pages, func(p types.PageEntity) int64 { return -p.CreatedAt })
	default:
		sortByName(pages)
	}
}

func sortByName(pages []types.PageEntity) {
	for i := 1; i < len(pages); i++ {
		for j := i; j > 0 && strings.ToLower(pages[j].Name) < strings.ToLower(pages[j-1].Name); j-- {
			pages[j], pages[j-1] = pages[j-1], pages[j]
		}
	}
}

func sortByField(pages []types.PageEntity, key func(types.PageEntity) int64) {
	for i := 1; i < len(pages); i++ {
		for j := i; j > 0 && key(pages[j]) < key(pages[j-1]); j-- {
			pages[j], pages[j-1] = pages[j-1], pages[j]
		}
	}
}

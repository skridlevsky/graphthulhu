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

// Decision implements decision protocol MCP tools.
type Decision struct {
	client backend.Backend
}

// NewDecision creates a new Decision tool handler.
func NewDecision(c backend.Backend) *Decision {
	return &Decision{client: c}
}

// decisionBlock is a parsed decision found in the graph.
type decisionBlock struct {
	UUID       string `json:"uuid"`
	Content    string `json:"content"`
	Page       string `json:"page"`
	Marker     string `json:"marker"`
	Deadline   string `json:"deadline,omitempty"`
	Resolved   string `json:"resolved,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	DaysLeft   *int   `json:"daysLeft,omitempty"`
	Overdue    bool   `json:"overdue"`
	Deferred   int    `json:"deferred,omitempty"`
	DeferredOn string `json:"deferredOn,omitempty"`
}

// findDecisions queries all #decision tagged blocks and parses them.
func (d *Decision) findDecisions(ctx context.Context) ([]decisionBlock, error) {
	query := `[:find (pull ?b [:block/uuid :block/content
		{:block/page [:block/name :block/original-name]}])
		:where
		[?b :block/refs ?ref]
		[?ref :block/name "decision"]]`

	raw, err := d.client.DatascriptQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("decision query failed: %w", err)
	}

	var results [][]json.RawMessage
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, fmt.Errorf("parse results: %w", err)
	}

	todayStr := time.Now().Format("2006-01-02")
	today, _ := time.Parse("2006-01-02", todayStr)

	var decisions []decisionBlock
	for _, r := range results {
		if len(r) == 0 {
			continue
		}
		var block types.BlockEntity
		if err := json.Unmarshal(r[0], &block); err != nil {
			continue
		}

		db, ok := parseDecisionBlock(block, today)
		if !ok {
			continue
		}
		decisions = append(decisions, db)
	}

	return decisions, nil
}

// parseDecisionBlock extracts decision metadata from a block entity.
// Returns the parsed decisionBlock and whether it's a valid decision (has a marker).
func parseDecisionBlock(block types.BlockEntity, today time.Time) (decisionBlock, bool) {
	parsed := parser.Parse(block.Content)

	db := decisionBlock{
		UUID:    block.UUID,
		Content: block.Content,
	}
	if block.Page != nil {
		db.Page = block.Page.Name
	}

	// Extract marker from first line.
	firstLine := strings.SplitN(strings.TrimSpace(block.Content), "\n", 2)[0]
	for _, m := range []string{"DECIDE", "DONE", "TODO", "DOING", "WAIT", "LATER"} {
		if strings.HasPrefix(firstLine, m+" ") || firstLine == m {
			db.Marker = m
			break
		}
	}

	// Blocks without a decision marker are documentation mentions of
	// #decision, not actual decisions.
	if db.Marker == "" {
		return db, false
	}

	// Extract properties.
	if parsed.Properties != nil {
		if v, ok := parsed.Properties["deadline"]; ok {
			db.Deadline = fmt.Sprint(v)
		}
		if v, ok := parsed.Properties["resolved"]; ok {
			db.Resolved = fmt.Sprint(v)
		}
		if v, ok := parsed.Properties["outcome"]; ok {
			db.Outcome = fmt.Sprint(v)
		}
		if v, ok := parsed.Properties["deferred"]; ok {
			fmt.Sscanf(fmt.Sprint(v), "%d", &db.Deferred)
		}
		if v, ok := parsed.Properties["deferred-on"]; ok {
			db.DeferredOn = fmt.Sprint(v)
		}
	}

	// Calculate days left for unresolved decisions with deadlines.
	if db.Deadline != "" && db.Resolved == "" {
		if deadlineTime, err := time.Parse("2006-01-02", db.Deadline); err == nil {
			days := int(deadlineTime.Sub(today).Hours() / 24)
			db.DaysLeft = &days
			db.Overdue = days < 0
		}
	}

	return db, true
}

// DecisionCheck surfaces all decisions, highlighting overdue ones.
func (d *Decision) DecisionCheck(ctx context.Context, req *mcp.CallToolRequest, input types.DecisionCheckInput) (*mcp.CallToolResult, any, error) {
	decisions, err := d.findDecisions(ctx)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	if len(decisions) == 0 {
		return textResult("No decisions found. Use decision_create to track a decision with #decision tag and deadline."), nil, nil
	}

	var open, resolved, overdue []decisionBlock
	for _, db := range decisions {
		if db.Marker == "DONE" || db.Resolved != "" {
			resolved = append(resolved, db)
		} else if db.Overdue {
			overdue = append(overdue, db)
		} else {
			open = append(open, db)
		}
	}

	result := map[string]any{
		"total":    len(decisions),
		"open":     len(open),
		"overdue":  len(overdue),
		"resolved": len(resolved),
	}

	if input.IncludeResolved {
		result["decisions"] = decisions
	} else {
		active := append(overdue, open...)
		result["decisions"] = active
	}

	if len(overdue) > 0 {
		result["alert"] = fmt.Sprintf("%d decision(s) overdue — action required", len(overdue))
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// DecisionCreate creates a new DECIDE block with #decision tag and deadline.
func (d *Decision) DecisionCreate(ctx context.Context, req *mcp.CallToolRequest, input types.DecisionCreateInput) (*mcp.CallToolResult, any, error) {
	content := fmt.Sprintf("DECIDE %s #decision", input.Question)
	content += fmt.Sprintf("\ndeadline:: %s", input.Deadline)

	if len(input.Options) > 0 {
		content += fmt.Sprintf("\noptions:: %s", strings.Join(input.Options, ", "))
	}
	if input.Context != "" {
		content += fmt.Sprintf("\ncontext:: %s", input.Context)
	}

	block, err := d.client.AppendBlockInPage(ctx, input.Page, content)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create decision: %v", err)), nil, nil
	}

	result := map[string]any{
		"created":  true,
		"page":     input.Page,
		"question": input.Question,
		"deadline": input.Deadline,
	}
	if block != nil {
		result["uuid"] = block.UUID
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// DecisionResolve marks a decision as DONE with resolution date and outcome.
func (d *Decision) DecisionResolve(ctx context.Context, req *mcp.CallToolRequest, input types.DecisionResolveInput) (*mcp.CallToolResult, any, error) {
	block, err := d.client.GetBlock(ctx, input.UUID)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get block: %v", err)), nil, nil
	}

	content := block.Content
	today := time.Now().Format("2006-01-02")

	// Replace DECIDE marker with DONE.
	if strings.HasPrefix(content, "DECIDE ") {
		content = "DONE " + content[len("DECIDE "):]
	}

	// Add or update resolved property.
	content = upsertProperty(content, "resolved", today)

	// Add outcome if provided.
	if input.Outcome != "" {
		content = upsertProperty(content, "outcome", input.Outcome)
	}

	if err := d.client.UpdateBlock(ctx, input.UUID, content); err != nil {
		return errorResult(fmt.Sprintf("failed to update block: %v", err)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"resolved": true,
		"uuid":     input.UUID,
		"date":     today,
		"outcome":  input.Outcome,
	})
	return res, nil, err
}

// DecisionDefer pushes a deadline with a reason and increments defer count.
func (d *Decision) DecisionDefer(ctx context.Context, req *mcp.CallToolRequest, input types.DecisionDeferInput) (*mcp.CallToolResult, any, error) {
	block, err := d.client.GetBlock(ctx, input.UUID)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get block: %v", err)), nil, nil
	}

	content := block.Content
	today := time.Now().Format("2006-01-02")

	// Count existing deferrals.
	deferCount := 0
	parsed := parser.Parse(content)
	if parsed.Properties != nil {
		if v, ok := parsed.Properties["deferred"]; ok {
			fmt.Sscanf(fmt.Sprint(v), "%d", &deferCount)
		}
	}
	deferCount++

	// Update properties.
	content = upsertProperty(content, "deadline", input.NewDeadline)
	content = upsertProperty(content, "deferred", fmt.Sprintf("%d", deferCount))
	content = upsertProperty(content, "deferred-on", today)

	if err := d.client.UpdateBlock(ctx, input.UUID, content); err != nil {
		return errorResult(fmt.Sprintf("failed to update block: %v", err)), nil, nil
	}

	// Add reason as child block.
	if input.Reason != "" {
		reasonContent := fmt.Sprintf("Deferred %s: %s", today, input.Reason)
		d.client.InsertBlock(ctx, input.UUID, reasonContent, map[string]any{"sibling": false})
	}

	result := map[string]any{
		"deferred":    true,
		"uuid":        input.UUID,
		"newDeadline": input.NewDeadline,
		"deferCount":  deferCount,
		"date":        today,
	}
	if deferCount >= 3 {
		result["warning"] = fmt.Sprintf("Deferred %d times. Consider resolving or abandoning.", deferCount)
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// AnalysisHealth audits analysis/strategy pages for graph connectivity.
func (d *Decision) AnalysisHealth(ctx context.Context, req *mcp.CallToolRequest, input types.AnalysisHealthInput) (*mcp.CallToolResult, any, error) {
	query := `[:find (pull ?p [:block/name :block/original-name])
		:where
		[?p :block/name]
		[?p :block/properties ?props]
		[(get ?props :type) ?t]
		[(contains? #{"analysis" "strategy" "assessment"} ?t)]]`

	raw, err := d.client.DatascriptQuery(ctx, query)
	if err != nil {
		return errorResult(fmt.Sprintf("query failed: %v", err)), nil, nil
	}

	var results [][]json.RawMessage
	if err := json.Unmarshal(raw, &results); err != nil {
		return errorResult(fmt.Sprintf("parse failed: %v", err)), nil, nil
	}

	type pageHealth struct {
		Name          string `json:"name"`
		OutgoingLinks int    `json:"outgoingLinks"`
		Backlinks     int    `json:"backlinks"`
		HasDecision   bool   `json:"hasDecision"`
		Healthy       bool   `json:"healthy"`
		Issue         string `json:"issue,omitempty"`
	}

	var pages []pageHealth
	healthy := 0
	unhealthy := 0

	for _, r := range results {
		if len(r) == 0 {
			continue
		}
		var page types.PageEntity
		if err := json.Unmarshal(r[0], &page); err != nil {
			continue
		}

		name := page.Name

		blocks, err := d.client.GetPageBlocksTree(ctx, page.Name)
		if err != nil {
			continue
		}

		outgoing := 0
		hasDecision := false
		for _, b := range blocks {
			p := parser.Parse(b.Content)
			outgoing += len(p.Links)
			outgoing += countLinksInTree(b.Children)
			if strings.HasPrefix(b.Content, "DECIDE ") {
				hasDecision = true
			}
			if p.Tags != nil {
				for _, tag := range p.Tags {
					if strings.ToLower(tag) == "decision" {
						hasDecision = true
					}
				}
			}
		}

		// Count backlinks.
		backlinkData, _ := d.client.GetPageLinkedReferences(ctx, page.Name)
		backlinks := 0
		if backlinkData != nil {
			var bl []json.RawMessage
			if json.Unmarshal(backlinkData, &bl) == nil {
				backlinks = len(bl)
			}
		}

		// Healthy = 3+ outgoing links OR has a decision.
		isHealthy := outgoing >= 3 || hasDecision
		issue := ""
		if !isHealthy {
			issue = "isolated analysis — fewer than 3 outgoing links and no decision"
		}

		if isHealthy {
			healthy++
		} else {
			unhealthy++
		}

		pages = append(pages, pageHealth{
			Name:          name,
			OutgoingLinks: outgoing,
			Backlinks:     backlinks,
			HasDecision:   hasDecision,
			Healthy:       isHealthy,
			Issue:         issue,
		})
	}

	res, err := jsonTextResult(map[string]any{
		"total":     len(pages),
		"healthy":   healthy,
		"unhealthy": unhealthy,
		"pages":     pages,
	})
	return res, nil, err
}

// countLinksInTree recursively counts [[links]] in a block tree.
func countLinksInTree(blocks []types.BlockEntity) int {
	count := 0
	for _, b := range blocks {
		p := parser.Parse(b.Content)
		count += len(p.Links)
		count += countLinksInTree(b.Children)
	}
	return count
}

// upsertProperty adds or updates a key:: value property in block content.
func upsertProperty(content, key, value string) string {
	prefix := key + "::"
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = fmt.Sprintf("%s:: %s", key, value)
			return strings.Join(lines, "\n")
		}
	}
	return content + fmt.Sprintf("\n%s:: %s", key, value)
}

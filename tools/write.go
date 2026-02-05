package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/backend"
	"github.com/skridlevsky/graphthulhu/types"
)

// Write implements write MCP tools.
type Write struct {
	client backend.Backend
}

// NewWrite creates a new Write tool handler.
func NewWrite(c backend.Backend) *Write {
	return &Write{client: c}
}

// CreatePage creates a new page with optional properties and initial blocks.
func (w *Write) CreatePage(ctx context.Context, req *mcp.CallToolRequest, input types.CreatePageInput) (*mcp.CallToolResult, any, error) {
	page, err := w.client.CreatePage(ctx, input.Name, input.Properties, nil)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create page '%s': %v", input.Name, err)), nil, nil
	}

	for _, content := range input.Blocks {
		_, err := w.client.AppendBlockInPage(ctx, input.Name, content)
		if err != nil {
			return errorResult(fmt.Sprintf("created page but failed to add block: %v", err)), nil, nil
		}
	}

	result := map[string]any{
		"created":     true,
		"name":        input.Name,
		"blocksAdded": len(input.Blocks),
	}
	if page != nil {
		result["uuid"] = page.UUID
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// AppendBlocks appends plain-string blocks to an existing page (same API as create_page blocks).
func (w *Write) AppendBlocks(ctx context.Context, req *mcp.CallToolRequest, input types.AppendBlocksInput) (*mcp.CallToolResult, any, error) {
	if len(input.Blocks) == 0 {
		return errorResult("no blocks provided"), nil, nil
	}

	var createdUUIDs []string

	for _, content := range input.Blocks {
		block, err := w.client.AppendBlockInPage(ctx, input.Page, content)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to append block to '%s': %v (appended %d before failure)", input.Page, err, len(createdUUIDs))), nil, nil
		}
		if block != nil {
			createdUUIDs = append(createdUUIDs, block.UUID)
		}
	}

	res, err := jsonTextResult(map[string]any{
		"page":          input.Page,
		"blocksCreated": len(createdUUIDs),
		"uuids":         createdUUIDs,
	})
	return res, nil, err
}

// UpsertBlocksRaw is the raw ToolHandler for upsert_blocks (avoids recursive type cycle in schema generation).
func (w *Write) UpsertBlocksRaw(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var input types.UpsertBlocksInput
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return errorResult(fmt.Sprintf("invalid input: %v", err)), nil
	}
	result, _, err := w.upsertBlocks(ctx, input)
	return result, err
}

// upsertBlocks is the shared implementation.
func (w *Write) upsertBlocks(ctx context.Context, input types.UpsertBlocksInput) (*mcp.CallToolResult, any, error) {
	position := input.Position
	if position == "" {
		position = "append"
	}

	var createdUUIDs []string

	for _, block := range input.Blocks {
		content := block.Content
		for k, v := range block.Properties {
			content += fmt.Sprintf("\n%s:: %s", k, v)
		}

		var created *types.BlockEntity
		var err error

		if position == "prepend" {
			created, err = w.client.PrependBlockInPage(ctx, input.Page, content)
		} else {
			created, err = w.client.AppendBlockInPage(ctx, input.Page, content)
		}

		if err != nil {
			return errorResult(fmt.Sprintf("failed to create block: %v", err)), nil, nil
		}

		if created != nil {
			createdUUIDs = append(createdUUIDs, created.UUID)

			if len(block.Children) > 0 {
				childUUIDs, err := w.insertChildren(ctx, created.UUID, block.Children)
				if err != nil {
					return errorResult(fmt.Sprintf("failed to create child blocks: %v", err)), nil, nil
				}
				createdUUIDs = append(createdUUIDs, childUUIDs...)
			}
		}
	}

	res, err := jsonTextResult(map[string]any{
		"page":          input.Page,
		"blocksCreated": len(createdUUIDs),
		"uuids":         createdUUIDs,
	})
	return res, nil, err
}

// UpdateBlock updates an existing block's content.
func (w *Write) UpdateBlock(ctx context.Context, req *mcp.CallToolRequest, input types.UpdateBlockInput) (*mcp.CallToolResult, any, error) {
	err := w.client.UpdateBlock(ctx, input.UUID, input.Content)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to update block %s: %v", input.UUID, err)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"updated": true,
		"uuid":    input.UUID,
	})
	return res, nil, err
}

// DeleteBlock removes a block from the graph.
func (w *Write) DeleteBlock(ctx context.Context, req *mcp.CallToolRequest, input types.DeleteBlockInput) (*mcp.CallToolResult, any, error) {
	err := w.client.RemoveBlock(ctx, input.UUID)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to delete block %s: %v", input.UUID, err)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"deleted": true,
		"uuid":    input.UUID,
	})
	return res, nil, err
}

// MoveBlock moves a block to a new location.
func (w *Write) MoveBlock(ctx context.Context, req *mcp.CallToolRequest, input types.MoveBlockInput) (*mcp.CallToolResult, any, error) {
	position := input.Position
	if position == "" {
		position = "child"
	}

	opts := map[string]any{}
	switch position {
	case "before":
		opts["before"] = true
	case "child":
		opts["children"] = true
	}

	err := w.client.MoveBlock(ctx, input.UUID, input.TargetUUID, opts)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to move block: %v", err)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"moved":    true,
		"uuid":     input.UUID,
		"target":   input.TargetUUID,
		"position": position,
	})
	return res, nil, err
}

// LinkPages creates bidirectional links between two pages.
func (w *Write) LinkPages(ctx context.Context, req *mcp.CallToolRequest, input types.LinkPagesInput) (*mcp.CallToolResult, any, error) {
	fromContent := fmt.Sprintf("[[%s]]", input.To)
	if input.Context != "" {
		fromContent = fmt.Sprintf("%s — [[%s]]", input.Context, input.To)
	}

	fromBlock, err := w.client.AppendBlockInPage(ctx, input.From, fromContent)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to add link in '%s': %v", input.From, err)), nil, nil
	}

	toContent := fmt.Sprintf("[[%s]]", input.From)
	if input.Context != "" {
		toContent = fmt.Sprintf("%s — [[%s]]", input.Context, input.From)
	}

	toBlock, err := w.client.AppendBlockInPage(ctx, input.To, toContent)
	if err != nil {
		return errorResult(fmt.Sprintf("linked from '%s' but failed to link back from '%s': %v", input.From, input.To, err)), nil, nil
	}

	result := map[string]any{
		"linked": true,
		"from":   input.From,
		"to":     input.To,
	}
	if fromBlock != nil {
		result["fromBlockUUID"] = fromBlock.UUID
	}
	if toBlock != nil {
		result["toBlockUUID"] = toBlock.UUID
	}

	res, err := jsonTextResult(result)
	return res, nil, err
}

// DeletePage removes a page from the graph.
func (w *Write) DeletePage(ctx context.Context, req *mcp.CallToolRequest, input types.DeletePageInput) (*mcp.CallToolResult, any, error) {
	err := w.client.DeletePage(ctx, input.Name)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to delete page '%s': %v", input.Name, err)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"deleted": true,
		"name":    input.Name,
	})
	return res, nil, err
}

// RenamePage renames a page and updates all links across the graph.
func (w *Write) RenamePage(ctx context.Context, req *mcp.CallToolRequest, input types.RenamePageInput) (*mcp.CallToolResult, any, error) {
	err := w.client.RenamePage(ctx, input.OldName, input.NewName)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to rename '%s' to '%s': %v", input.OldName, input.NewName, err)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"renamed": true,
		"from":    input.OldName,
		"to":      input.NewName,
	})
	return res, nil, err
}

// BulkUpdateProperties sets a property on multiple pages at once.
func (w *Write) BulkUpdateProperties(ctx context.Context, req *mcp.CallToolRequest, input types.BulkUpdatePropertiesInput) (*mcp.CallToolResult, any, error) {
	if len(input.Pages) == 0 {
		return errorResult("no pages specified"), nil, nil
	}

	var updated []string
	var failed []string

	for _, pageName := range input.Pages {
		// Get the page's first block (property block in Logseq).
		blocks, err := w.client.GetPageBlocksTree(ctx, pageName)
		if err != nil || len(blocks) == 0 {
			failed = append(failed, pageName)
			continue
		}

		// Find or create the property in the first block.
		firstBlock := blocks[0]
		content := firstBlock.Content
		propLine := fmt.Sprintf("%s:: %s", input.Property, input.Value)

		// Check if property already exists.
		lines := strings.Split(content, "\n")
		found := false
		for i, line := range lines {
			if strings.HasPrefix(line, input.Property+":: ") {
				lines[i] = propLine
				found = true
				break
			}
		}
		if !found {
			// Append property line.
			lines = append(lines, propLine)
		}

		newContent := strings.Join(lines, "\n")
		if err := w.client.UpdateBlock(ctx, firstBlock.UUID, newContent); err != nil {
			failed = append(failed, pageName)
			continue
		}
		updated = append(updated, pageName)
	}

	res, err := jsonTextResult(map[string]any{
		"property":     input.Property,
		"value":        input.Value,
		"updated":      updated,
		"updatedCount": len(updated),
		"failed":       failed,
		"failedCount":  len(failed),
	})
	return res, nil, err
}

func (w *Write) insertChildren(ctx context.Context, parentUUID string, children []types.BlockInput) ([]string, error) {
	var uuids []string

	for _, child := range children {
		content := child.Content
		for k, v := range child.Properties {
			content += fmt.Sprintf("\n%s:: %s", k, v)
		}

		created, err := w.client.InsertBlock(ctx, parentUUID, content, map[string]any{
			"isPageBlock": false,
		})
		if err != nil {
			return uuids, err
		}

		if created != nil {
			uuids = append(uuids, created.UUID)

			if len(child.Children) > 0 {
				childUUIDs, err := w.insertChildren(ctx, created.UUID, child.Children)
				if err != nil {
					return uuids, err
				}
				uuids = append(uuids, childUUIDs...)
			}
		}
	}

	return uuids, nil
}

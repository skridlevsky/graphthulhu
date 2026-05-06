package tools

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/types"
	"github.com/skridlevsky/graphthulhu/vault"
)

// Attachments implements binary attachment MCP tools (Obsidian-only).
type Attachments struct {
	client *vault.Client
}

// NewAttachments creates a new Attachments tool handler.
func NewAttachments(c *vault.Client) *Attachments {
	return &Attachments{client: c}
}

// Upload writes a binary attachment to the vault.
func (a *Attachments) Upload(ctx context.Context, req *mcp.CallToolRequest, input types.UploadAttachmentInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" {
		return errorResult("path is required"), nil, nil
	}
	data, err := base64.StdEncoding.DecodeString(input.ContentBase64)
	if err != nil {
		return errorResult(fmt.Sprintf("invalid base64 content: %v", err)), nil, nil
	}
	result, err := a.client.UploadAttachment(input.Path, data)
	if err != nil {
		return errorResult(fmt.Sprintf("upload_attachment failed: %v", err)), nil, nil
	}
	res, err := jsonTextResult(result)
	return res, nil, err
}

// Delete removes a binary attachment from the vault.
func (a *Attachments) Delete(ctx context.Context, req *mcp.CallToolRequest, input types.DeleteAttachmentInput) (*mcp.CallToolResult, any, error) {
	if input.Path == "" {
		return errorResult("path is required"), nil, nil
	}
	if err := a.client.DeleteAttachment(input.Path); err != nil {
		return errorResult(fmt.Sprintf("delete_attachment failed: %v", err)), nil, nil
	}
	res, err := jsonTextResult(map[string]any{
		"deleted": true,
		"path":    input.Path,
	})
	return res, nil, err
}

// List returns non-.md files in a folder.
func (a *Attachments) List(ctx context.Context, req *mcp.CallToolRequest, input types.ListAttachmentsInput) (*mcp.CallToolResult, any, error) {
	if input.Folder == "" {
		return errorResult("folder is required"), nil, nil
	}
	entries, err := a.client.ListAttachments(input.Folder, input.Recursive)
	if err != nil {
		return errorResult(fmt.Sprintf("list_attachments failed: %v", err)), nil, nil
	}
	res, err := jsonTextResult(map[string]any{
		"folder":  input.Folder,
		"count":   len(entries),
		"entries": entries,
	})
	return res, nil, err
}

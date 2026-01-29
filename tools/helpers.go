package tools

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// textResult creates a successful text CallToolResult.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// errorResult creates an error CallToolResult (visible to the LLM for self-correction).
func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

// jsonTextResult marshals any value to indented JSON and wraps it as text content.
func jsonTextResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return textResult(string(data)), nil
}

// jsonRawTextResult pretty-prints raw JSON bytes.
func jsonRawTextResult(raw json.RawMessage) (*mcp.CallToolResult, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return textResult(string(raw)), nil
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return textResult(string(raw)), nil
	}
	return textResult(string(data)), nil
}

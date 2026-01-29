package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/skridlevsky/graphthulhu/types"
)

const (
	defaultAPIURL      = "http://127.0.0.1:12315"
	defaultTimeout     = 10 * time.Second
	maxRetries         = 3
	initialBackoff     = 100 * time.Millisecond
)

// Client communicates with the Logseq HTTP API.
type Client struct {
	apiURL     string
	token      string
	httpClient *http.Client
}

// New creates a new Logseq API client.
// Reads LOGSEQ_API_URL and LOGSEQ_API_TOKEN from environment if not provided.
func New(apiURL, token string) *Client {
	if apiURL == "" {
		apiURL = os.Getenv("LOGSEQ_API_URL")
	}
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	if token == "" {
		token = os.Getenv("LOGSEQ_API_TOKEN")
	}

	return &Client{
		apiURL: apiURL,
		token:  token,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// call makes a POST request to the Logseq API with retry and backoff.
func (c *Client) call(ctx context.Context, method string, args ...any) (json.RawMessage, error) {
	reqBody := types.LogseqAPIRequest{
		Method: method,
		Args:   args,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/api", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("logseq API %s (attempt %d): %w", method, attempt+1, err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response %s (attempt %d): %w", method, attempt+1, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("logseq API %s returned %d: %s", method, resp.StatusCode, string(respBody))
			if resp.StatusCode >= 500 {
				continue // retry server errors
			}
			return nil, lastErr // don't retry client errors
		}

		return respBody, nil
	}

	return nil, lastErr
}

// callTyped makes a Logseq API call and unmarshals the response into T.
func callTyped[T any](c *Client, ctx context.Context, method string, args ...any) (T, error) {
	var zero T
	raw, err := c.call(ctx, method, args...)
	if err != nil {
		return zero, err
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return zero, fmt.Errorf("unmarshal %s response: %w", method, err)
	}
	return result, nil
}

// --- Page Operations ---

// GetAllPages returns all pages in the graph.
func (c *Client) GetAllPages(ctx context.Context) ([]types.PageEntity, error) {
	return callTyped[[]types.PageEntity](c, ctx, "logseq.Editor.getAllPages")
}

// GetPage returns a page by name or ID.
func (c *Client) GetPage(ctx context.Context, nameOrID any) (*types.PageEntity, error) {
	return callTyped[*types.PageEntity](c, ctx, "logseq.Editor.getPage", nameOrID)
}

// GetPageBlocksTree returns the full block tree for a page.
func (c *Client) GetPageBlocksTree(ctx context.Context, nameOrID any) ([]types.BlockEntity, error) {
	return callTyped[[]types.BlockEntity](c, ctx, "logseq.Editor.getPageBlocksTree", nameOrID)
}

// GetPageLinkedReferences returns pages that link to this page, with the blocks containing the links.
func (c *Client) GetPageLinkedReferences(ctx context.Context, nameOrID any) (json.RawMessage, error) {
	return c.call(ctx, "logseq.Editor.getPageLinkedReferences", nameOrID)
}

// CreatePage creates a new page with optional properties.
func (c *Client) CreatePage(ctx context.Context, name string, properties map[string]any, opts map[string]any) (*types.PageEntity, error) {
	args := []any{name}
	if properties != nil {
		args = append(args, properties)
	} else {
		args = append(args, nil)
	}
	if opts != nil {
		args = append(args, opts)
	}
	return callTyped[*types.PageEntity](c, ctx, "logseq.Editor.createPage", args...)
}

// DeletePage removes a page by name.
func (c *Client) DeletePage(ctx context.Context, name string) error {
	_, err := c.call(ctx, "logseq.Editor.deletePage", name)
	return err
}

// --- Block Operations ---

// GetBlock returns a block by UUID.
func (c *Client) GetBlock(ctx context.Context, uuid string, opts ...map[string]any) (*types.BlockEntity, error) {
	args := []any{uuid}
	if len(opts) > 0 {
		args = append(args, opts[0])
	}
	return callTyped[*types.BlockEntity](c, ctx, "logseq.Editor.getBlock", args...)
}

// InsertBlock inserts a block relative to another block.
func (c *Client) InsertBlock(ctx context.Context, srcBlock any, content string, opts map[string]any) (*types.BlockEntity, error) {
	args := []any{srcBlock, content}
	if opts != nil {
		args = append(args, opts)
	}
	return callTyped[*types.BlockEntity](c, ctx, "logseq.Editor.insertBlock", args...)
}

// UpdateBlock modifies a block's content.
func (c *Client) UpdateBlock(ctx context.Context, uuid string, content string, opts ...map[string]any) error {
	args := []any{uuid, content}
	if len(opts) > 0 {
		args = append(args, opts[0])
	}
	_, err := c.call(ctx, "logseq.Editor.updateBlock", args...)
	return err
}

// RemoveBlock deletes a block.
func (c *Client) RemoveBlock(ctx context.Context, uuid string) error {
	_, err := c.call(ctx, "logseq.Editor.removeBlock", uuid)
	return err
}

// AppendBlockInPage adds a block at the end of a page.
func (c *Client) AppendBlockInPage(ctx context.Context, page string, content string) (*types.BlockEntity, error) {
	return callTyped[*types.BlockEntity](c, ctx, "logseq.Editor.appendBlockInPage", page, content)
}

// PrependBlockInPage adds a block at the start of a page.
func (c *Client) PrependBlockInPage(ctx context.Context, page string, content string) (*types.BlockEntity, error) {
	return callTyped[*types.BlockEntity](c, ctx, "logseq.Editor.prependBlockInPage", page, content)
}

// MoveBlock moves a block to a new location.
func (c *Client) MoveBlock(ctx context.Context, uuid string, targetUUID string, opts map[string]any) error {
	args := []any{uuid, targetUUID}
	if opts != nil {
		args = append(args, opts)
	}
	_, err := c.call(ctx, "logseq.Editor.moveBlock", args...)
	return err
}

// InsertBatchBlock inserts multiple blocks at once.
func (c *Client) InsertBatchBlock(ctx context.Context, srcBlock any, batch []map[string]any, opts map[string]any) ([]types.BlockEntity, error) {
	args := []any{srcBlock, batch}
	if opts != nil {
		args = append(args, opts)
	}
	return callTyped[[]types.BlockEntity](c, ctx, "logseq.Editor.insertBatchBlock", args...)
}

// --- Query Operations ---

// DatascriptQuery executes a Datalog query against the Logseq database.
func (c *Client) DatascriptQuery(ctx context.Context, query string, inputs ...any) (json.RawMessage, error) {
	args := []any{query}
	args = append(args, inputs...)
	return c.call(ctx, "logseq.DB.datascriptQuery", args...)
}

// DSLQuery executes a Logseq DSL query.
func (c *Client) DSLQuery(ctx context.Context, dsl string) (json.RawMessage, error) {
	return c.call(ctx, "logseq.DB.q", dsl)
}

// --- Tag Operations ---

// GetAllTags returns all tags in the graph.
func (c *Client) GetAllTags(ctx context.Context) ([]types.PageEntity, error) {
	return callTyped[[]types.PageEntity](c, ctx, "logseq.Editor.getAllTags")
}

// --- Property Operations ---

// GetPageProperties returns all properties for a page.
func (c *Client) GetPageProperties(ctx context.Context, nameOrID any) (map[string]any, error) {
	return callTyped[map[string]any](c, ctx, "logseq.Editor.getPageProperties", nameOrID)
}

// GetBlockProperties returns all properties for a block.
func (c *Client) GetBlockProperties(ctx context.Context, uuid string) (map[string]any, error) {
	return callTyped[map[string]any](c, ctx, "logseq.Editor.getBlockProperties", uuid)
}

// --- Namespace Operations ---

// GetPagesFromNamespace returns all pages in a namespace.
func (c *Client) GetPagesFromNamespace(ctx context.Context, namespace string) ([]types.PageEntity, error) {
	return callTyped[[]types.PageEntity](c, ctx, "logseq.Editor.getPagesFromNamespace", namespace)
}

// --- App Operations ---

// GraphInfo represents the current graph's metadata.
type GraphInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// GetCurrentGraph returns the name and filesystem path of the current graph.
func (c *Client) GetCurrentGraph(ctx context.Context) (*GraphInfo, error) {
	return callTyped[*GraphInfo](c, ctx, "logseq.App.getCurrentGraph")
}

// Ping checks if the Logseq API is reachable by attempting to list pages.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.call(ctx, "logseq.Editor.getCurrentPage")
	return err
}

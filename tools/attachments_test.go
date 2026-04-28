package tools

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/types"
	"github.com/skridlevsky/graphthulhu/vault"
)

func newTestAttachments(t *testing.T) (*Attachments, *vault.Client) {
	t.Helper()
	dir := t.TempDir()
	c := vault.New(dir)
	return NewAttachments(c), c
}

func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is not TextContent: %T", r.Content[0])
	}
	return tc.Text
}

func TestAttachmentsUpload_HappyPath(t *testing.T) {
	a, _ := newTestAttachments(t)
	input := types.UploadAttachmentInput{
		Path:          "Attach/note.png",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("fake-png-bytes")),
	}
	res, _, err := a.Upload(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.IsError {
		t.Fatalf("IsError=true: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, `"path": "Attach/note.png"`) {
		t.Errorf("response missing path: %s", text)
	}
	if !strings.Contains(text, `"size": 14`) {
		t.Errorf("response missing size: %s", text)
	}
}

func TestAttachmentsUpload_InvalidBase64(t *testing.T) {
	a, _ := newTestAttachments(t)
	input := types.UploadAttachmentInput{
		Path:          "Attach/note.png",
		ContentBase64: "!!!not-base64!!!",
	}
	res, _, _ := a.Upload(context.Background(), nil, input)
	if !res.IsError {
		t.Fatal("IsError should be true on invalid base64")
	}
	if !strings.Contains(resultText(t, res), "invalid base64") {
		t.Errorf("error message missing 'invalid base64': %s", resultText(t, res))
	}
}

func TestAttachmentsUpload_EmptyPath(t *testing.T) {
	a, _ := newTestAttachments(t)
	res, _, _ := a.Upload(context.Background(), nil, types.UploadAttachmentInput{Path: ""})
	if !res.IsError {
		t.Fatal("IsError should be true on empty path")
	}
}

func TestAttachmentsUpload_RejectsMarkdown(t *testing.T) {
	a, _ := newTestAttachments(t)
	input := types.UploadAttachmentInput{
		Path:          "page.md",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("x")),
	}
	res, _, _ := a.Upload(context.Background(), nil, input)
	if !res.IsError {
		t.Fatal("IsError should be true on .md path")
	}
	if !strings.Contains(resultText(t, res), ".md") {
		t.Errorf("error message should mention .md: %s", resultText(t, res))
	}
}

func TestAttachmentsDelete_HappyPath(t *testing.T) {
	a, c := newTestAttachments(t)
	if _, err := c.UploadAttachment("Attach/note.png", []byte("x")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, _, err := a.Delete(context.Background(), nil, types.DeleteAttachmentInput{Path: "Attach/note.png"})
	if err != nil || res.IsError {
		t.Fatalf("delete: err=%v isError=%v body=%s", err, res.IsError, resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), `"deleted": true`) {
		t.Errorf("response missing deleted=true: %s", resultText(t, res))
	}
}

func TestAttachmentsDelete_NotFound(t *testing.T) {
	a, _ := newTestAttachments(t)
	res, _, _ := a.Delete(context.Background(), nil, types.DeleteAttachmentInput{Path: "missing.bin"})
	if !res.IsError {
		t.Fatal("IsError should be true when not found")
	}
}

func TestAttachmentsDelete_EmptyPath(t *testing.T) {
	a, _ := newTestAttachments(t)
	res, _, _ := a.Delete(context.Background(), nil, types.DeleteAttachmentInput{Path: ""})
	if !res.IsError {
		t.Fatal("IsError should be true on empty path")
	}
}

func TestAttachmentsList_HappyPath(t *testing.T) {
	a, c := newTestAttachments(t)
	if _, err := c.UploadAttachment("Attach/a.png", []byte("a")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := c.UploadAttachment("Attach/b.pdf", []byte("bb")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, _, err := a.List(context.Background(), nil, types.ListAttachmentsInput{Folder: "Attach"})
	if err != nil || res.IsError {
		t.Fatalf("list: err=%v isError=%v body=%s", err, res.IsError, resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, `"count": 2`) {
		t.Errorf("response missing count=2: %s", text)
	}
	if !strings.Contains(text, `"path": "Attach/a.png"`) || !strings.Contains(text, `"path": "Attach/b.pdf"`) {
		t.Errorf("entries missing in response: %s", text)
	}
}

func TestAttachmentsList_EmptyFolder(t *testing.T) {
	a, _ := newTestAttachments(t)
	res, _, _ := a.List(context.Background(), nil, types.ListAttachmentsInput{Folder: ""})
	if !res.IsError {
		t.Fatal("IsError should be true on empty folder")
	}
}

func TestAttachmentsList_FolderNotFound(t *testing.T) {
	a, _ := newTestAttachments(t)
	res, _, _ := a.List(context.Background(), nil, types.ListAttachmentsInput{Folder: "missing"})
	if !res.IsError {
		t.Fatal("IsError should be true when folder missing")
	}
}

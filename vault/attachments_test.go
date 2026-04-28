package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rwVault returns a Client rooted at a fresh t.TempDir, suitable for
// tests that exercise UploadAttachment / DeleteAttachment / ListAttachments.
func rwVault(t *testing.T) *Client {
	t.Helper()
	dir := t.TempDir()
	return New(dir)
}

func TestUploadAttachment_HappyPath(t *testing.T) {
	c := rwVault(t)

	res, err := c.UploadAttachment("00_Inbox/email-x/Attachements/invoice.pdf", []byte("PDF-1.4 fake"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if res.Path != "00_Inbox/email-x/Attachements/invoice.pdf" {
		t.Errorf("path = %q, want forward-slash relative path", res.Path)
	}
	if res.Size != int64(len("PDF-1.4 fake")) {
		t.Errorf("size = %d, want %d", res.Size, len("PDF-1.4 fake"))
	}

	abs := filepath.Join(c.vaultPath, "00_Inbox", "email-x", "Attachements", "invoice.pdf")
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "PDF-1.4 fake" {
		t.Errorf("content = %q", got)
	}
}

func TestUploadAttachment_AutoCreatesParent(t *testing.T) {
	c := rwVault(t)
	rel := "deep/nested/path/that/does/not/exist/file.bin"

	if _, err := c.UploadAttachment(rel, []byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.vaultPath, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("parent dirs not created: %v", err)
	}
}

func TestUploadAttachment_OverwritesIdempotently(t *testing.T) {
	c := rwVault(t)
	rel := "data/file.bin"

	if _, err := c.UploadAttachment(rel, []byte("first")); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if _, err := c.UploadAttachment(rel, []byte("second-longer")); err != nil {
		t.Fatalf("second upload: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(c.vaultPath, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "second-longer" {
		t.Errorf("content = %q, want second-longer (atomic replace)", got)
	}
}

func TestUploadAttachment_RejectsMarkdown(t *testing.T) {
	c := rwVault(t)

	cases := []string{"foo.md", "deep/path/file.MD", "weird.Md"}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			_, err := c.UploadAttachment(p, []byte("x"))
			if !errors.Is(err, ErrAttachmentIsMarkdown) {
				t.Errorf("err = %v, want ErrAttachmentIsMarkdown", err)
			}
		})
	}
}

func TestUploadAttachment_RejectsTraversal(t *testing.T) {
	c := rwVault(t)

	_, err := c.UploadAttachment("../escape.bin", []byte("x"))
	if err == nil || !errors.Is(err, ErrPathEscape) {
		t.Errorf("err = %v, want ErrPathEscape", err)
	}
}

func TestDeleteAttachment_HappyPath(t *testing.T) {
	c := rwVault(t)
	rel := "data/file.bin"
	if _, err := c.UploadAttachment(rel, []byte("x")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := c.DeleteAttachment(rel); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.vaultPath, filepath.FromSlash(rel))); !os.IsNotExist(err) {
		t.Errorf("file still exists after delete: %v", err)
	}
}

func TestDeleteAttachment_NotFound(t *testing.T) {
	c := rwVault(t)

	err := c.DeleteAttachment("does/not/exist.bin")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want 'not found'", err)
	}
}

func TestDeleteAttachment_RejectsMarkdown(t *testing.T) {
	c := rwVault(t)
	err := c.DeleteAttachment("notes/page.md")
	if !errors.Is(err, ErrAttachmentIsMarkdown) {
		t.Errorf("err = %v, want ErrAttachmentIsMarkdown", err)
	}
}

func TestDeleteAttachment_RejectsDirectory(t *testing.T) {
	c := rwVault(t)
	if err := os.MkdirAll(filepath.Join(c.vaultPath, "somedir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err := c.DeleteAttachment("somedir")
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Errorf("err = %v, want directory rejection", err)
	}
}

func TestListAttachments_HappyPath_NonRecursive(t *testing.T) {
	c := rwVault(t)
	mustUpload(t, c, "Attach/a.png", "a")
	mustUpload(t, c, "Attach/b.pdf", "bb")
	mustUpload(t, c, "Attach/sub/c.jpg", "ccc") // shouldn't appear (non-recursive)

	entries, err := c.ListAttachments("Attach", false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("count = %d, want 2 (got: %v)", len(entries), entries)
	}
	if entries[0].Path != "Attach/a.png" || entries[1].Path != "Attach/b.pdf" {
		t.Errorf("entries not sorted by path: %v", entries)
	}
	if entries[0].Size != 1 || entries[1].Size != 2 {
		t.Errorf("sizes wrong: %v", entries)
	}
	if entries[0].Mtime == "" {
		t.Error("mtime empty")
	}
}

func TestListAttachments_HappyPath_Recursive(t *testing.T) {
	c := rwVault(t)
	mustUpload(t, c, "Attach/a.png", "a")
	mustUpload(t, c, "Attach/sub/b.jpg", "bb")
	mustUpload(t, c, "Attach/sub/deep/c.gif", "ccc")

	entries, err := c.ListAttachments("Attach", true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("count = %d, want 3 (got: %v)", len(entries), entries)
	}
	expected := []string{"Attach/a.png", "Attach/sub/b.jpg", "Attach/sub/deep/c.gif"}
	for i, e := range expected {
		if entries[i].Path != e {
			t.Errorf("entry[%d].Path = %q, want %q", i, entries[i].Path, e)
		}
	}
}

func TestListAttachments_FiltersOutMarkdown(t *testing.T) {
	c := rwVault(t)
	mustUpload(t, c, "Mixed/binary.png", "x")
	if err := os.WriteFile(filepath.Join(c.vaultPath, "Mixed", "note.md"), []byte("# md"), 0o644); err != nil {
		t.Fatalf("seed md: %v", err)
	}

	entries, err := c.ListAttachments("Mixed", false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "Mixed/binary.png" {
		t.Errorf("entries = %v, want only binary.png", entries)
	}
}

func TestListAttachments_FolderNotFound(t *testing.T) {
	c := rwVault(t)
	_, err := c.ListAttachments("does/not/exist", false)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want 'not found'", err)
	}
}

func TestListAttachments_NotADirectory(t *testing.T) {
	c := rwVault(t)
	mustUpload(t, c, "data/file.bin", "x")
	_, err := c.ListAttachments("data/file.bin", false)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("err = %v, want 'not a directory'", err)
	}
}

func TestListAttachments_EmptyFolder(t *testing.T) {
	c := rwVault(t)
	if err := os.MkdirAll(filepath.Join(c.vaultPath, "Empty"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	entries, err := c.ListAttachments("Empty", false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %v, want empty", entries)
	}
}

func mustUpload(t *testing.T, c *Client, rel, content string) {
	t.Helper()
	if _, err := c.UploadAttachment(rel, []byte(content)); err != nil {
		t.Fatalf("seed upload %s: %v", rel, err)
	}
}

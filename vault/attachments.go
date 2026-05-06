package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AttachmentResult is returned by UploadAttachment.
type AttachmentResult struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// AttachmentEntry is one item in a list_attachments response.
type AttachmentEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Mtime string `json:"mtime"`
}

// ErrAttachmentIsMarkdown is returned when an attachment operation targets a .md file.
var ErrAttachmentIsMarkdown = fmt.Errorf("attachment path cannot end in .md (use create_page/delete_page)")

// UploadAttachment writes a binary file at relPath (relative to vault root).
// Parent directories are auto-created. Atomic via temp+rename.
// Refuses .md paths (use CreatePage instead).
func (c *Client) UploadAttachment(relPath string, data []byte) (*AttachmentResult, error) {
	if strings.HasSuffix(strings.ToLower(relPath), ".md") {
		return nil, ErrAttachmentIsMarkdown
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	absPath, err := c.safePath(relPath)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	if err := atomicWriteBytes(absPath, data); err != nil {
		return nil, fmt.Errorf("write attachment: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat attachment: %w", err)
	}

	return &AttachmentResult{
		Path: filepath.ToSlash(relPath),
		Size: info.Size(),
	}, nil
}

// DeleteAttachment removes a binary file from the vault.
// Refuses .md paths (use DeletePage instead).
func (c *Client) DeleteAttachment(relPath string) error {
	if strings.HasSuffix(strings.ToLower(relPath), ".md") {
		return ErrAttachmentIsMarkdown
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	absPath, err := c.safePath(relPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("attachment not found: %s", relPath)
		}
		return fmt.Errorf("stat attachment: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", relPath)
	}

	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

// ListAttachments returns non-.md files under relFolder (relative to vault root).
// recursive=true walks sub-folders. Sorted by path.
func (c *Client) ListAttachments(relFolder string, recursive bool) ([]AttachmentEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	absFolder, err := c.safePath(relFolder)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absFolder)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("folder not found: %s", relFolder)
		}
		return nil, fmt.Errorf("stat folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", relFolder)
	}

	entries := []AttachmentEntry{}

	walk := func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if fi.IsDir() {
			if !recursive && path != absFolder {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(fi.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(c.vaultPath, path)
		if err != nil {
			return nil
		}
		entries = append(entries, AttachmentEntry{
			Path:  filepath.ToSlash(rel),
			Size:  fi.Size(),
			Mtime: fi.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	}

	if err := filepath.Walk(absFolder, walk); err != nil {
		return nil, fmt.Errorf("walk folder: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

// atomicWriteBytes writes raw bytes atomically: temp file in same dir, then rename.
func atomicWriteBytes(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

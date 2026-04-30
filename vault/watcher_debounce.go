package vault

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultDebounceDelay is how long we wait before resolving an fsnotify
// event. 100ms absorbs the macOS / iCloud atomic temp+rename pattern
// (Remove followed by Create within ~10ms) without noticeably delaying
// real deletes or legitimate writes.
const defaultDebounceDelay = 100 * time.Millisecond

// pendingEvent pairs a debounce timer with the path it resolves.
type pendingEvent struct {
	absPath string
	relPath string
	timer   *time.Timer
}

// scheduleEventResolution coalesces fsnotify events for the same path.
// A new event for an in-flight path resets the existing timer so we wait
// debounceDelay again before resolving. The resolver reads disk state
// and applies remove or reindex accordingly — race-free against atomic
// temp+rename, since the final state always reflects what is on disk at
// resolve time, no matter which Remove/Create sequence we received.
func (c *Client) scheduleEventResolution(absPath, relPath string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	if c.pendingEvents == nil {
		c.pendingEvents = make(map[string]*pendingEvent)
	}

	delay := c.debounceDelay
	if delay == 0 {
		delay = defaultDebounceDelay
	}

	if existing, ok := c.pendingEvents[absPath]; ok {
		existing.timer.Stop()
	}

	pe := &pendingEvent{absPath: absPath, relPath: relPath}
	pe.timer = time.AfterFunc(delay, func() {
		c.resolveFileState(absPath, relPath)
		c.pendingMu.Lock()
		// Match by pointer identity: a concurrent schedule may have
		// installed a newer pendingEvent between timer fire and lock
		// acquisition. Don't delete that newer entry.
		if cur, ok := c.pendingEvents[absPath]; ok && cur == pe {
			delete(c.pendingEvents, absPath)
		}
		c.pendingMu.Unlock()
	})
	c.pendingEvents[absPath] = pe
}

// resolveFileState reads the current disk state and applies remove or
// reindex. Called by the debounce timer. Race-free against fsnotify: the
// final state always reflects what is on disk at the time of the call,
// no matter which event sequence was delivered.
func (c *Client) resolveFileState(absPath, relPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		// File absent (real delete or unresolved rename). Remove for good.
		name := strings.TrimSuffix(filepath.ToSlash(relPath), ".md")
		lowerName := strings.ToLower(name)
		c.removePageWithLinks(lowerName)
		log.Printf("graphthulhu: removed %s from index\n", relPath)
		return
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		log.Printf("graphthulhu: failed to read %s: %v\n", absPath, err)
		return
	}
	c.indexFileWithLinks(filepath.ToSlash(relPath), string(content), info)
	log.Printf("graphthulhu: reindexed %s\n", relPath)
}

// indexFileWithLinks parses, indexes a file, AND rebuilds backlinks under
// a single lock. A reader cannot observe a state where the page exists in
// c.pages but backlinks don't yet reflect its new content. Replaces the
// previous indexFile + BuildBacklinks sequence (two separate lock
// acquisitions with an observable intermediate window).
func (c *Client) indexFileWithLinks(relPath, content string, info os.FileInfo) {
	page := c.parseFile(relPath, content, info)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyPageIndex(page)
	c.rebuildLinksLocked()
}

// removePageWithLinks removes a page AND rebuilds backlinks under a
// single lock, so the removal is atomic to readers.
func (c *Client) removePageWithLinks(lowerName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removePageFromIndexLocked(lowerName)
	c.rebuildLinksLocked()
}

// flushPendingEventsForTest synchronously resolves all pending events.
// Used only by tests to avoid waiting for the real debounceDelay.
// Production code has no reason to call this.
func (c *Client) flushPendingEventsForTest() {
	c.pendingMu.Lock()
	pending := make([]*pendingEvent, 0, len(c.pendingEvents))
	for _, pe := range c.pendingEvents {
		pe.timer.Stop()
		pending = append(pending, pe)
	}
	c.pendingEvents = nil
	c.pendingMu.Unlock()

	for _, pe := range pending {
		c.resolveFileState(pe.absPath, pe.relPath)
	}
}

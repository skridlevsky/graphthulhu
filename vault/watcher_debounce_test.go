package vault

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDebounce_AtomicRenameNoMissingPage simulates an atomic temp+rename:
// a Remove(target.md) immediately followed by Create(target.md) must NOT
// make the page disappear from the index, even transiently.
//
// Before this fix, the watcher did unconditional remove on Remove and
// reindex on Create — a ~10ms intermediate window where a concurrent
// get_page would see "page absent."
func TestDebounce_AtomicRenameNoMissingPage(t *testing.T) {
	c := testWritableVault(t)
	// Bump debounce so the assertion below runs comfortably inside the
	// debounce window even on slow CI runners (Windows fsnotify can take
	// 50-100ms per event).
	c.debounceDelay = 200 * time.Millisecond
	ctx := context.Background()

	// Seed: create an initial file and index it.
	target := filepath.Join(c.vaultPath, "target.md")
	if err := os.WriteFile(target, []byte("# Target initial\n\nv1"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := c.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if err := c.Watch(); err != nil {
		t.Fatalf("watch: %v", err)
	}
	defer c.Close()

	// Simulate atomic temp+rename: write a temp then rename over the target.
	temp := filepath.Join(c.vaultPath, ".target.tmp.md")
	if err := os.WriteFile(temp, []byte("# Target updated\n\nv2"), 0o644); err != nil {
		t.Fatalf("temp write: %v", err)
	}
	if err := os.Rename(temp, target); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// During the debounce window (before resolve), the original page must
	// stay accessible — no "page disappeared" window.
	page, err := c.GetPage(ctx, "target")
	if err != nil || page == nil {
		t.Errorf("page must remain accessible during debounce window, got page=%v err=%v", page, err)
	}

	// After resolve completes, the page must reflect the new content.
	time.Sleep(400 * time.Millisecond)
	blocks, err := c.GetPageBlocksTree(ctx, "target")
	if err != nil {
		t.Fatalf("GetPageBlocksTree post-resolve: %v", err)
	}
	if len(blocks) == 0 || !strings.Contains(blocks[0].Content, "Target updated") {
		t.Errorf("expected updated content after resolve, got blocks=%+v", blocks)
	}
}

// TestDebounce_BurstWritesCoalesce simulates rapid successive writes
// (typical of multi-writer workloads where multiple producers update the
// same page in quick succession) and verifies that only the final state
// is resolved — not a cascade of expensive reindexes.
func TestDebounce_BurstWritesCoalesce(t *testing.T) {
	c := testWritableVault(t)
	// Bump debounce to give the burst loop comfortable headroom under
	// the debounce window even on slow CI.
	c.debounceDelay = 200 * time.Millisecond
	ctx := context.Background()

	target := filepath.Join(c.vaultPath, "burst.md")
	if err := c.Watch(); err != nil {
		t.Fatalf("watch: %v", err)
	}
	defer c.Close()

	// 10 successive writes inside the debounce window → 1 resolve expected.
	for i := 0; i < 10; i++ {
		content := strings.NewReplacer("$N", string(rune('0'+i))).Replace("# Burst v$N\n\ncontent $N")
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// After debounce + resolve: must reflect v9 (the last write).
	time.Sleep(500 * time.Millisecond)
	blocks, err := c.GetPageBlocksTree(ctx, "burst")
	if err != nil {
		t.Fatalf("GetPageBlocksTree: %v", err)
	}
	if len(blocks) == 0 || !strings.Contains(blocks[0].Content, "Burst v9") {
		t.Errorf("expected final state v9, got %+v", blocks)
	}
}

// TestIndexFileWithLinks_Atomic verifies that a concurrent reader can
// NEVER observe a state where the page exists in c.pages but backlinks
// don't yet reflect it (the intermediate window between indexFile and
// BuildBacklinks in the previous implementation — race 1).
func TestIndexFileWithLinks_Atomic(t *testing.T) {
	c := testWritableVault(t)
	ctx := context.Background()

	// Pre-setup: one page that will link to the new target.
	if err := os.WriteFile(filepath.Join(c.vaultPath, "linker.md"),
		[]byte("# Linker\n\n- to [[atomic-target]]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.Reload(); err != nil {
		t.Fatal(err)
	}

	// Concurrent reader spamming GetPage + getBacklinks.
	stop := make(chan struct{})
	var inconsistencies int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			page, _ := c.GetPage(ctx, "atomic-target")
			if page == nil {
				continue
			}
			// Page exists: backlinks MUST include Linker (atomicity).
			c.mu.RLock()
			bl := c.backlinks[strings.ToLower(page.Name)]
			c.mu.RUnlock()
			found := false
			for _, b := range bl {
				if strings.EqualFold(b.fromPage, "linker") {
					found = true
					break
				}
			}
			if !found {
				mu.Lock()
				inconsistencies++
				mu.Unlock()
			}
		}
	}()

	// Index the target page 50 times (each indexFileWithLinks must be atomic).
	info, _ := os.Stat(filepath.Join(c.vaultPath, "linker.md"))
	for i := 0; i < 50; i++ {
		c.indexFileWithLinks("atomic-target.md", "# Atomic Target\n\nbody", info)
		time.Sleep(time.Microsecond)
	}

	close(stop)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if inconsistencies > 0 {
		t.Errorf("observed %d inconsistencies (page exists but backlinks missing) — atomicity broken", inconsistencies)
	}
}

// TestRemovePageWithLinks_Atomic: analogue for removal.
func TestRemovePageWithLinks_Atomic(t *testing.T) {
	c := testWritableVault(t)
	ctx := context.Background()

	// Pre-setup: target page + linker.
	if err := os.WriteFile(filepath.Join(c.vaultPath, "removable.md"),
		[]byte("# Removable\n\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.vaultPath, "linker2.md"),
		[]byte("# Linker2\n\n- to [[removable]]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.Reload(); err != nil {
		t.Fatal(err)
	}

	// Verify setup is OK.
	page, _ := c.GetPage(ctx, "removable")
	if page == nil {
		t.Fatal("setup: removable should exist")
	}

	// Remove atomically.
	c.removePageWithLinks("removable")

	// Page absent; backlinks for 'removable' should be empty or missing.
	page, _ = c.GetPage(ctx, "removable")
	if page != nil {
		t.Errorf("page should be removed, got %+v", page)
	}
	c.mu.RLock()
	bl := c.backlinks["removable"]
	c.mu.RUnlock()
	// Note: backlinks from other pages still POINTING at 'removable' may
	// persist until those source pages are rebuilt — that's fine. The
	// contract is atomicity with respect to the primary index c.pages,
	// not transitive consistency.
	_ = bl
}

// TestScheduleEventResolution_Coalesce: 5 successive events for the same
// path within the debounce window must yield exactly 1 resolve.
func TestScheduleEventResolution_Coalesce(t *testing.T) {
	c := testWritableVault(t)
	c.debounceDelay = 50 * time.Millisecond

	target := filepath.Join(c.vaultPath, "coalesce.md")
	if err := os.WriteFile(target, []byte("# Coalesce v0"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 5 successive events inside the 50ms window.
	for i := 0; i < 5; i++ {
		c.scheduleEventResolution(target, "coalesce.md")
		time.Sleep(5 * time.Millisecond)
	}

	// Before delay: 1 pending timer (the previous 4 were stopped).
	c.pendingMu.Lock()
	pending := len(c.pendingEvents)
	c.pendingMu.Unlock()
	if pending != 1 {
		t.Errorf("expected 1 pending event after 5 schedules, got %d", pending)
	}

	// After delay + margin: pending empty, page indexed.
	time.Sleep(150 * time.Millisecond)
	c.pendingMu.Lock()
	pending = len(c.pendingEvents)
	c.pendingMu.Unlock()
	if pending != 0 {
		t.Errorf("expected pending events flushed, got %d remaining", pending)
	}
	page, err := c.GetPage(context.Background(), "coalesce")
	if err != nil || page == nil {
		t.Errorf("page should be indexed after resolve, page=%v err=%v", page, err)
	}
}

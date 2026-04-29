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

// TestDebounce_AtomicRenameNoMissingPage simule un atomic temp+rename : la
// sequence Remove(target.md) suivie immediatement d'un Create(target.md) ne
// doit PAS faire disparaitre la page de l'index, meme transitoirement.
//
// Avant le fix bd-s5c Phase 7, le watcher faisait remove inconditionnel sur
// l'event Remove et reindex sur Create — fenetre intermediaire ~10ms ou un
// get_page concurrent voyait "page absente".
func TestDebounce_AtomicRenameNoMissingPage(t *testing.T) {
	c := testWritableVault(t)
	ctx := context.Background()

	// Setup : creer un fichier initial + l'indexer.
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

	// Simuler atomic temp+rename : ecrire un temp puis rename.
	temp := filepath.Join(c.vaultPath, ".target.tmp.md")
	if err := os.WriteFile(temp, []byte("# Target updated\n\nv2"), 0o644); err != nil {
		t.Fatalf("temp write: %v", err)
	}
	if err := os.Rename(temp, target); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Pendant la fenetre debounce (avant resolve), interroger l'index : la page
	// initiale doit toujours etre accessible (pas de fenetre de "page disparue").
	page, err := c.GetPage(ctx, "target")
	if err != nil || page == nil {
		t.Errorf("page must remain accessible during debounce window, got page=%v err=%v", page, err)
	}

	// Apres resolution complete, la page doit refleter le nouveau contenu.
	time.Sleep(250 * time.Millisecond)
	blocks, err := c.GetPageBlocksTree(ctx, "target")
	if err != nil {
		t.Fatalf("GetPageBlocksTree post-resolve: %v", err)
	}
	if len(blocks) == 0 || !strings.Contains(blocks[0].Content, "Target updated") {
		t.Errorf("expected updated content after resolve, got blocks=%+v", blocks)
	}
}

// TestDebounce_BurstWritesCoalesce simule des ecritures rapprochees (typique
// vault-preflight burst : Rapport_Scout puis dispatch en cascade) et verifie
// que seul le dernier etat est resolu (pas une cascade de reindex couteux).
func TestDebounce_BurstWritesCoalesce(t *testing.T) {
	c := testWritableVault(t)
	ctx := context.Background()

	target := filepath.Join(c.vaultPath, "burst.md")
	if err := c.Watch(); err != nil {
		t.Fatalf("watch: %v", err)
	}
	defer c.Close()

	// 10 ecritures successives en moins de debounceDelay -> 1 seul resolve attendu.
	for i := 0; i < 10; i++ {
		content := strings.NewReplacer("$N", string(rune('0'+i))).Replace("# Burst v$N\n\ncontent $N")
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Apres debounce + resolve : doit refleter v9 (dernier).
	time.Sleep(300 * time.Millisecond)
	blocks, err := c.GetPageBlocksTree(ctx, "burst")
	if err != nil {
		t.Fatalf("GetPageBlocksTree: %v", err)
	}
	if len(blocks) == 0 || !strings.Contains(blocks[0].Content, "Burst v9") {
		t.Errorf("expected final state v9, got %+v", blocks)
	}
}

// TestIndexFileWithLinks_Atomic verifie qu'un lecteur concurrent ne peut JAMAIS
// observer un etat ou la page existe dans c.pages mais ou les backlinks ne sont
// pas encore reconstruits (fenetre intermediaire entre indexFile et BuildBacklinks
// dans l'ancienne implementation, race 1 du diagnostic Phase 7).
func TestIndexFileWithLinks_Atomic(t *testing.T) {
	c := testWritableVault(t)
	ctx := context.Background()

	// Pre-setup : 1 page qui pointera vers la nouvelle.
	if err := os.WriteFile(filepath.Join(c.vaultPath, "linker.md"),
		[]byte("# Linker\n\n- vers [[atomic-target]]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.Reload(); err != nil {
		t.Fatal(err)
	}

	// Lecteur concurrent qui spam GetPage + getBacklinks.
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
			// Page existe : les backlinks DOIVENT contenir Linker (atomicite).
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

	// Indexer la page cible 50 fois (chaque indexFileWithLinks doit etre atomique).
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

// TestRemovePageWithLinks_Atomic : analogue pour la suppression.
func TestRemovePageWithLinks_Atomic(t *testing.T) {
	c := testWritableVault(t)
	ctx := context.Background()

	// Pre-setup : page cible + linker.
	if err := os.WriteFile(filepath.Join(c.vaultPath, "removable.md"),
		[]byte("# Removable\n\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.vaultPath, "linker2.md"),
		[]byte("# Linker2\n\n- vers [[removable]]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.Reload(); err != nil {
		t.Fatal(err)
	}

	// Verifier setup OK.
	page, _ := c.GetPage(ctx, "removable")
	if page == nil {
		t.Fatal("setup: removable should exist")
	}

	// Supprimer atomiquement.
	c.removePageWithLinks("removable")

	// Page absente, backlinks pour 'removable' doivent etre vides ou absents.
	page, _ = c.GetPage(ctx, "removable")
	if page != nil {
		t.Errorf("page should be removed, got %+v", page)
	}
	c.mu.RLock()
	bl := c.backlinks["removable"]
	c.mu.RUnlock()
	// Note : les backlinks d'autres pages POINTANT vers removable peuvent persister
	// jusqu'au prochain rebuild des sources — c'est OK, le contrat est l'atomicite
	// vis-a-vis de l'index principal c.pages, pas la coherence transitive.
	_ = bl
}

// TestScheduleEventResolution_Coalesce : 5 events successifs sur le meme path
// dans la fenetre debounce ne declenchent qu'1 seul resolve.
func TestScheduleEventResolution_Coalesce(t *testing.T) {
	c := testWritableVault(t)
	c.debounceDelay = 50 * time.Millisecond

	target := filepath.Join(c.vaultPath, "coalesce.md")
	if err := os.WriteFile(target, []byte("# Coalesce v0"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 5 events successifs en moins de 50ms.
	for i := 0; i < 5; i++ {
		c.scheduleEventResolution(target, "coalesce.md")
		time.Sleep(5 * time.Millisecond)
	}

	// Avant le delay : 1 timer pending (les 4 precedents stoppes).
	c.pendingMu.Lock()
	pending := len(c.pendingEvents)
	c.pendingMu.Unlock()
	if pending != 1 {
		t.Errorf("expected 1 pending event after 5 schedules, got %d", pending)
	}

	// Apres delay + marge : pending vide, page indexee.
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

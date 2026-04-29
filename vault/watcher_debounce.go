package vault

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultDebounceDelay : delai d'attente avant de resoudre un event fsnotify.
// 100ms : suffisant pour absorber un atomic temp+rename macOS / iCloud (qui
// genere des sequences Remove puis Create rapprochees, fenetre typique ~10ms),
// court enough pour ne pas faire trainer les vrais deletes ni les writes
// legitimes. Voir bd-s5c Phase 7.
const defaultDebounceDelay = 100 * time.Millisecond

// pendingEvent encapsule un timer + son path pour le debounce d'events fsnotify.
type pendingEvent struct {
	absPath string
	relPath string
	timer   *time.Timer
}

// scheduleEventResolution coalesce les events fsnotify pour un meme path.
// Si un event arrive pour un path deja en attente, le timer existant est
// reset et on attend a nouveau debounceDelay avant de resoudre. La resolution
// finale lit l'etat disque et applique remove OU reindex selon. Race-free
// vis-a-vis des atomic temp+rename : peu importe la sequence Remove+Create,
// le resultat reflete toujours ce qui est sur disque au moment du resolve.
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
		delete(c.pendingEvents, absPath)
		c.pendingMu.Unlock()
	})
	c.pendingEvents[absPath] = pe
}

// resolveFileState lit l'etat disque actuel et applique remove ou reindex.
// Appele par le timer de debounce. Race-free vis-a-vis de fsnotify : peu
// importe la sequence d'events recue, le resultat final reflete toujours ce
// qui est sur disque au moment de l'appel.
func (c *Client) resolveFileState(absPath, relPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		// Fichier absent (delete reel ou rename non resolu). Remove definitif.
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

// indexFileWithLinks parses + indexe un fichier ET reconstruit les links sous
// un seul lock. Race-free : un lecteur ne peut pas voir un etat ou la page
// existe dans c.pages mais ou les backlinks ne refletent pas encore le nouveau
// contenu. Remplace la sequence indexFile + BuildBacklinks (2 prises de lock
// distinctes avec fenetre intermediaire visible aux lecteurs).
func (c *Client) indexFileWithLinks(relPath, content string, info os.FileInfo) {
	page := c.parseFile(relPath, content, info)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyPageIndex(page)
	c.rebuildLinksLocked()
}

// removePageWithLinks retire une page ET reconstruit les links sous un seul
// lock. Pendant la construction des nouveaux backlinks, l'ensemble est
// atomique aux lecteurs.
func (c *Client) removePageWithLinks(lowerName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removePageFromIndexLocked(lowerName)
	c.rebuildLinksLocked()
}

// flushPendingEventsForTest force la resolution synchrone de tous les events
// en attente. Utilise uniquement par les tests pour eviter d'attendre
// debounceDelay reel. Production code n'a aucune raison d'appeler ca.
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

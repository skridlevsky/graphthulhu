package graph

import (
	"sort"
	"testing"

	"github.com/skridlevsky/graphthulhu/types"
)

// --- Helpers ---

// newGraph builds a Graph from a simple adjacency list.
// edges: map of "source" → ["target1", "target2"]
// All pages are non-journal unless listed in journals.
func newGraph(edges map[string][]string, journals ...string) *Graph {
	journalSet := make(map[string]bool)
	for _, j := range journals {
		journalSet[j] = true
	}

	g := &Graph{
		Forward:     make(map[string]map[string]bool),
		Backward:    make(map[string]map[string]bool),
		Pages:       make(map[string]types.PageEntity),
		BlockCounts: make(map[string]int),
	}

	// Collect all page names
	allPages := make(map[string]bool)
	for src, targets := range edges {
		allPages[src] = true
		for _, tgt := range targets {
			allPages[tgt] = true
		}
	}

	// Create pages
	for name := range allPages {
		g.Pages[name] = types.PageEntity{
			Name:         name,
			OriginalName: name,
			Journal:      journalSet[name],
		}
		g.Forward[name] = make(map[string]bool)
		g.BlockCounts[name] = 1 // default 1 block per page
	}

	// Create edges
	for src, targets := range edges {
		for _, tgt := range targets {
			g.Forward[src][tgt] = true
			if g.Backward[tgt] == nil {
				g.Backward[tgt] = make(map[string]bool)
			}
			g.Backward[tgt][src] = true
		}
	}

	return g
}

// --- Degree ---

func TestOutDegree(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b", "c"},
		"b": {"c"},
		"c": {},
	})
	if got := g.OutDegree("a"); got != 2 {
		t.Errorf("OutDegree(a) = %d, want 2", got)
	}
	if got := g.OutDegree("c"); got != 0 {
		t.Errorf("OutDegree(c) = %d, want 0", got)
	}
}

func TestInDegree(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b", "c"},
		"b": {"c"},
	})
	if got := g.InDegree("c"); got != 2 {
		t.Errorf("InDegree(c) = %d, want 2", got)
	}
	if got := g.InDegree("a"); got != 0 {
		t.Errorf("InDegree(a) = %d, want 0", got)
	}
}

func TestTotalDegree(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {"a"},
	})
	// a: out=1 (→b), in=1 (←b) = 2
	if got := g.TotalDegree("a"); got != 2 {
		t.Errorf("TotalDegree(a) = %d, want 2", got)
	}
}

func TestDegree_NonExistentPage(t *testing.T) {
	g := newGraph(map[string][]string{"a": {"b"}})
	if got := g.OutDegree("z"); got != 0 {
		t.Errorf("OutDegree(z) = %d, want 0", got)
	}
	if got := g.InDegree("z"); got != 0 {
		t.Errorf("InDegree(z) = %d, want 0", got)
	}
}

// --- OriginalName ---

func TestOriginalName_Exists(t *testing.T) {
	g := newGraph(map[string][]string{"a": {}})
	if got := g.OriginalName("a"); got != "a" {
		t.Errorf("OriginalName(a) = %q, want 'a'", got)
	}
}

func TestOriginalName_Missing(t *testing.T) {
	g := newGraph(map[string][]string{})
	if got := g.OriginalName("missing"); got != "missing" {
		t.Errorf("OriginalName(missing) = %q, want 'missing'", got)
	}
}

// --- Overview ---

func TestOverview_Empty(t *testing.T) {
	g := newGraph(map[string][]string{})
	stats := g.Overview()
	if stats.TotalPages != 0 {
		t.Errorf("TotalPages = %d, want 0", stats.TotalPages)
	}
	if stats.TotalLinks != 0 {
		t.Errorf("TotalLinks = %d, want 0", stats.TotalLinks)
	}
}

func TestOverview_Basic(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b", "c"},
		"b": {"c"},
		"c": {},
	})
	stats := g.Overview()

	if stats.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", stats.TotalPages)
	}
	// Total links = out-degree sum = 2 + 1 + 0 = 3
	if stats.TotalLinks != 3 {
		t.Errorf("TotalLinks = %d, want 3", stats.TotalLinks)
	}
}

func TestOverview_CountsJournals(t *testing.T) {
	g := newGraph(map[string][]string{
		"page":    {},
		"journal": {},
	}, "journal")
	stats := g.Overview()
	if stats.JournalPages != 1 {
		t.Errorf("JournalPages = %d, want 1", stats.JournalPages)
	}
}

func TestOverview_CountsOrphans(t *testing.T) {
	g := newGraph(map[string][]string{
		"connected": {"other"},
		"other":     {},
		"orphan":    {},
	})
	stats := g.Overview()
	if stats.OrphanPages != 1 {
		t.Errorf("OrphanPages = %d, want 1", stats.OrphanPages)
	}
}

func TestOverview_Namespaces(t *testing.T) {
	g := &Graph{
		Forward:     make(map[string]map[string]bool),
		Backward:    make(map[string]map[string]bool),
		Pages:       make(map[string]types.PageEntity),
		BlockCounts: make(map[string]int),
	}
	g.Pages["graphthulhu/vision"] = types.PageEntity{Name: "graphthulhu/vision", OriginalName: "graphthulhu/vision"}
	g.Pages["graphthulhu/decisions"] = types.PageEntity{Name: "graphthulhu/decisions", OriginalName: "graphthulhu/decisions"}
	g.Pages["openchaos/token"] = types.PageEntity{Name: "openchaos/token", OriginalName: "openchaos/token"}
	g.Forward["graphthulhu/vision"] = make(map[string]bool)
	g.Forward["graphthulhu/decisions"] = make(map[string]bool)
	g.Forward["openchaos/token"] = make(map[string]bool)

	stats := g.Overview()
	if stats.Namespaces["graphthulhu"] != 2 {
		t.Errorf("Namespaces[graphthulhu] = %d, want 2", stats.Namespaces["graphthulhu"])
	}
	if stats.Namespaces["openchaos"] != 1 {
		t.Errorf("Namespaces[openchaos] = %d, want 1", stats.Namespaces["openchaos"])
	}
}

func TestOverview_MostConnectedOrder(t *testing.T) {
	g := newGraph(map[string][]string{
		"hub":  {"a", "b", "c"},
		"mid":  {"a"},
		"a":    {},
		"b":    {},
		"c":    {},
	})
	stats := g.Overview()
	if len(stats.MostConnected) == 0 {
		t.Fatal("MostConnected is empty")
	}
	// hub has TotalDegree=3 (outDeg=3), should be first
	if stats.MostConnected[0].Name != "hub" {
		t.Errorf("MostConnected[0] = %q, want 'hub'", stats.MostConnected[0].Name)
	}
}

func TestOverview_MostLinkedToOrder(t *testing.T) {
	g := newGraph(map[string][]string{
		"hub":  {"a", "b", "c"},
		"mid":  {"a"},
		"a":    {},
		"b":    {},
		"c":    {},
	})
	stats := g.Overview()
	if len(stats.MostLinkedTo) == 0 {
		t.Fatal("MostLinkedTo is empty")
	}
	// "a" has InLinks=2 (from hub + mid), should be first
	if stats.MostLinkedTo[0].Name != "a" {
		t.Errorf("MostLinkedTo[0] = %q, want 'a'", stats.MostLinkedTo[0].Name)
	}
}

// --- FindConnections ---

func TestFindConnections_DirectLink(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {},
	})
	r := g.FindConnections("a", "b", 5)
	if !r.DirectlyLinked {
		t.Error("DirectlyLinked = false, want true")
	}
}

func TestFindConnections_NoDirectLink(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {},
	})
	r := g.FindConnections("a", "c", 5)
	if r.DirectlyLinked {
		t.Error("DirectlyLinked = true, want false")
	}
}

func TestFindConnections_FindsPath(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {},
	})
	r := g.FindConnections("a", "c", 5)
	if len(r.Paths) == 0 {
		t.Fatal("Paths is empty, want at least 1 path")
	}
	// Path should be [a, b, c]
	if len(r.Paths[0]) != 3 {
		t.Errorf("Path length = %d, want 3", len(r.Paths[0]))
	}
}

func TestFindConnections_NoPath(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {},
		"b": {},
	})
	r := g.FindConnections("a", "b", 5)
	if len(r.Paths) != 0 {
		t.Errorf("Paths = %v, want empty (no connection)", r.Paths)
	}
}

func TestFindConnections_SharedConnections(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"shared"},
		"b": {"shared"},
		"shared": {},
	})
	r := g.FindConnections("a", "b", 5)
	if len(r.SharedConnections) != 1 || r.SharedConnections[0] != "shared" {
		t.Errorf("SharedConnections = %v, want [shared]", r.SharedConnections)
	}
}

func TestFindConnections_MaxDepthDefault(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {},
	})
	// maxDepth <= 0 should default to 5
	r := g.FindConnections("a", "b", 0)
	if !r.DirectlyLinked {
		t.Error("DirectlyLinked = false with maxDepth=0 (should default to 5)")
	}
}

// --- KnowledgeGaps ---

func TestKnowledgeGaps_Orphans(t *testing.T) {
	g := newGraph(map[string][]string{
		"connected": {"other"},
		"other":     {},
		"orphan":    {},
	})
	gaps := g.KnowledgeGaps()
	if len(gaps.OrphanPages) != 1 || gaps.OrphanPages[0] != "orphan" {
		t.Errorf("OrphanPages = %v, want [orphan]", gaps.OrphanPages)
	}
}

func TestKnowledgeGaps_DeadEnds(t *testing.T) {
	g := newGraph(map[string][]string{
		"a":        {"deadend"},
		"deadend":  {},
	})
	gaps := g.KnowledgeGaps()
	if len(gaps.DeadEndPages) != 1 || gaps.DeadEndPages[0] != "deadend" {
		t.Errorf("DeadEndPages = %v, want [deadend]", gaps.DeadEndPages)
	}
}

func TestKnowledgeGaps_WeaklyLinked(t *testing.T) {
	g := newGraph(map[string][]string{
		"hub":  {"a", "b", "c", "d"},
		"a":    {"hub"},
		"b":    {"hub"},
		"c":    {"hub"},
		"d":    {"hub"},
		"weak": {"hub"},
	})
	gaps := g.KnowledgeGaps()
	// "weak" has outDeg=1, inDeg=0, total=1 → weakly linked
	found := false
	for _, w := range gaps.WeaklyLinked {
		if w.Name == "weak" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("WeaklyLinked = %v, expected 'weak' to be present", gaps.WeaklyLinked)
	}
}

func TestKnowledgeGaps_SkipsJournals(t *testing.T) {
	g := newGraph(map[string][]string{
		"journal": {},
		"page":    {},
	}, "journal")
	gaps := g.KnowledgeGaps()
	// journal should NOT appear in orphans (skipped)
	for _, o := range gaps.OrphanPages {
		if o == "journal" {
			t.Error("journal page should be excluded from gap analysis")
		}
	}
}

func TestKnowledgeGaps_SortedAlphabetically(t *testing.T) {
	g := newGraph(map[string][]string{
		"z-orphan": {},
		"a-orphan": {},
		"m-orphan": {},
	})
	gaps := g.KnowledgeGaps()
	if len(gaps.OrphanPages) != 3 {
		t.Fatalf("OrphanPages = %v, want 3", gaps.OrphanPages)
	}
	if !sort.StringsAreSorted(gaps.OrphanPages) {
		t.Errorf("OrphanPages not sorted: %v", gaps.OrphanPages)
	}
}

// --- TopicClusters ---

func TestTopicClusters_SingleCluster(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {},
	})
	clusters := g.TopicClusters()
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(clusters))
	}
	if clusters[0].Size != 3 {
		t.Errorf("cluster size = %d, want 3", clusters[0].Size)
	}
}

func TestTopicClusters_TwoClusters(t *testing.T) {
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {},
		"x": {"y"},
		"y": {},
	})
	clusters := g.TopicClusters()
	if len(clusters) != 2 {
		t.Fatalf("clusters = %d, want 2", len(clusters))
	}
	// Both clusters should have size 2
	for _, c := range clusters {
		if c.Size != 2 {
			t.Errorf("cluster %d size = %d, want 2", c.ID, c.Size)
		}
	}
}

func TestTopicClusters_SkipsSingletons(t *testing.T) {
	g := newGraph(map[string][]string{
		"a":         {"b"},
		"b":         {},
		"singleton": {},
	})
	clusters := g.TopicClusters()
	for _, c := range clusters {
		for _, p := range c.Pages {
			if p == "singleton" {
				t.Error("singleton should not appear in clusters")
			}
		}
	}
}

func TestTopicClusters_SkipsJournals(t *testing.T) {
	g := newGraph(map[string][]string{
		"a":       {"b"},
		"b":       {},
		"journal": {},
	}, "journal")
	clusters := g.TopicClusters()
	for _, c := range clusters {
		for _, p := range c.Pages {
			if p == "journal" {
				t.Error("journal should not appear in clusters")
			}
		}
	}
}

func TestTopicClusters_HubIsHighestDegree(t *testing.T) {
	g := newGraph(map[string][]string{
		"hub":  {"a", "b", "c"},
		"a":    {},
		"b":    {},
		"c":    {},
	})
	clusters := g.TopicClusters()
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(clusters))
	}
	if clusters[0].Hub != "hub" {
		t.Errorf("Hub = %q, want 'hub'", clusters[0].Hub)
	}
}

func TestTopicClusters_SortedBySizeDesc(t *testing.T) {
	g := newGraph(map[string][]string{
		// Small cluster
		"x": {"y"},
		"y": {},
		// Big cluster
		"a": {"b", "c", "d"},
		"b": {},
		"c": {},
		"d": {},
	})
	clusters := g.TopicClusters()
	if len(clusters) < 2 {
		t.Fatalf("clusters = %d, want >= 2", len(clusters))
	}
	if clusters[0].Size < clusters[1].Size {
		t.Errorf("clusters not sorted by size desc: %d < %d", clusters[0].Size, clusters[1].Size)
	}
}

func TestTopicClusters_PagesAreSorted(t *testing.T) {
	g := newGraph(map[string][]string{
		"c": {"a"},
		"a": {"b"},
		"b": {},
	})
	clusters := g.TopicClusters()
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1", len(clusters))
	}
	if !sort.StringsAreSorted(clusters[0].Pages) {
		t.Errorf("Pages not sorted: %v", clusters[0].Pages)
	}
}

func TestTopicClusters_UndirectedTraversal(t *testing.T) {
	// a→b only (no b→a), but they should be in same cluster (undirected BFS)
	g := newGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {},
	})
	clusters := g.TopicClusters()
	if len(clusters) != 1 {
		t.Fatalf("clusters = %d, want 1 (undirected traversal)", len(clusters))
	}
	if clusters[0].Size != 3 {
		t.Errorf("cluster size = %d, want 3", clusters[0].Size)
	}
}

// --- countBlocksRecursive ---

func TestCountBlocksRecursive_Flat(t *testing.T) {
	blocks := []types.BlockEntity{
		{Content: "a"},
		{Content: "b"},
		{Content: "c"},
	}
	if got := countBlocksRecursive(blocks); got != 3 {
		t.Errorf("countBlocksRecursive = %d, want 3", got)
	}
}

func TestCountBlocksRecursive_Nested(t *testing.T) {
	blocks := []types.BlockEntity{
		{
			Content: "parent",
			Children: []types.BlockEntity{
				{Content: "child1"},
				{
					Content: "child2",
					Children: []types.BlockEntity{
						{Content: "grandchild"},
					},
				},
			},
		},
	}
	// 1 parent + 2 children + 1 grandchild = 4
	if got := countBlocksRecursive(blocks); got != 4 {
		t.Errorf("countBlocksRecursive = %d, want 4", got)
	}
}

func TestCountBlocksRecursive_Empty(t *testing.T) {
	if got := countBlocksRecursive(nil); got != 0 {
		t.Errorf("countBlocksRecursive(nil) = %d, want 0", got)
	}
}

// --- extractLinksRecursive ---

func TestExtractLinksRecursive_Simple(t *testing.T) {
	g := &Graph{
		Forward:  map[string]map[string]bool{"source": make(map[string]bool)},
		Backward: make(map[string]map[string]bool),
	}
	blocks := []types.BlockEntity{
		{Content: "Link to [[Target]]"},
	}
	extractLinksRecursive(blocks, "source", g)
	if !g.Forward["source"]["Target"] {
		t.Errorf("Forward[source] = %v, want Target", g.Forward["source"])
	}
	if !g.Backward["target"]["source"] {
		t.Errorf("Backward[target] = %v, want source", g.Backward["target"])
	}
}

func TestExtractLinksRecursive_Nested(t *testing.T) {
	g := &Graph{
		Forward:  map[string]map[string]bool{"source": make(map[string]bool)},
		Backward: make(map[string]map[string]bool),
	}
	blocks := []types.BlockEntity{
		{
			Content: "[[Top]]",
			Children: []types.BlockEntity{
				{Content: "[[Nested]]"},
			},
		},
	}
	extractLinksRecursive(blocks, "source", g)
	if !g.Forward["source"]["Top"] {
		t.Error("missing forward link to Top")
	}
	if !g.Forward["source"]["Nested"] {
		t.Error("missing forward link to Nested")
	}
}

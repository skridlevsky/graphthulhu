package graph

import (
	"sort"
	"strings"
)

// OverviewStats contains global graph statistics.
type OverviewStats struct {
	TotalPages      int              `json:"totalPages"`
	TotalBlocks     int              `json:"totalBlocks"`
	TotalLinks      int              `json:"totalLinks"`
	JournalPages    int              `json:"journalPages"`
	OrphanPages     int              `json:"orphanPages"`
	MostConnected   []PageStat       `json:"mostConnected"`
	MostLinkedTo    []PageStat       `json:"mostLinkedTo"`
	Namespaces      map[string]int   `json:"namespaces"`
}

// PageStat is a page with its connectivity score.
type PageStat struct {
	Name        string `json:"name"`
	OutLinks    int    `json:"outLinks"`
	InLinks     int    `json:"inLinks"`
	TotalDegree int    `json:"totalDegree"`
	BlockCount  int    `json:"blockCount"`
}

// ConnectionResult describes how two pages are connected.
type ConnectionResult struct {
	From              string     `json:"from"`
	To                string     `json:"to"`
	DirectlyLinked    bool       `json:"directlyLinked"`
	Paths             [][]string `json:"paths"`
	SharedConnections []string   `json:"sharedConnections"`
}

// GapInfo describes a knowledge gap or sparse area.
type GapInfo struct {
	OrphanPages   []string   `json:"orphanPages"`
	DeadEndPages  []string   `json:"deadEndPages"`
	WeaklyLinked  []PageStat `json:"weaklyLinked"`
	SingletonTags []string   `json:"singletonTags,omitempty"`
}

// Cluster is a group of densely connected pages.
type Cluster struct {
	ID    int      `json:"id"`
	Size  int      `json:"size"`
	Pages []string `json:"pages"`
	Hub   string   `json:"hub"`
}

// Overview computes global graph statistics.
func (g *Graph) Overview() OverviewStats {
	stats := OverviewStats{
		TotalPages: len(g.Pages),
		Namespaces: make(map[string]int),
	}

	var totalLinks int
	var totalBlocks int
	var pageStats []PageStat

	for key, page := range g.Pages {
		if page.Journal {
			stats.JournalPages++
		}

		outDeg := g.OutDegree(key)
		inDeg := g.InDegree(key)
		totalLinks += outDeg
		totalBlocks += g.BlockCounts[key]

		if outDeg == 0 && inDeg == 0 {
			stats.OrphanPages++
		}

		pageStats = append(pageStats, PageStat{
			Name:        g.OriginalName(key),
			OutLinks:    outDeg,
			InLinks:     inDeg,
			TotalDegree: outDeg + inDeg,
			BlockCount:  g.BlockCounts[key],
		})

		// Count namespaces
		if strings.Contains(page.Name, "/") {
			ns := strings.SplitN(page.Name, "/", 2)[0]
			stats.Namespaces[ns]++
		}
	}

	stats.TotalLinks = totalLinks
	stats.TotalBlocks = totalBlocks

	// Top 10 most connected
	sort.Slice(pageStats, func(i, j int) bool {
		return pageStats[i].TotalDegree > pageStats[j].TotalDegree
	})
	limit := 10
	if len(pageStats) < limit {
		limit = len(pageStats)
	}
	stats.MostConnected = make([]PageStat, limit)
	copy(stats.MostConnected, pageStats[:limit])

	// Top 10 most linked to (by in-degree)
	sort.Slice(pageStats, func(i, j int) bool {
		return pageStats[i].InLinks > pageStats[j].InLinks
	})
	if len(pageStats) < 10 {
		limit = len(pageStats)
	} else {
		limit = 10
	}
	stats.MostLinkedTo = pageStats[:limit]

	return stats
}

// FindConnections finds how two pages are connected.
func (g *Graph) FindConnections(from, to string, maxDepth int) ConnectionResult {
	fromKey := strings.ToLower(from)
	toKey := strings.ToLower(to)

	if maxDepth <= 0 {
		maxDepth = 5
	}

	result := ConnectionResult{
		From: g.OriginalName(fromKey),
		To:   g.OriginalName(toKey),
	}

	// Check direct link
	if g.Forward[fromKey][to] || hasKeyInsensitive(g.Forward[fromKey], toKey) {
		result.DirectlyLinked = true
	}

	// BFS for paths
	result.Paths = g.bfsPaths(fromKey, toKey, maxDepth)

	// Find shared connections (pages both link to, or that link to both)
	fromNeighbors := g.allNeighbors(fromKey)
	toNeighbors := g.allNeighbors(toKey)

	for n := range fromNeighbors {
		if toNeighbors[n] && n != fromKey && n != toKey {
			result.SharedConnections = append(result.SharedConnections, g.OriginalName(n))
		}
	}
	sort.Strings(result.SharedConnections)

	return result
}

// KnowledgeGaps finds sparse areas in the graph.
func (g *Graph) KnowledgeGaps() GapInfo {
	var gaps GapInfo
	var weakStats []PageStat

	for key := range g.Pages {
		page := g.Pages[key]
		if page.Journal {
			continue // skip journal pages from gap analysis
		}

		outDeg := g.OutDegree(key)
		inDeg := g.InDegree(key)
		name := g.OriginalName(key)

		if outDeg == 0 && inDeg == 0 {
			gaps.OrphanPages = append(gaps.OrphanPages, name)
		} else if outDeg == 0 && inDeg > 0 {
			gaps.DeadEndPages = append(gaps.DeadEndPages, name)
		} else if outDeg+inDeg <= 2 {
			weakStats = append(weakStats, PageStat{
				Name:        name,
				OutLinks:    outDeg,
				InLinks:     inDeg,
				TotalDegree: outDeg + inDeg,
				BlockCount:  g.BlockCounts[key],
			})
		}
	}

	sort.Strings(gaps.OrphanPages)
	sort.Strings(gaps.DeadEndPages)
	sort.Slice(weakStats, func(i, j int) bool {
		return weakStats[i].TotalDegree < weakStats[j].TotalDegree
	})

	limit := 20
	if len(weakStats) < limit {
		limit = len(weakStats)
	}
	gaps.WeaklyLinked = weakStats[:limit]

	return gaps
}

// TopicClusters finds connected components in the undirected link graph.
func (g *Graph) TopicClusters() []Cluster {
	visited := make(map[string]bool)
	var clusters []Cluster
	clusterID := 0

	for key := range g.Pages {
		if visited[key] {
			continue
		}
		if g.Pages[key].Journal {
			visited[key] = true
			continue
		}

		// BFS to find connected component
		component := g.bfsComponent(key, visited)
		if len(component) < 2 {
			continue // skip singletons
		}

		// Find hub (highest degree in component)
		hub := component[0]
		hubDegree := g.TotalDegree(hub)
		for _, n := range component[1:] {
			if d := g.TotalDegree(n); d > hubDegree {
				hub = n
				hubDegree = d
			}
		}

		names := make([]string, len(component))
		for i, c := range component {
			names[i] = g.OriginalName(c)
		}
		sort.Strings(names)

		clusters = append(clusters, Cluster{
			ID:    clusterID,
			Size:  len(component),
			Pages: names,
			Hub:   g.OriginalName(hub),
		})
		clusterID++
	}

	// Sort clusters by size descending
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Size > clusters[j].Size
	})

	return clusters
}

// --- Internal helpers ---

func (g *Graph) bfsPaths(fromKey, toKey string, maxDepth int) [][]string {
	type node struct {
		key  string
		path []string
	}

	queue := []node{{key: fromKey, path: []string{g.OriginalName(fromKey)}}}
	visited := map[string]bool{fromKey: true}
	var paths [][]string

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if len(current.path) > maxDepth+1 {
			break
		}

		for linked := range g.Forward[current.key] {
			linkedKey := strings.ToLower(linked)

			if linkedKey == toKey {
				path := make([]string, len(current.path)+1)
				copy(path, current.path)
				path[len(path)-1] = g.OriginalName(linkedKey)
				paths = append(paths, path)
				if len(paths) >= 10 {
					return paths
				}
				continue
			}

			if !visited[linkedKey] && len(current.path) < maxDepth {
				visited[linkedKey] = true
				newPath := make([]string, len(current.path)+1)
				copy(newPath, current.path)
				newPath[len(newPath)-1] = g.OriginalName(linkedKey)
				queue = append(queue, node{key: linkedKey, path: newPath})
			}
		}
	}

	return paths
}

func (g *Graph) bfsComponent(start string, visited map[string]bool) []string {
	queue := []string{start}
	visited[start] = true
	var component []string

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		component = append(component, current)

		// Forward neighbors
		for linked := range g.Forward[current] {
			key := strings.ToLower(linked)
			if !visited[key] {
				if _, exists := g.Pages[key]; exists {
					visited[key] = true
					queue = append(queue, key)
				}
			}
		}
		// Backward neighbors
		for linker := range g.Backward[current] {
			if !visited[linker] {
				if _, exists := g.Pages[linker]; exists {
					visited[linker] = true
					queue = append(queue, linker)
				}
			}
		}
	}

	return component
}

func (g *Graph) allNeighbors(key string) map[string]bool {
	neighbors := make(map[string]bool)
	for linked := range g.Forward[key] {
		neighbors[strings.ToLower(linked)] = true
	}
	for linker := range g.Backward[key] {
		neighbors[linker] = true
	}
	return neighbors
}

func hasKeyInsensitive(m map[string]bool, target string) bool {
	for k := range m {
		if strings.ToLower(k) == target {
			return true
		}
	}
	return false
}

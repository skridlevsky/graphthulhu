package parser

import (
	"regexp"
	"strings"

	"github.com/skridlevsky/graphthulhu/types"
)

var (
	// [[page name]] — wiki-style page links
	linkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

	// ((uuid)) — block references
	blockRefPattern = regexp.MustCompile(`\(\(([0-9a-f-]{36})\)\)`)

	// #tag or #[[multi word tag]] — tags
	tagPattern = regexp.MustCompile(`(?:^|\s)#([a-zA-Z0-9_-]+)`)
	tagBracketPattern = regexp.MustCompile(`#\[\[([^\]]+)\]\]`)

	// key:: value — inline properties
	propertyPattern = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_-]*)::\s*(.+)$`)

	// TODO/DOING/DONE/LATER/NOW/WAITING/CANCELLED markers
	markerPattern = regexp.MustCompile(`^(TODO|DOING|DONE|LATER|NOW|WAITING|CANCELLED)\s`)

	// Priority [#A], [#B], [#C]
	priorityPattern = regexp.MustCompile(`\[#([A-C])\]`)
)

// Parse extracts structured data from a block's raw content string.
func Parse(content string) types.ParsedContent {
	result := types.ParsedContent{
		Raw:             content,
		Links:           extractLinks(content),
		BlockReferences: extractBlockRefs(content),
		Tags:            extractTags(content),
		Properties:      extractProperties(content),
	}

	if m := markerPattern.FindStringSubmatch(content); len(m) > 1 {
		result.Marker = m[1]
	}

	if m := priorityPattern.FindStringSubmatch(content); len(m) > 1 {
		result.Priority = m[1]
	}

	return result
}

// extractLinks finds all [[page name]] patterns in content.
func extractLinks(content string) []string {
	matches := linkPattern.FindAllStringSubmatch(content, -1)
	links := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			links = append(links, name)
			seen[name] = true
		}
	}
	return links
}

// extractBlockRefs finds all ((uuid)) patterns in content.
func extractBlockRefs(content string) []string {
	matches := blockRefPattern.FindAllStringSubmatch(content, -1)
	refs := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		uuid := m[1]
		if !seen[uuid] {
			refs = append(refs, uuid)
			seen[uuid] = true
		}
	}
	return refs
}

// extractTags finds all #tag and #[[multi word tag]] patterns in content.
func extractTags(content string) []string {
	seen := make(map[string]bool)
	var tags []string

	// Simple #tag
	for _, m := range tagPattern.FindAllStringSubmatch(content, -1) {
		tag := m[1]
		if !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}

	// #[[multi word tag]]
	for _, m := range tagBracketPattern.FindAllStringSubmatch(content, -1) {
		tag := m[1]
		if !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}

	return tags
}

// extractProperties finds key:: value pairs in content.
// In Logseq, properties are typically on their own line or at the start of a block.
func extractProperties(content string) map[string]string {
	props := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if m := propertyPattern.FindStringSubmatch(line); len(m) > 2 {
			props[m[1]] = strings.TrimSpace(m[2])
		}
	}
	if len(props) == 0 {
		return nil
	}
	return props
}

// StripMarker removes the TODO/DOING/DONE prefix from block content.
func StripMarker(content string) string {
	return markerPattern.ReplaceAllString(content, "")
}

// StripBullet removes the leading "- " bullet from Logseq block content.
func StripBullet(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "- ") {
		return content[2:]
	}
	return content
}

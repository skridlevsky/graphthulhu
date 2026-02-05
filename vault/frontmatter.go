package vault

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// parseFrontmatter extracts YAML frontmatter from markdown content.
// Returns the parsed properties and the remaining content after the frontmatter.
// If no frontmatter is found, returns nil properties and the original content.
func parseFrontmatter(content string) (map[string]any, string) {
	if !strings.HasPrefix(content, "---") {
		return nil, content
	}

	// Split on the first line-starting "---" after the opening delimiter.
	// content[3:] removes the opening "---", leaving "\n<yaml>\n---\n<body>".
	parts := strings.SplitN(content[3:], "\n---", 2)
	if len(parts) < 2 {
		return nil, content
	}

	yamlBlock := strings.TrimPrefix(parts[0], "\n")
	yamlBlock = strings.TrimPrefix(yamlBlock, "\r\n")
	after := strings.TrimPrefix(parts[1], "\n")
	after = strings.TrimPrefix(after, "\r\n")

	var props map[string]any
	if err := yaml.Unmarshal([]byte(yamlBlock), &props); err != nil {
		return nil, content
	}

	return props, after
}

// renderFrontmatter serializes properties to a YAML frontmatter block.
// Returns "---\nkey: value\n---\n".
func renderFrontmatter(properties map[string]any) string {
	if len(properties) == 0 {
		return ""
	}

	data, err := yaml.Marshal(properties)
	if err != nil {
		return ""
	}

	return "---\n" + strings.TrimRight(string(data), "\n") + "\n---\n"
}

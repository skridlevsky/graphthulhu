package vault

import (
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantProps map[string]any
		wantBody  string
	}{
		{
			name:      "no frontmatter",
			input:     "# Hello\nSome content",
			wantProps: nil,
			wantBody:  "# Hello\nSome content",
		},
		{
			name:  "simple frontmatter",
			input: "---\ntitle: My Page\nstatus: active\n---\n# Hello\nContent here",
			wantProps: map[string]any{
				"title":  "My Page",
				"status": "active",
			},
			wantBody: "# Hello\nContent here",
		},
		{
			name:  "array property",
			input: "---\ntags: [go, mcp, logseq]\n---\n# Page",
			wantProps: map[string]any{
				"tags": []any{"go", "mcp", "logseq"},
			},
			wantBody: "# Page",
		},
		{
			name:  "nested object",
			input: "---\ntype: project\nmetadata:\n  created: \"2026-01-15\"\n  priority: high\n---\nBody text",
			wantProps: map[string]any{
				"type": "project",
				"metadata": map[string]any{
					"created":  "2026-01-15",
					"priority": "high",
				},
			},
			wantBody: "Body text",
		},
		{
			name:  "aliases as list",
			input: "---\naliases:\n  - graphthulhu-mcp\n  - graph-server\n---\n# Main",
			wantProps: map[string]any{
				"aliases": []any{"graphthulhu-mcp", "graph-server"},
			},
			wantBody: "# Main",
		},
		{
			name:      "unclosed frontmatter treated as no frontmatter",
			input:     "---\ntitle: Broken\n# No closing delimiter",
			wantProps: nil,
			wantBody:  "---\ntitle: Broken\n# No closing delimiter",
		},
		{
			name:      "empty frontmatter",
			input:     "---\n---\nContent only",
			wantProps: nil, // yaml.Unmarshal of empty string returns nil map
			wantBody:  "Content only",
		},
		{
			name:      "not starting with frontmatter delimiter",
			input:     "# Title\n---\nstatus: active\n---",
			wantProps: nil,
			wantBody:  "# Title\n---\nstatus: active\n---",
		},
		{
			name:  "boolean and integer values",
			input: "---\ndraft: true\nversion: 3\n---\nBody",
			wantProps: map[string]any{
				"draft":   true,
				"version": 3,
			},
			wantBody: "Body",
		},
		{
			name:  "empty body after frontmatter",
			input: "---\ntitle: Meta Only\n---\n",
			wantProps: map[string]any{
				"title": "Meta Only",
			},
			wantBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProps, gotBody := parseFrontmatter(tt.input)

			if tt.wantProps == nil {
				if gotProps != nil {
					t.Errorf("props = %v, want nil", gotProps)
				}
			} else {
				if gotProps == nil {
					t.Fatalf("props = nil, want %v", tt.wantProps)
				}
				for k, wantV := range tt.wantProps {
					gotV, ok := gotProps[k]
					if !ok {
						t.Errorf("missing property %q", k)
						continue
					}
					if !deepEqual(gotV, wantV) {
						t.Errorf("props[%q] = %v (%T), want %v (%T)", k, gotV, gotV, wantV, wantV)
					}
				}
			}

			if gotBody != tt.wantBody {
				t.Errorf("body =\n%q\nwant\n%q", gotBody, tt.wantBody)
			}
		})
	}
}

// deepEqual compares values handling YAML's type coercion (int vs int).
func deepEqual(a, b any) bool {
	switch av := a.(type) {
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !deepEqual(v, bv[k]) {
				return false
			}
		}
		return true
	case int:
		// yaml.v3 unmarshals integers as int
		if bv, ok := b.(int); ok {
			return av == bv
		}
		return false
	default:
		return a == b
	}
}

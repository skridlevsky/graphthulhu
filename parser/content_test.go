package parser

import (
	"testing"
)

// --- Parse (integration) ---

func TestParse_Empty(t *testing.T) {
	r := Parse("")
	if r.Raw != "" {
		t.Errorf("Raw = %q, want empty", r.Raw)
	}
	if len(r.Links) != 0 {
		t.Errorf("Links = %v, want empty", r.Links)
	}
	if len(r.BlockReferences) != 0 {
		t.Errorf("BlockReferences = %v, want empty", r.BlockReferences)
	}
	if len(r.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", r.Tags)
	}
	if r.Properties != nil {
		t.Errorf("Properties = %v, want nil", r.Properties)
	}
	if r.Marker != "" {
		t.Errorf("Marker = %q, want empty", r.Marker)
	}
	if r.Priority != "" {
		t.Errorf("Priority = %q, want empty", r.Priority)
	}
}

func TestParse_AllFeatures(t *testing.T) {
	content := "TODO [#A] Check [[Important Page]] ref ((550e8400-e29b-41d4-a716-446655440000)) #urgent #[[high priority]]\nstatus:: active"
	r := Parse(content)

	if r.Raw != content {
		t.Errorf("Raw mismatch")
	}
	// #[[high priority]] produces both a link ([[high priority]]) and a tag — correct Logseq behavior
	if len(r.Links) != 2 || r.Links[0] != "Important Page" || r.Links[1] != "high priority" {
		t.Errorf("Links = %v, want [Important Page, high priority]", r.Links)
	}
	if len(r.BlockReferences) != 1 || r.BlockReferences[0] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("BlockReferences = %v", r.BlockReferences)
	}
	if r.Marker != "TODO" {
		t.Errorf("Marker = %q, want TODO", r.Marker)
	}
	if r.Priority != "A" {
		t.Errorf("Priority = %q, want A", r.Priority)
	}
	// Tags
	hasTag := func(tag string) bool {
		for _, t := range r.Tags {
			if t == tag {
				return true
			}
		}
		return false
	}
	if !hasTag("urgent") {
		t.Errorf("missing tag 'urgent', got %v", r.Tags)
	}
	if !hasTag("high priority") {
		t.Errorf("missing tag 'high priority', got %v", r.Tags)
	}
	// Properties
	if r.Properties == nil || r.Properties["status"] != "active" {
		t.Errorf("Properties = %v, want {status: active}", r.Properties)
	}
}

// --- Links ---

func TestLinks_Single(t *testing.T) {
	r := Parse("See [[My Page]]")
	if len(r.Links) != 1 || r.Links[0] != "My Page" {
		t.Errorf("Links = %v, want [My Page]", r.Links)
	}
}

func TestLinks_Multiple(t *testing.T) {
	r := Parse("[[A]] links to [[B]] and [[C]]")
	want := []string{"A", "B", "C"}
	if len(r.Links) != len(want) {
		t.Fatalf("Links = %v, want %v", r.Links, want)
	}
	for i, l := range r.Links {
		if l != want[i] {
			t.Errorf("Links[%d] = %q, want %q", i, l, want[i])
		}
	}
}

func TestLinks_Deduplicate(t *testing.T) {
	r := Parse("[[Page]] and again [[Page]]")
	if len(r.Links) != 1 {
		t.Errorf("Links = %v, want 1 deduplicated entry", r.Links)
	}
}

func TestLinks_SpecialChars(t *testing.T) {
	r := Parse("[[Page Name (2024)]] and [[graphthulhu/vision]]")
	if len(r.Links) != 2 {
		t.Fatalf("Links = %v, want 2", r.Links)
	}
	if r.Links[0] != "Page Name (2024)" {
		t.Errorf("Links[0] = %q", r.Links[0])
	}
	if r.Links[1] != "graphthulhu/vision" {
		t.Errorf("Links[1] = %q", r.Links[1])
	}
}

func TestLinks_None(t *testing.T) {
	r := Parse("plain text with no links")
	if len(r.Links) != 0 {
		t.Errorf("Links = %v, want empty", r.Links)
	}
}

// --- Block References ---

func TestBlockRefs_Valid(t *testing.T) {
	r := Parse("see ((550e8400-e29b-41d4-a716-446655440000))")
	if len(r.BlockReferences) != 1 || r.BlockReferences[0] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("BlockReferences = %v", r.BlockReferences)
	}
}

func TestBlockRefs_Multiple(t *testing.T) {
	r := Parse("((550e8400-e29b-41d4-a716-446655440000)) and ((660e8400-e29b-41d4-a716-446655440001))")
	if len(r.BlockReferences) != 2 {
		t.Errorf("BlockReferences = %v, want 2 entries", r.BlockReferences)
	}
}

func TestBlockRefs_Deduplicate(t *testing.T) {
	r := Parse("((550e8400-e29b-41d4-a716-446655440000)) again ((550e8400-e29b-41d4-a716-446655440000))")
	if len(r.BlockReferences) != 1 {
		t.Errorf("BlockReferences = %v, want 1 deduplicated entry", r.BlockReferences)
	}
}

func TestBlockRefs_TooShort(t *testing.T) {
	r := Parse("((short-uuid))")
	if len(r.BlockReferences) != 0 {
		t.Errorf("BlockReferences = %v, want empty for short UUID", r.BlockReferences)
	}
}

func TestBlockRefs_UppercaseHex(t *testing.T) {
	// Regex only allows lowercase hex [0-9a-f-]
	r := Parse("((550E8400-E29B-41D4-A716-446655440000))")
	if len(r.BlockReferences) != 0 {
		t.Errorf("BlockReferences = %v, want empty for uppercase hex", r.BlockReferences)
	}
}

// --- Tags ---

func TestTags_Simple(t *testing.T) {
	r := Parse("#urgent")
	if len(r.Tags) != 1 || r.Tags[0] != "urgent" {
		t.Errorf("Tags = %v, want [urgent]", r.Tags)
	}
}

func TestTags_MultipleSimple(t *testing.T) {
	r := Parse("#one #two #three")
	if len(r.Tags) != 3 {
		t.Errorf("Tags = %v, want 3 entries", r.Tags)
	}
}

func TestTags_BracketTag(t *testing.T) {
	r := Parse("#[[multi word tag]]")
	if len(r.Tags) != 1 || r.Tags[0] != "multi word tag" {
		t.Errorf("Tags = %v, want [multi word tag]", r.Tags)
	}
}

func TestTags_MixedSimpleAndBracket(t *testing.T) {
	r := Parse("#simple and #[[bracket tag]]")
	if len(r.Tags) != 2 {
		t.Fatalf("Tags = %v, want 2 entries", r.Tags)
	}
	if r.Tags[0] != "simple" {
		t.Errorf("Tags[0] = %q, want 'simple'", r.Tags[0])
	}
	if r.Tags[1] != "bracket tag" {
		t.Errorf("Tags[1] = %q, want 'bracket tag'", r.Tags[1])
	}
}

func TestTags_NoWordBoundary(t *testing.T) {
	// #tag requires whitespace or start-of-line before #
	r := Parse("hello#tag")
	if len(r.Tags) != 0 {
		t.Errorf("Tags = %v, want empty (no word boundary)", r.Tags)
	}
}

func TestTags_WithHyphenUnderscore(t *testing.T) {
	r := Parse("#my-tag_name")
	if len(r.Tags) != 1 || r.Tags[0] != "my-tag_name" {
		t.Errorf("Tags = %v, want [my-tag_name]", r.Tags)
	}
}

func TestTags_DeduplicateMixed(t *testing.T) {
	// If #tag appears as both simple and bracket, only count once
	r := Parse("#tag and #[[tag]]")
	if len(r.Tags) != 1 {
		t.Errorf("Tags = %v, want 1 deduplicated entry", r.Tags)
	}
}

// --- Properties ---

func TestProperties_Single(t *testing.T) {
	r := Parse("status:: active")
	if r.Properties == nil || r.Properties["status"] != "active" {
		t.Errorf("Properties = %v, want {status: active}", r.Properties)
	}
}

func TestProperties_Multiple(t *testing.T) {
	r := Parse("status:: active\npriority:: high\nowner:: Max")
	if len(r.Properties) != 3 {
		t.Fatalf("Properties = %v, want 3 entries", r.Properties)
	}
	if r.Properties["status"] != "active" {
		t.Errorf("status = %q", r.Properties["status"])
	}
	if r.Properties["priority"] != "high" {
		t.Errorf("priority = %q", r.Properties["priority"])
	}
	if r.Properties["owner"] != "Max" {
		t.Errorf("owner = %q", r.Properties["owner"])
	}
}

func TestProperties_NilWhenNone(t *testing.T) {
	r := Parse("just plain text")
	if r.Properties != nil {
		t.Errorf("Properties = %v, want nil", r.Properties)
	}
}

func TestProperties_KeyMustStartWithLetter(t *testing.T) {
	r := Parse("123key:: value")
	if r.Properties != nil {
		t.Errorf("Properties = %v, want nil (key starts with digit)", r.Properties)
	}
}

func TestProperties_ValueWithColons(t *testing.T) {
	r := Parse("url:: https://example.com")
	if r.Properties == nil || r.Properties["url"] != "https://example.com" {
		t.Errorf("Properties = %v", r.Properties)
	}
}

func TestProperties_WhitespaceTrimmed(t *testing.T) {
	r := Parse("key::   value with spaces  ")
	if r.Properties == nil || r.Properties["key"] != "value with spaces" {
		t.Errorf("Properties[key] = %q, want 'value with spaces'", r.Properties["key"])
	}
}

// --- Markers ---

func TestMarkers_All(t *testing.T) {
	markers := []string{"TODO", "DOING", "DONE", "LATER", "NOW", "WAITING", "CANCELLED"}
	for _, m := range markers {
		r := Parse(m + " some task")
		if r.Marker != m {
			t.Errorf("Parse(%q).Marker = %q, want %q", m+" some task", r.Marker, m)
		}
	}
}

func TestMarker_MidLine(t *testing.T) {
	r := Parse("text TODO task")
	if r.Marker != "" {
		t.Errorf("Marker = %q, want empty (marker must be at start)", r.Marker)
	}
}

func TestMarker_Lowercase(t *testing.T) {
	r := Parse("todo task")
	if r.Marker != "" {
		t.Errorf("Marker = %q, want empty (case-sensitive)", r.Marker)
	}
}

func TestMarker_NoSpace(t *testing.T) {
	r := Parse("TODOtask")
	if r.Marker != "" {
		t.Errorf("Marker = %q, want empty (requires space after marker)", r.Marker)
	}
}

// --- Priority ---

func TestPriority_AllLevels(t *testing.T) {
	for _, p := range []string{"A", "B", "C"} {
		r := Parse("[#" + p + "] task")
		if r.Priority != p {
			t.Errorf("Priority = %q, want %q", r.Priority, p)
		}
	}
}

func TestPriority_InvalidLetter(t *testing.T) {
	r := Parse("[#D] task")
	if r.Priority != "" {
		t.Errorf("Priority = %q, want empty (only A-C valid)", r.Priority)
	}
}

func TestPriority_MidContent(t *testing.T) {
	r := Parse("task with [#B] priority")
	if r.Priority != "B" {
		t.Errorf("Priority = %q, want B (matches anywhere)", r.Priority)
	}
}

// --- StripMarker ---

func TestStripMarker_Removes(t *testing.T) {
	got := StripMarker("TODO buy milk")
	if got != "buy milk" {
		t.Errorf("StripMarker = %q, want 'buy milk'", got)
	}
}

func TestStripMarker_NoMarker(t *testing.T) {
	got := StripMarker("just text")
	if got != "just text" {
		t.Errorf("StripMarker = %q, want 'just text'", got)
	}
}

func TestStripMarker_AllTypes(t *testing.T) {
	markers := []string{"TODO", "DOING", "DONE", "LATER", "NOW", "WAITING", "CANCELLED"}
	for _, m := range markers {
		got := StripMarker(m + " task")
		if got != "task" {
			t.Errorf("StripMarker(%q) = %q, want 'task'", m+" task", got)
		}
	}
}

// --- StripBullet ---

func TestStripBullet_Removes(t *testing.T) {
	got := StripBullet("- item")
	if got != "item" {
		t.Errorf("StripBullet = %q, want 'item'", got)
	}
}

func TestStripBullet_NoBullet(t *testing.T) {
	got := StripBullet("item")
	if got != "item" {
		t.Errorf("StripBullet = %q, want 'item'", got)
	}
}

func TestStripBullet_NoSpace(t *testing.T) {
	got := StripBullet("-item")
	if got != "-item" {
		t.Errorf("StripBullet = %q, want '-item' (no space after dash)", got)
	}
}

func TestStripBullet_LeadingWhitespace(t *testing.T) {
	got := StripBullet("  - item")
	if got != "item" {
		t.Errorf("StripBullet = %q, want 'item' (trims whitespace first)", got)
	}
}

func TestStripBullet_AsteriskBullet(t *testing.T) {
	got := StripBullet("* item")
	if got != "* item" {
		t.Errorf("StripBullet = %q, want '* item' (only strips dash bullet)", got)
	}
}

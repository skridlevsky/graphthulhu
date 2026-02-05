package types

import (
	"encoding/json"
	"testing"
)

func TestBlockEntityUnmarshalJSON(t *testing.T) {
	t.Run("full children", func(t *testing.T) {
		data := `{
			"uuid": "parent-uuid",
			"content": "parent block",
			"children": [
				{"uuid": "child-1", "content": "first child"},
				{"uuid": "child-2", "content": "second child"}
			]
		}`
		var b BlockEntity
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if b.UUID != "parent-uuid" {
			t.Errorf("UUID = %q, want %q", b.UUID, "parent-uuid")
		}
		if len(b.Children) != 2 {
			t.Fatalf("Children count = %d, want 2", len(b.Children))
		}
		if b.Children[0].UUID != "child-1" {
			t.Errorf("Children[0].UUID = %q, want %q", b.Children[0].UUID, "child-1")
		}
		if b.Children[1].Content != "second child" {
			t.Errorf("Children[1].Content = %q, want %q", b.Children[1].Content, "second child")
		}
	})

	t.Run("compact children silently skipped", func(t *testing.T) {
		// Logseq getBlock returns children as [["uuid", "value"]]
		data := `{
			"uuid": "parent-uuid",
			"content": "parent block",
			"children": [["uuid", "child-uuid-1"], ["uuid", "child-uuid-2"]]
		}`
		var b BlockEntity
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if b.UUID != "parent-uuid" {
			t.Errorf("UUID = %q, want %q", b.UUID, "parent-uuid")
		}
		if b.Content != "parent block" {
			t.Errorf("Content = %q, want %q", b.Content, "parent block")
		}
		if len(b.Children) != 0 {
			t.Errorf("Children count = %d, want 0 (compact refs should be skipped)", len(b.Children))
		}
	})

	t.Run("no children field", func(t *testing.T) {
		data := `{"uuid": "abc", "content": "no children"}`
		var b BlockEntity
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if b.UUID != "abc" {
			t.Errorf("UUID = %q, want %q", b.UUID, "abc")
		}
		if len(b.Children) != 0 {
			t.Errorf("Children count = %d, want 0", len(b.Children))
		}
	})

	t.Run("empty children array", func(t *testing.T) {
		data := `{"uuid": "abc", "content": "empty", "children": []}`
		var b BlockEntity
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if len(b.Children) != 0 {
			t.Errorf("Children count = %d, want 0", len(b.Children))
		}
	})

	t.Run("preserves all fields with compact children", func(t *testing.T) {
		data := `{
			"id": 42,
			"uuid": "test-uuid",
			"content": "DECIDE something",
			"format": "markdown",
			"marker": "DECIDE",
			"priority": "A",
			"page": {"id": 10, "name": "test-page"},
			"parent": {"id": 5},
			"left": {"id": 4},
			"children": [["uuid", "child-1"]],
			"properties": {"deadline": "2026-03-01"},
			"propertiesOrder": ["deadline"],
			"preBlock": false
		}`
		var b BlockEntity
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if b.ID != 42 {
			t.Errorf("ID = %d, want 42", b.ID)
		}
		if b.Format != "markdown" {
			t.Errorf("Format = %q, want %q", b.Format, "markdown")
		}
		if b.Marker != "DECIDE" {
			t.Errorf("Marker = %q, want %q", b.Marker, "DECIDE")
		}
		if b.Priority != "A" {
			t.Errorf("Priority = %q, want %q", b.Priority, "A")
		}
		if b.Page == nil || b.Page.Name != "test-page" {
			t.Errorf("Page = %v, want name=test-page", b.Page)
		}
		if b.Parent == nil || b.Parent.ID != 5 {
			t.Errorf("Parent = %v, want id=5", b.Parent)
		}
		if b.Left == nil || b.Left.ID != 4 {
			t.Errorf("Left = %v, want id=4", b.Left)
		}
		if len(b.Children) != 0 {
			t.Errorf("Children count = %d, want 0 (compact)", len(b.Children))
		}
		if len(b.PropertiesOrder) != 1 || b.PropertiesOrder[0] != "deadline" {
			t.Errorf("PropertiesOrder = %v, want [deadline]", b.PropertiesOrder)
		}
	})

	t.Run("nested full children", func(t *testing.T) {
		data := `{
			"uuid": "root",
			"content": "root block",
			"children": [
				{
					"uuid": "child-1",
					"content": "child with grandchildren",
					"children": [
						{"uuid": "grandchild-1", "content": "gc1"}
					]
				}
			]
		}`
		var b BlockEntity
		if err := json.Unmarshal([]byte(data), &b); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if len(b.Children) != 1 {
			t.Fatalf("Children count = %d, want 1", len(b.Children))
		}
		child := b.Children[0]
		if child.UUID != "child-1" {
			t.Errorf("Child UUID = %q, want %q", child.UUID, "child-1")
		}
		if len(child.Children) != 1 {
			t.Fatalf("Grandchildren count = %d, want 1", len(child.Children))
		}
		if child.Children[0].UUID != "grandchild-1" {
			t.Errorf("Grandchild UUID = %q, want %q", child.Children[0].UUID, "grandchild-1")
		}
	})
}

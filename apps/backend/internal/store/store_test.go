package store

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildFileTree(t *testing.T) {
	projectID := uuid.New()
	tree := BuildFileTree([]ProjectFile{
		{ProjectID: projectID, Path: "src/components/Button.tsx"},
		{ProjectID: projectID, Path: "src/App.tsx"},
		{ProjectID: projectID, Path: "package.json"},
	})
	if len(tree) != 2 {
		t.Fatalf("root node count = %d, want 2: %#v", len(tree), tree)
	}
	if tree[0].Name != "src" || tree[0].Type != "folder" {
		t.Fatalf("first root node = %#v, want src folder", tree[0])
	}
	if len(tree[0].Children) != 2 {
		t.Fatalf("src child count = %d, want 2: %#v", len(tree[0].Children), tree[0].Children)
	}
	if tree[1].Name != "package.json" || tree[1].Type != "file" {
		t.Fatalf("second root node = %#v, want package.json file", tree[1])
	}
}

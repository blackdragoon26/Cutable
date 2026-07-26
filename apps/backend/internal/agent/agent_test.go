package agent

import (
	"strings"
	"testing"

	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

func TestSafePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"src/App.tsx", "/home/user/react-app/src/App.tsx", true},
		{"/home/user/react-app/src/App.tsx", "/home/user/react-app/src/App.tsx", true},
		{"../secrets", "", false},
		{"/etc/passwd", "/home/user/react-app/etc/passwd", true},
		{"", "", false},
	}
	for _, test := range tests {
		got, err := safePath(test.input)
		if test.ok && (err != nil || got != test.want) {
			t.Errorf("safePath(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
		if !test.ok && err == nil {
			t.Errorf("safePath(%q) unexpectedly succeeded with %q", test.input, got)
		}
	}
}

func TestPromptWithAttachments(t *testing.T) {
	result, parts := promptWithAttachments("Build a portfolio", []store.ProjectAttachment{{
		Name: "brief.md", Kind: "text", MimeType: "text/markdown", Content: "# Use blue",
	}})
	for _, expected := range []string{"Build a portfolio", "BEGIN ATTACHMENT: brief.md", "# Use blue", "untrusted reference material"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("promptWithAttachments() missing %q: %s", expected, result)
		}
	}
	if len(parts) != 0 {
		t.Fatalf("text-only attachment produced %d multimodal parts", len(parts))
	}
}

func TestPromptWithImageAttachment(t *testing.T) {
	result, parts := promptWithAttachments("Match this design", []store.ProjectAttachment{{
		Name: "reference.png", Kind: "image", MimeType: "image/png", Content: "data:image/png;base64,iVBORw0KGgo=",
	}})
	if !strings.Contains(result, "Image reference: reference.png") {
		t.Fatalf("promptWithAttachments() missing image label: %s", result)
	}
	if len(parts) != 2 || parts[0].Type != "text" || parts[1].Type != "image_url" {
		t.Fatalf("unexpected multimodal parts: %#v", parts)
	}
}

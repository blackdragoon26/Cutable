package httpapi

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

func TestValidateAttachments(t *testing.T) {
	valid, err := validateAttachments([]store.ProjectAttachmentInput{{
		Name: "brief.md", Content: "# Build brief", MimeType: "text/markdown",
	}})
	if err != nil {
		t.Fatalf("validateAttachments(valid) = %v", err)
	}
	if len(valid) != 1 || valid[0].Size != len("# Build brief") {
		t.Fatalf("validated attachment = %#v", valid)
	}

	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	imageContent := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	images, err := validateAttachments([]store.ProjectAttachmentInput{{
		Name: "reference.png", Kind: "image", MimeType: "image/png", Content: imageContent,
	}})
	if err != nil {
		t.Fatalf("validateAttachments(image) = %v", err)
	}
	if len(images) != 1 || images[0].Size != len(png) {
		t.Fatalf("validated image = %#v", images)
	}

	tests := []struct {
		name  string
		input []store.ProjectAttachmentInput
	}{
		{name: "unsupported extension", input: []store.ProjectAttachmentInput{{Name: "photo.png", Content: "binary"}}},
		{name: "path traversal", input: []store.ProjectAttachmentInput{{Name: "../secret.txt", Content: "nope"}}},
		{name: "empty", input: []store.ProjectAttachmentInput{{Name: "empty.txt"}}},
		{name: "too large", input: []store.ProjectAttachmentInput{{Name: "large.txt", Content: strings.Repeat("a", 100*1024+1)}}},
		{name: "mismatched image", input: []store.ProjectAttachmentInput{{Name: "photo.png", Kind: "image", MimeType: "image/jpeg", Content: imageContent}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := validateAttachments(test.input); err == nil {
				t.Fatal("validateAttachments() succeeded, want error")
			}
		})
	}
}

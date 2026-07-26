package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageMarshalMultimodal(t *testing.T) {
	encoded, err := json.Marshal(Message{
		Role: "user",
		Parts: []ContentPart{
			{Type: "text", Text: "Use this reference"},
			{Type: "image_url", ImageURL: &ImageURLPart{URL: "data:image/png;base64,iVBORw0KGgo="}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	for _, expected := range []string{`"type":"text"`, `"type":"image_url"`, `"data:image/png;base64`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("multimodal message missing %s: %s", expected, body)
		}
	}
}

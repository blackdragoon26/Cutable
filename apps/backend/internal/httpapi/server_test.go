package httpapi

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blackdragoon26/cutable/apps/backend/internal/config"
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

func TestCORSAllowsOnlyConfiguredCredentialedOrigins(t *testing.T) {
	server := &Server{cfg: config.Config{
		FrontendOrigins: []string{
			"https://cutable.sankalpjha.dev",
			"https://cutable.vercel.app",
		},
	}}
	handler := server.cors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, origin := range []string{
		"https://cutable.sankalpjha.dev",
		"https://cutable.vercel.app",
	} {
		request := httptest.NewRequest(http.MethodOptions, "/api/auth/me", nil)
		request.Header.Set("Origin", origin)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("allowed origin header = %q, want %q", got, origin)
		}
		if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("credential header = %q", got)
		}
	}

	request := httptest.NewRequest(http.MethodOptions, "/api/auth/me", nil)
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("untrusted origin unexpectedly allowed: %q", got)
	}
}

func TestAuthCookieSameSite(t *testing.T) {
	if got := authCookieSameSite(false); got != http.SameSiteLaxMode {
		t.Fatalf("local SameSite = %v", got)
	}
	if got := authCookieSameSite(true); got != http.SameSiteNoneMode {
		t.Fatalf("secure SameSite = %v", got)
	}
}

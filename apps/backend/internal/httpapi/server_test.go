package httpapi

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

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

func TestBearerTokenPrefersHeaderThenWebSocketQueryFallback(t *testing.T) {
	rest := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rest.Header.Set("Authorization", "Bearer header-token")
	if got := bearerToken(rest); got != "header-token" {
		t.Fatalf("bearerToken(header) = %q", got)
	}

	noHeaderQuery := httptest.NewRequest(http.MethodGet, "/api/auth/me?token=query-token", nil)
	if got := bearerToken(noHeaderQuery); got != "" {
		t.Fatalf("bearerToken(REST query fallback) = %q, want empty (query fallback only applies to websocket upgrades)", got)
	}

	wsUpgrade := httptest.NewRequest(http.MethodGet, "/ws?projectId=1&token=ws-query-token", nil)
	wsUpgrade.Header.Set("Upgrade", "websocket")
	if got := bearerToken(wsUpgrade); got != "ws-query-token" {
		t.Fatalf("bearerToken(ws query fallback) = %q", got)
	}

	wsUpgradeWithHeader := httptest.NewRequest(http.MethodGet, "/ws?projectId=1&token=ws-query-token", nil)
	wsUpgradeWithHeader.Header.Set("Upgrade", "websocket")
	wsUpgradeWithHeader.Header.Set("Authorization", "Bearer ws-header-token")
	if got := bearerToken(wsUpgradeWithHeader); got != "ws-header-token" {
		t.Fatalf("bearerToken(ws header priority) = %q, want header to win over query", got)
	}

	empty := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	if got := bearerToken(empty); got != "" {
		t.Fatalf("bearerToken(none) = %q, want empty", got)
	}
}

func TestIssueAuthTokenSetsCookieAndReturnsVerifiableJWT(t *testing.T) {
	userID := uuid.New()
	server := &Server{cfg: config.Config{JWTSecret: strings.Repeat("s", 32), CookieSecure: false}}
	recorder := httptest.NewRecorder()

	token, err := server.issueAuthToken(recorder, userID)
	if err != nil {
		t.Fatalf("issueAuthToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("issueAuthToken() returned empty token")
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "token" || cookies[0].Value != token {
		t.Fatalf("cookie = %#v, want single token cookie matching returned token", cookies)
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte(server.cfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("token did not parse as a valid JWT: %v", err)
	}
	if subject, _ := claims.GetSubject(); subject != userID.String() {
		t.Fatalf("token subject = %q, want %q", subject, userID.String())
	}
}

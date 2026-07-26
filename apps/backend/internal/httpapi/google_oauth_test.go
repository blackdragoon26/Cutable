package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/blackdragoon26/cutable/apps/backend/internal/config"
)

func TestSafeNextPath(t *testing.T) {
	tests := map[string]string{
		"":                       "/",
		"https://evil.example":   "/",
		"//evil.example/path":    "/",
		"/projects/123?tab=code": "/projects/123?tab=code",
	}
	for input, expected := range tests {
		if actual := safeNextPath(input); actual != expected {
			t.Errorf("safeNextPath(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestGoogleLoginUsesStateAndPKCE(t *testing.T) {
	server := &Server{
		cfg: config.Config{
			GoogleClientID: "client-id", GoogleClientSecret: "client-secret",
			GoogleRedirectURL: "http://localhost:3010/api/auth/google/callback",
		},
		google: defaultGoogleOAuthProvider(),
	}
	request := httptest.NewRequest(http.MethodGet, "/api/auth/google?next=/projects/123", nil)
	recorder := httptest.NewRecorder()

	server.googleLogin(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusFound)
	}
	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := location.Query()
	if query.Get("state") == "" || query.Get("code_challenge") == "" ||
		query.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization query missing state or PKCE: %s", location.RawQuery)
	}
	if query.Get("scope") != "openid email profile" {
		t.Fatalf("scope = %q", query.Get("scope"))
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("cookie count = %d, want 3", len(cookies))
	}
	for _, cookie := range cookies {
		if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode ||
			!strings.HasPrefix(cookie.Path, "/api/auth/google") {
			t.Fatalf("OAuth cookie has unsafe attributes: %#v", cookie)
		}
	}
}

func TestGoogleLoginRequiresConfiguration(t *testing.T) {
	server := &Server{cfg: config.Config{}, google: defaultGoogleOAuthProvider()}
	request := httptest.NewRequest(http.MethodGet, "/api/auth/google", nil)
	recorder := httptest.NewRecorder()

	server.googleLogin(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

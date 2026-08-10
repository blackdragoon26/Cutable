package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/config"
)

// These tests exercise verifyJWT's/s.auth's rejection paths, which all
// return before ever touching the store (a real *store.Store needs a live
// Postgres connection, which this test suite intentionally does not stand
// up, consistent with the rest of this package). Any token that gets far
// enough to require a user lookup is out of scope here.

func testServer() *Server {
	return &Server{cfg: config.Config{JWTSecret: strings.Repeat("s", 32)}}
}

func signToken(t *testing.T, secret string, method jwt.SigningMethod, claims jwt.Claims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestVerifyJWTRejectsExpiredToken(t *testing.T) {
	server := testServer()
	token := signToken(t, server.cfg.JWTSecret, jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})
	if _, err := server.verifyJWT(t.Context(), token); err == nil {
		t.Fatal("expected an expired token to be rejected")
	}
}

func TestVerifyJWTRejectsMissingExpiration(t *testing.T) {
	server := testServer()
	token := signToken(t, server.cfg.JWTSecret, jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:  uuid.New().String(),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	})
	if _, err := server.verifyJWT(t.Context(), token); err == nil {
		t.Fatal("expected a token with no expiration to be rejected")
	}
}

func TestVerifyJWTRejectsWrongSecret(t *testing.T) {
	server := testServer()
	token := signToken(t, "a-completely-different-secret-value", jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	if _, err := server.verifyJWT(t.Context(), token); err == nil {
		t.Fatal("expected a token signed with the wrong secret to be rejected")
	}
}

func TestVerifyJWTRejectsAlgNone(t *testing.T) {
	server := testServer()
	// "alg: none" is a classic JWT library footgun (an attacker strips the
	// signature and sets alg to "none"); confirm the library/our key-func
	// still refuses it rather than treating it as trusted.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign alg=none token: %v", err)
	}
	if _, err := server.verifyJWT(t.Context(), unsigned); err == nil {
		t.Fatal("expected an alg=none token to be rejected")
	}
}

func TestVerifyJWTRejectsNonUUIDSubject(t *testing.T) {
	server := testServer()
	token := signToken(t, server.cfg.JWTSecret, jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "not-a-uuid",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	if _, err := server.verifyJWT(t.Context(), token); err == nil {
		t.Fatal("expected a non-UUID subject to be rejected")
	}
}

func TestVerifyJWTRejectsGarbageToken(t *testing.T) {
	server := testServer()
	if _, err := server.verifyJWT(t.Context(), "not-a-jwt-at-all"); err == nil {
		t.Fatal("expected a malformed token string to be rejected")
	}
}

func TestAuthMiddlewareRejectsRequestsWithNoToken(t *testing.T) {
	server := testServer()
	called := false
	handler := server.auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("downstream handler should not run without a token")
	}
}

func TestAuthMiddlewareRejectsExpiredBearerToken(t *testing.T) {
	server := testServer()
	called := false
	handler := server.auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	expired := signToken(t, server.cfg.JWTSecret, jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+expired)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("downstream handler should not run for an expired token")
	}
}

func TestAuthMiddlewareRejectsExpiredCookie(t *testing.T) {
	server := testServer()
	handler := server.auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("downstream handler should not run for an expired cookie")
	}))

	expired := signToken(t, server.cfg.JWTSecret, jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "token", Value: expired})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

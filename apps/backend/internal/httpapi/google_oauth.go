package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleStateCookie    = "cutable_google_state"
	googleVerifierCookie = "cutable_google_verifier"
	googleNextCookie     = "cutable_google_next"
)

type googleOAuthProvider struct {
	authorizationURL string
	tokenURL         string
	userInfoURL      string
	client           *http.Client
}

type googleUserInfo struct {
	Subject       string `json:"sub"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

func defaultGoogleOAuthProvider() googleOAuthProvider {
	return googleOAuthProvider{
		authorizationURL: "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL:         "https://oauth2.googleapis.com/token",
		userInfoURL:      "https://openidconnect.googleapis.com/v1/userinfo",
		client:           &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Server) authConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"google": map[string]bool{"enabled": s.cfg.GoogleAuthEnabled()},
	})
}

func (s *Server) googleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.GoogleAuthEnabled() {
		writeError(w, http.StatusServiceUnavailable, "Google sign-in is not configured")
		return
	}
	state, err := randomURLToken(32)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	verifier, err := randomURLToken(32)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	next := safeNextPath(r.URL.Query().Get("next"))
	s.setOAuthCookie(w, googleStateCookie, state, 600)
	s.setOAuthCookie(w, googleVerifierCookie, verifier, 600)
	s.setOAuthCookie(w, googleNextCookie, base64.RawURLEncoding.EncodeToString([]byte(next)), 600)

	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"client_id":             {s.cfg.GoogleClientID},
		"redirect_uri":          {s.cfg.GoogleRedirectURL},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	http.Redirect(w, r, s.google.authorizationURL+"?"+query.Encode(), http.StatusFound)
}

func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.GoogleAuthEnabled() {
		s.redirectGoogleError(w, r, "Google sign-in is not configured")
		return
	}
	stateCookie, stateErr := r.Cookie(googleStateCookie)
	verifierCookie, verifierErr := r.Cookie(googleVerifierCookie)
	state := r.URL.Query().Get("state")
	if stateErr != nil || verifierErr != nil || state == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie.Value)) != 1 {
		s.clearOAuthCookies(w)
		s.redirectGoogleError(w, r, "Google sign-in session expired")
		return
	}
	next := "/"
	if nextCookie, err := r.Cookie(googleNextCookie); err == nil {
		if decoded, err := base64.RawURLEncoding.DecodeString(nextCookie.Value); err == nil {
			next = safeNextPath(string(decoded))
		}
	}
	s.clearOAuthCookies(w)
	if providerError := strings.TrimSpace(r.URL.Query().Get("error")); providerError != "" {
		s.redirectGoogleError(w, r, "Google sign-in was cancelled")
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		s.redirectGoogleError(w, r, "Google did not return an authorization code")
		return
	}

	accessToken, err := s.exchangeGoogleCode(r.Context(), code, verifierCookie.Value)
	if err != nil {
		s.logger.Warn("Google token exchange failed", "error", err)
		s.redirectGoogleError(w, r, "Google sign-in could not be completed")
		return
	}
	identity, err := s.fetchGoogleUser(r.Context(), accessToken)
	if err != nil {
		s.logger.Warn("Google user lookup failed", "error", err)
		s.redirectGoogleError(w, r, "Google profile could not be verified")
		return
	}
	if !identity.EmailVerified || identity.Subject == "" || identity.Email == "" {
		s.redirectGoogleError(w, r, "Google account email is not verified")
		return
	}
	user, err := s.store.UpsertGoogleUser(r.Context(), identity.Name, identity.Email, identity.Subject)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if err := s.setAuthCookie(w, user.ID); err != nil {
		s.internalError(w, r, err)
		return
	}
	http.Redirect(w, r, strings.TrimRight(s.cfg.FrontendOrigin, "/")+next, http.StatusFound)
}

func (s *Server) exchangeGoogleCode(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{
		"client_id":     {s.cfg.GoogleClientID},
		"client_secret": {s.cfg.GoogleClientSecret},
		"code":          {code},
		"code_verifier": {verifier},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {s.cfg.GoogleRedirectURL},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.google.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := s.google.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint status %d", res.StatusCode)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return "", errors.New("token endpoint returned an invalid response")
	}
	return token.AccessToken, nil
}

func (s *Server) fetchGoogleUser(ctx context.Context, accessToken string) (googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.google.userInfoURL, nil)
	if err != nil {
		return googleUserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := s.google.client.Do(req)
	if err != nil {
		return googleUserInfo{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
		return googleUserInfo{}, fmt.Errorf("userinfo endpoint status %d", res.StatusCode)
	}
	var identity googleUserInfo
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&identity); err != nil {
		return googleUserInfo{}, err
	}
	identity.Email = strings.ToLower(strings.TrimSpace(identity.Email))
	identity.Name = strings.TrimSpace(identity.Name)
	identity.Subject = strings.TrimSpace(identity.Subject)
	return identity, nil
}

func (s *Server) setOAuthCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/api/auth/google", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearOAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{googleStateCookie, googleVerifierCookie, googleNextCookie} {
		s.setOAuthCookie(w, name, "", -1)
	}
}

func (s *Server) redirectGoogleError(w http.ResponseWriter, r *http.Request, message string) {
	destination := strings.TrimRight(s.cfg.FrontendOrigin, "/") + "/sign-in?error=" + url.QueryEscape(message)
	http.Redirect(w, r, destination, http.StatusFound)
}

func safeNextPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return parsed.RequestURI()
}

func randomURLToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/agent"
	"github.com/blackdragoon26/cutable/apps/backend/internal/config"
	"github.com/blackdragoon26/cutable/apps/backend/internal/provider"
	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

type contextKey string

const userIDKey contextKey = "user_id"

type Server struct {
	cfg     config.Config
	store   *store.Store
	runner  *agent.Runner
	sandbox *provider.E2B
	google  googleOAuthProvider
	logger  *slog.Logger
	mux     *http.ServeMux
}

func New(cfg config.Config, database *store.Store, runner *agent.Runner, sandbox *provider.E2B, logger *slog.Logger) *Server {
	s := &Server{
		cfg: cfg, store: database, runner: runner, sandbox: sandbox,
		google: defaultGoogleOAuthProvider(), logger: logger, mux: http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.recoverer(s.cors(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("POST /api/auth/register", s.register)
	s.mux.HandleFunc("POST /api/auth/login", s.login)
	s.mux.HandleFunc("POST /api/auth/logout", s.logout)
	s.mux.HandleFunc("GET /api/auth/config", s.authConfig)
	s.mux.HandleFunc("GET /api/auth/google", s.googleLogin)
	s.mux.HandleFunc("GET /api/auth/google/callback", s.googleCallback)
	s.mux.Handle("GET /api/auth/me", s.auth(http.HandlerFunc(s.currentUser)))
	s.mux.Handle("GET /api/account/usage", s.auth(http.HandlerFunc(s.accountUsage)))
	s.mux.Handle("GET /api/projects", s.auth(http.HandlerFunc(s.listProjects)))
	s.mux.Handle("POST /api/projects", s.auth(http.HandlerFunc(s.createProject)))
	s.mux.Handle("GET /api/projects/{id}", s.auth(http.HandlerFunc(s.getProject)))
	s.mux.Handle("GET /api/projects/{id}/conversations", s.auth(http.HandlerFunc(s.listConversations)))
	s.mux.Handle("POST /api/projects/{id}/conversations", s.auth(http.HandlerFunc(s.createConversation)))
	s.mux.Handle("GET /api/projects/{id}/files", s.auth(http.HandlerFunc(s.listFiles)))
	s.mux.Handle("GET /api/projects/{id}/files/{path...}", s.auth(http.HandlerFunc(s.getFile)))
	s.mux.Handle("GET /api/projects/{id}/sandbox", s.auth(http.HandlerFunc(s.getSandbox)))
	s.mux.Handle("POST /api/projects/{id}/sandbox", s.auth(http.HandlerFunc(s.createSandbox)))
	s.mux.Handle("DELETE /api/projects/{id}/sandbox", s.auth(http.HandlerFunc(s.deleteSandbox)))
	s.mux.Handle("GET /ws", s.auth(http.HandlerFunc(s.websocket)))
	s.mux.HandleFunc("GET /", s.root)
}

func (s *Server) root(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		s.auth(http.HandlerFunc(s.websocket)).ServeHTTP(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": "Cutable API", "status": "ok"})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// bearerToken extracts a token from the Authorization header, and for
// WebSocket upgrade requests only, also accepts a ?token= query param as a
// fallback for native clients that cannot reliably set arbitrary headers on
// the upgrade request.
func bearerToken(r *http.Request) string {
	if header := r.Header.Get("Authorization"); header != "" {
		if value, ok := strings.CutPrefix(header, "Bearer "); ok && value != "" {
			return value
		}
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		if value := r.URL.Query().Get("token"); value != "" {
			return value
		}
	}
	return ""
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := bearerToken(r)
		if tokenString == "" {
			if cookie, err := r.Cookie("token"); err == nil {
				tokenString = cookie.Value
			}
		}
		if tokenString == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		id, err := s.verifyJWT(r.Context(), tokenString)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired authentication")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, id)))
	})
}

// verifyJWT parses and validates a signed auth token and confirms the
// referenced user still exists. Shared by the cookie/Bearer HTTP middleware
// and the WebSocket upgrade path.
func (s *Server) verifyJWT(ctx context.Context, tokenString string) (uuid.UUID, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return uuid.UUID{}, fmt.Errorf("invalid or expired token")
	}
	subject, err := claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, err
	}
	id, err := uuid.Parse(subject)
	if err != nil {
		return uuid.UUID{}, err
	}
	if _, err := s.store.UserByID(ctx, id); err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// issueAuthToken signs a fresh JWT for the given user, sets it as the
// HttpOnly web session cookie, and returns the raw token string so mobile
// clients can store and send it as a Bearer header instead of relying on
// cookies.
func (s *Server) issueAuthToken(w http.ResponseWriter, id uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   id.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    value,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: authCookieSameSite(s.cfg.CookieSecure),
	})
	return value, nil
}

func authCookieSameSite(secure bool) http.SameSite {
	if secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.cfg.AllowsFrontendOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic recovered", "panic", recovered, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSONLimit(w, r, target, 1<<20)
}

func decodeJSONLimit(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func userID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userIDKey).(uuid.UUID)
	return id
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

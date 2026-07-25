package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

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
	logger  *slog.Logger
	mux     *http.ServeMux
}

func New(cfg config.Config, database *store.Store, runner *agent.Runner, sandbox *provider.E2B, logger *slog.Logger) *Server {
	s := &Server{cfg: cfg, store: database, runner: runner, sandbox: sandbox, logger: logger, mux: http.NewServeMux()}
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
	s.mux.Handle("GET /api/projects", s.auth(http.HandlerFunc(s.listProjects)))
	s.mux.Handle("POST /api/projects", s.auth(http.HandlerFunc(s.createProject)))
	s.mux.Handle("GET /api/projects/{id}", s.auth(http.HandlerFunc(s.getProject)))
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

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := mail.ParseAddress(input.Email); err != nil || len(input.Password) < 8 {
		writeError(w, http.StatusBadRequest, "valid email and password of at least 8 characters are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), input.Email, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "user already exists")
			return
		}
		s.internalError(w, r, err)
		return
	}
	if err := s.setAuthCookie(w, user.ID); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "User created successfully"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.store.UserByEmail(r.Context(), input.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := s.setAuthCookie(w, user.ID); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Login successful"})
}

func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"message": "Logged out"})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.Projects(r.Context(), userID(r.Context()))
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Projects fetched successfully", "projects": projects})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title         string `json:"title"`
		InitialPrompt string `json:"initialPrompt"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.InitialPrompt = strings.TrimSpace(input.InitialPrompt)
	if input.Title == "" || input.InitialPrompt == "" || len(input.Title) > 120 || len(input.InitialPrompt) > 20000 {
		writeError(w, http.StatusBadRequest, "title and initialPrompt are required and must be within limits")
		return
	}
	project, err := s.store.CreateProject(r.Context(), userID(r.Context()), input.Title, input.InitialPrompt)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "Project created successfully", "project": project})
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Project fetched successfully", "project": project})
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	var input struct {
		Contents string  `json:"contents"`
		Type     string  `json:"type"`
		From     string  `json:"from"`
		ToolCall *string `json:"toolCall"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Contents) == "" || !oneOf(input.Type, "TOOL_CALL", "TEXT_MESSAGE", "ERROR_MESSAGE") || !oneOf(input.From, "USER", "AGENT") {
		writeError(w, http.StatusBadRequest, "invalid conversation message")
		return
	}
	conversation, err := s.store.AddConversation(r.Context(), project.ID, input.From, input.Type, input.Contents, input.ToolCall)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "Conversation created", "conversation": conversation})
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	files, err := s.store.Files(r.Context(), project.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Files fetched successfully", "files": store.BuildFileTree(files)})
}

func (s *Server) getFile(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	filePath, err := url.PathUnescape(r.PathValue("path"))
	if err != nil || strings.TrimSpace(filePath) == "" {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}
	file, err := s.store.File(r.Context(), project.ID, filePath)
	if store.IsNotFound(err) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "File fetched successfully", "path": file.Path, "content": file.Content})
}

func (s *Server) getSandbox(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    "Sandbox info fetched",
		"sandboxId":  project.SandboxID,
		"previewUrl": project.SandboxURL,
	})
}

func (s *Server) createSandbox(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	if project.SandboxID != nil {
		if connected, err := s.sandbox.Connect(r.Context(), *project.SandboxID); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"message": "Sandbox reconnected", "sandboxId": connected.SandboxID,
				"previewUrl": s.sandbox.PreviewURL(connected.SandboxID),
			})
			return
		}
	}
	created, err := s.sandbox.Create(r.Context(), project.ID.String())
	if err != nil {
		writeError(w, http.StatusBadGateway, "sandbox creation failed: "+err.Error())
		return
	}
	preview := s.sandbox.PreviewURL(created.SandboxID)
	if err := s.store.SaveSandbox(r.Context(), project.ID, created.SandboxID, preview); err != nil {
		_ = s.sandbox.Kill(context.Background(), created.SandboxID)
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Sandbox created", "sandboxId": created.SandboxID, "previewUrl": preview,
	})
}

func (s *Server) deleteSandbox(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	if project.SandboxID != nil {
		if err := s.sandbox.Kill(r.Context(), *project.SandboxID); err != nil && !strings.Contains(err.Error(), "404") {
			writeError(w, http.StatusBadGateway, "sandbox shutdown failed")
			return
		}
	}
	if err := s.store.ClearSandbox(r.Context(), project.ID); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Sandbox closed"})
}

func (s *Server) ownedProject(w http.ResponseWriter, r *http.Request) (store.Project, bool) {
	projectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project ID")
		return store.Project{}, false
	}
	project, err := s.store.Project(r.Context(), userID(r.Context()), projectID)
	if store.IsNotFound(err) {
		writeError(w, http.StatusNotFound, "project not found")
		return store.Project{}, false
	}
	if err != nil {
		s.internalError(w, r, err)
		return store.Project{}, false
	}
	return project, true
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := ""
		if cookie, err := r.Cookie("token"); err == nil {
			tokenString = cookie.Value
		}
		if tokenString == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(s.cfg.JWTSecret), nil
		}, jwt.WithExpirationRequired(), jwt.WithIssuedAt())
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid or expired authentication")
			return
		}
		subject, err := claims.GetSubject()
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid authentication")
			return
		}
		id, err := uuid.Parse(subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid authentication")
			return
		}
		if _, err := s.store.UserByID(r.Context(), id); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid authentication")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, id)))
	})
}

func (s *Server) setAuthCookie(w http.ResponseWriter, id uuid.UUID) error {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   id.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    value,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == s.cfg.FrontendOrigin {
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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
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

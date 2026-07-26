package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

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
	s.mux.Handle("GET /api/account/usage", s.auth(http.HandlerFunc(s.accountUsage)))
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
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if _, err := mail.ParseAddress(input.Email); err != nil || len(input.Password) < 8 || len(input.Name) > 100 {
		writeError(w, http.StatusBadRequest, "valid email and password of at least 8 characters are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), input.Name, input.Email, string(hash))
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

func (s *Server) accountUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := s.store.DemoUsage(r.Context(), userID(r.Context()), s.cfg.DemoRunLimit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"demo": usage})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title         string                         `json:"title"`
		InitialPrompt string                         `json:"initialPrompt"`
		Attachments   []store.ProjectAttachmentInput `json:"attachments"`
	}
	if !decodeJSONLimit(w, r, &input, 6<<20) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.InitialPrompt = strings.TrimSpace(input.InitialPrompt)
	if input.Title == "" || input.InitialPrompt == "" || len(input.Title) > 120 || len(input.InitialPrompt) > 20000 {
		writeError(w, http.StatusBadRequest, "title and initialPrompt are required and must be within limits")
		return
	}
	attachments, err := validateAttachments(input.Attachments)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.store.CreateProject(r.Context(), userID(r.Context()), input.Title, input.InitialPrompt, attachments)
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
	credentials, ok := decodeOptionalCredentials(w, r)
	if !ok {
		return
	}
	usage, err := s.store.DemoUsage(r.Context(), userID(r.Context()), s.cfg.DemoRunLimit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	sandbox := s.sandbox
	if credentials.E2BAPIKey != "" {
		if err := credentials.validateE2B(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sandbox = provider.NewE2B(credentials.E2BAPIKey, s.cfg.E2BTemplateAlias, s.cfg.SandboxTimeout)
	} else if usage.RequiresKeys {
		writeError(w, http.StatusPaymentRequired, "your two demo builds are used; add your E2B API key to restart the preview")
		return
	}
	if project.SandboxID != nil {
		if connected, err := sandbox.Connect(r.Context(), *project.SandboxID); err == nil &&
			connected.EnvdAccessToken != nil {
			if err := s.prepareSandboxPreview(r.Context(), sandbox, project.ID, connected.SandboxID, *connected.EnvdAccessToken); err != nil {
				writeError(w, http.StatusBadGateway, "sandbox preview restart failed")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"message": "Sandbox reconnected", "sandboxId": connected.SandboxID,
				"previewUrl": sandbox.PreviewURL(connected.SandboxID),
			})
			return
		}
	}
	created, err := sandbox.Create(r.Context(), project.ID.String())
	if err != nil {
		writeError(w, http.StatusBadGateway, "sandbox creation failed: "+err.Error())
		return
	}
	if err := s.prepareSandboxPreview(r.Context(), sandbox, project.ID, created.SandboxID, *created.EnvdAccessToken); err != nil {
		_ = sandbox.Kill(context.Background(), created.SandboxID)
		writeError(w, http.StatusBadGateway, "sandbox preview preparation failed")
		return
	}
	preview := sandbox.PreviewURL(created.SandboxID)
	if err := s.store.SaveSandbox(r.Context(), project.ID, created.SandboxID, preview); err != nil {
		_ = sandbox.Kill(context.Background(), created.SandboxID)
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Sandbox created", "sandboxId": created.SandboxID, "previewUrl": preview,
	})
}

func (s *Server) prepareSandboxPreview(ctx context.Context, sandbox *provider.E2B, projectID uuid.UUID, sandboxID, accessToken string) error {
	files, err := s.store.Files(ctx, projectID)
	if err != nil {
		return err
	}
	for _, file := range files {
		filePath := "/home/user/react-app/" + strings.TrimPrefix(path.Clean("/"+file.Path), "/")
		if err := sandbox.WriteFile(ctx, sandboxID, accessToken, filePath, []byte(file.Content)); err != nil {
			return err
		}
	}
	result, err := sandbox.Run(ctx, sandboxID, accessToken,
		"pkill -f 'node .*/node_modules/.bin/[v]ite' >/dev/null 2>&1 || true; "+
			"nohup npm run dev -- --host 0.0.0.0 --port 5173 --strictPort >/tmp/cutable-vite.log 2>&1 </dev/null & "+
			"for attempt in $(seq 1 20); do "+
			"if curl -fsS http://127.0.0.1:5173/ >/dev/null && curl -fsS http://127.0.0.1:5173/src/main.tsx >/dev/null; then exit 0; fi; "+
			"sleep 1; done; cat /tmp/cutable-vite.log >&2; exit 1",
		"/home/user/react-app")
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("preview process exited with code %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func decodeOptionalCredentials(w http.ResponseWriter, r *http.Request) (providerCredentials, bool) {
	if r.Body == nil || r.ContentLength == 0 {
		return providerCredentials{}, true
	}
	var input struct {
		Credentials providerCredentials `json:"credentials"`
	}
	if !decodeJSONLimit(w, r, &input, 8<<10) {
		return providerCredentials{}, false
	}
	return input.Credentials.normalized(), true
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

func validateAttachments(inputs []store.ProjectAttachmentInput) ([]store.ProjectAttachmentInput, error) {
	const (
		maxFiles          = 3
		maxTextFileBytes  = 100 * 1024
		maxTextTotal      = 200 * 1024
		maxImageFileBytes = 2 * 1024 * 1024
		maxImageTotal     = 3 * 1024 * 1024
	)
	if len(inputs) > maxFiles {
		return nil, fmt.Errorf("a maximum of %d attachments is allowed", maxFiles)
	}
	allowedTextExtensions := map[string]bool{
		".css": true, ".csv": true, ".go": true, ".html": true, ".java": true,
		".js": true, ".json": true, ".jsx": true, ".md": true, ".py": true,
		".rs": true, ".sql": true, ".ts": true, ".tsx": true, ".txt": true,
		".xml": true, ".yaml": true, ".yml": true,
	}
	allowedImageTypes := map[string]string{
		".jpeg": "image/jpeg",
		".jpg":  "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
	}
	seen := map[string]bool{}
	textTotal := 0
	imageTotal := 0
	validated := make([]store.ProjectAttachmentInput, 0, len(inputs))
	for _, input := range inputs {
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || len(input.Name) > 180 || path.Base(input.Name) != input.Name || strings.Contains(input.Name, "\\") {
			return nil, errors.New("attachment names must be simple filenames of at most 180 characters")
		}
		key := strings.ToLower(input.Name)
		if seen[key] {
			return nil, fmt.Errorf("duplicate attachment %q", input.Name)
		}
		seen[key] = true
		extension := strings.ToLower(path.Ext(input.Name))
		if input.Kind == "" {
			if allowedImageTypes[extension] != "" {
				input.Kind = "image"
			} else {
				input.Kind = "text"
			}
		}
		switch input.Kind {
		case "text":
			if !allowedTextExtensions[extension] {
				return nil, fmt.Errorf("attachment %q is not a supported text or source file", input.Name)
			}
			if !utf8.ValidString(input.Content) || strings.ContainsRune(input.Content, '\x00') {
				return nil, fmt.Errorf("attachment %q must contain UTF-8 text", input.Name)
			}
			size := len([]byte(input.Content))
			if size == 0 || size > maxTextFileBytes {
				return nil, fmt.Errorf("attachment %q must be between 1 byte and 100 KB", input.Name)
			}
			textTotal += size
			if textTotal > maxTextTotal {
				return nil, errors.New("text attachments may not exceed 200 KB in total")
			}
			input.Size = size
			if strings.TrimSpace(input.MimeType) == "" {
				input.MimeType = "text/plain"
			}
		case "image":
			expectedMime := allowedImageTypes[extension]
			if expectedMime == "" {
				return nil, fmt.Errorf("attachment %q is not a supported PNG, JPEG, or WebP image", input.Name)
			}
			input.MimeType = strings.ToLower(strings.TrimSpace(input.MimeType))
			if input.MimeType != expectedMime {
				return nil, fmt.Errorf("attachment %q has an invalid image type", input.Name)
			}
			prefix := "data:" + expectedMime + ";base64,"
			encoded, ok := strings.CutPrefix(input.Content, prefix)
			if !ok || encoded == "" {
				return nil, fmt.Errorf("attachment %q must be a base64 image data URL", input.Name)
			}
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return nil, fmt.Errorf("attachment %q contains invalid base64 image data", input.Name)
			}
			size := len(decoded)
			if size == 0 || size > maxImageFileBytes {
				return nil, fmt.Errorf("attachment %q must be between 1 byte and 2 MB", input.Name)
			}
			if detected := http.DetectContentType(decoded); detected != expectedMime {
				return nil, fmt.Errorf("attachment %q content does not match its image type", input.Name)
			}
			imageTotal += size
			if imageTotal > maxImageTotal {
				return nil, errors.New("image attachments may not exceed 3 MB in total")
			}
			input.Size = size
		default:
			return nil, fmt.Errorf("attachment %q has an invalid kind", input.Name)
		}
		validated = append(validated, input)
	}
	return validated, nil
}

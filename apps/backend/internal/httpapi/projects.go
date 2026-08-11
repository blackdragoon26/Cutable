package httpapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

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

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	project, ok := s.ownedProject(w, r)
	if !ok {
		return
	}
	conversations, err := s.store.Conversations(r.Context(), project.ID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if conversations == nil {
		conversations = []store.Conversation{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Conversations fetched successfully", "conversations": conversations})
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

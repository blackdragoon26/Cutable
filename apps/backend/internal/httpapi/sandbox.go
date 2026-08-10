package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/provider"
)

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
		s.killAbandonedSandbox(sandbox, created.SandboxID, err)
		writeError(w, http.StatusBadGateway, "sandbox preview preparation failed")
		return
	}
	preview := sandbox.PreviewURL(created.SandboxID)
	if err := s.store.SaveSandbox(r.Context(), project.ID, created.SandboxID, preview); err != nil {
		s.killAbandonedSandbox(sandbox, created.SandboxID, err)
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
		if err := s.sandbox.Kill(r.Context(), *project.SandboxID); err != nil && !provider.IsNotFound(err) {
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

// killAbandonedSandbox best-effort tears down a just-created sandbox after a
// later setup step fails, logging (rather than silently discarding) any
// cleanup failure so a leaked E2B sandbox is at least visible in logs.
func (s *Server) killAbandonedSandbox(sandbox *provider.E2B, sandboxID string, cause error) {
	if err := sandbox.Kill(context.Background(), sandboxID); err != nil {
		s.logger.Error("failed to clean up abandoned sandbox", "sandboxId", sandboxID, "cause", cause, "killError", err)
	}
}

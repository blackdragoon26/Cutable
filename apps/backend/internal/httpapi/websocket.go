package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/agent"
	"github.com/blackdragoon26/cutable/apps/backend/internal/provider"
)

type socketWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

type providerCredentials struct {
	OpenRouterAPIKey string `json:"openRouterApiKey"`
	E2BAPIKey        string `json:"e2bApiKey"`
}

func (c providerCredentials) normalized() providerCredentials {
	return providerCredentials{
		OpenRouterAPIKey: strings.TrimSpace(c.OpenRouterAPIKey),
		E2BAPIKey:        strings.TrimSpace(c.E2BAPIKey),
	}
}

func (c providerCredentials) complete() bool {
	return c.OpenRouterAPIKey != "" && c.E2BAPIKey != ""
}

func (c providerCredentials) empty() bool {
	return c.OpenRouterAPIKey == "" && c.E2BAPIKey == ""
}

func (c providerCredentials) validate() error {
	if c.empty() {
		return nil
	}
	if !c.complete() {
		return errors.New("both OpenRouter and E2B API keys are required")
	}
	if len(c.OpenRouterAPIKey) < 16 || len(c.OpenRouterAPIKey) > 512 {
		return errors.New("OpenRouter API key has an invalid length")
	}
	return c.validateE2B()
}

func (c providerCredentials) validateE2B() error {
	if len(c.E2BAPIKey) < 16 || len(c.E2BAPIKey) > 512 {
		return errors.New("E2B API key has an invalid length")
	}
	return nil
}

func (w *socketWriter) send(ctx context.Context, event map[string]any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	writeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return wsjson.Write(writeCtx, w.conn, event)
}

func (s *Server) websocket(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.URL.Query().Get("projectId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "valid projectId is required")
		return
	}
	if _, err := s.store.Project(r.Context(), userID(r.Context()), projectID); err != nil {
		writeError(w, http.StatusForbidden, "project access denied")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{originHost(s.cfg.FrontendOrigin)},
	})
	if err != nil {
		s.logger.Error("websocket accept failed", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(1 << 20)
	writer := &socketWriter{conn: conn}
	if err := writer.send(r.Context(), map[string]any{"e": "connected", "authenticated": true, "projectId": projectID}); err != nil {
		return
	}

	for {
		var message struct {
			Type        string              `json:"type"`
			Prompt      string              `json:"prompt"`
			Credentials providerCredentials `json:"credentials"`
		}
		if err := wsjson.Read(r.Context(), conn, &message); err != nil {
			return
		}
		switch message.Type {
		case "ping":
			if err := writer.send(r.Context(), map[string]any{"e": "pong"}); err != nil {
				return
			}
		case "start_agent":
			if message.Prompt == "" {
				_ = writer.send(r.Context(), map[string]any{"e": "agent_error", "message": "prompt is required"})
				continue
			}
			credentials := message.Credentials.normalized()
			if err := credentials.validate(); err != nil {
				_ = writer.send(r.Context(), map[string]any{"e": "credentials_required", "message": err.Error()})
				continue
			}
			runner := s.runner
			usage, err := s.store.DemoUsage(r.Context(), userID(r.Context()), s.cfg.DemoRunLimit)
			if err != nil {
				_ = writer.send(r.Context(), map[string]any{"e": "agent_error", "message": "unable to check demo allowance"})
				continue
			}
			if credentials.complete() {
				runner = agent.NewRunner(
					s.store,
					provider.NewOpenRouter(credentials.OpenRouterAPIKey, s.cfg.OpenRouterModel),
					provider.NewE2B(credentials.E2BAPIKey, s.cfg.E2BTemplateAlias, s.cfg.SandboxTimeout),
					s.cfg.AgentMaxSteps,
				)
			} else {
				var claimed bool
				usage, claimed, err = s.store.ClaimDemoRun(r.Context(), userID(r.Context()), s.cfg.DemoRunLimit)
				if err != nil {
					_ = writer.send(r.Context(), map[string]any{"e": "agent_error", "message": "unable to reserve demo build"})
					continue
				}
				if !claimed {
					_ = writer.send(r.Context(), map[string]any{
						"e":       "credentials_required",
						"message": "Your two demo builds are used. Add your OpenRouter and E2B API keys to continue.",
						"demo":    usage,
					})
					continue
				}
			}
			_ = writer.send(r.Context(), map[string]any{"e": "usage_updated", "demo": usage, "usingOwnKeys": credentials.complete()})
			runCtx, cancel := context.WithTimeout(r.Context(), s.cfg.AgentTimeout)
			err = runner.Run(runCtx, userID(r.Context()), projectID, message.Prompt, func(event map[string]any) error {
				return writer.send(runCtx, event)
			})
			cancel()
			if err != nil && runCtx.Err() == context.DeadlineExceeded {
				_ = writer.send(r.Context(), map[string]any{"e": "agent_error", "message": "agent timed out"})
			}
		default:
			_ = writer.send(r.Context(), map[string]any{"e": "error", "message": fmt.Sprintf("unknown message type %q", message.Type)})
		}
	}
}

func originHost(origin string) string {
	parsed, err := url.Parse(origin)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return origin
}

package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

type socketWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
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
			Type   string `json:"type"`
			Prompt string `json:"prompt"`
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
			runCtx, cancel := context.WithTimeout(r.Context(), s.cfg.AgentTimeout)
			err := s.runner.Run(runCtx, userID(r.Context()), projectID, message.Prompt, func(event map[string]any) error {
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

package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const (
	e2bAPIBase = "https://api.e2b.app"
	envdPort   = 49983
)

type E2B struct {
	apiKey   string
	template string
	timeout  time.Duration
	client   *http.Client
}

// StatusError wraps a non-2xx E2B API response so callers can branch on the
// HTTP status (e.g. treat 404 as "already gone") instead of string-matching
// the error message.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("e2b status %d: %s", e.StatusCode, e.Body)
}

// IsNotFound reports whether err is an E2B StatusError for a 404 response,
// e.g. killing a sandbox that E2B has already reaped.
func IsNotFound(err error) bool {
	var statusErr *StatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound
}

type Sandbox struct {
	TemplateID         string  `json:"templateID"`
	SandboxID          string  `json:"sandboxID"`
	Alias              string  `json:"alias"`
	EnvdAccessToken    *string `json:"envdAccessToken"`
	TrafficAccessToken *string `json:"trafficAccessToken"`
	State              string  `json:"state,omitempty"`
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type DirEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Permissions string `json:"permissions"`
}

func NewE2B(apiKey, template string, timeout time.Duration) *E2B {
	return &E2B{
		apiKey:   apiKey,
		template: template,
		timeout:  timeout,
		client:   &http.Client{Timeout: 3 * time.Minute},
	}
}

func (e *E2B) Create(ctx context.Context, projectID string) (Sandbox, error) {
	body := map[string]any{
		"templateID":            e.template,
		"timeout":               int(e.timeout.Seconds()),
		"secure":                true,
		"allow_internet_access": true,
		"metadata":              map[string]string{"project_id": projectID, "app": "cutable"},
	}
	var sandbox Sandbox
	if err := e.apiJSON(ctx, http.MethodPost, "/sandboxes", body, &sandbox); err != nil {
		return Sandbox{}, err
	}
	if sandbox.SandboxID == "" || sandbox.EnvdAccessToken == nil || *sandbox.EnvdAccessToken == "" {
		return Sandbox{}, errors.New("e2b returned incomplete secured sandbox credentials")
	}
	return sandbox, nil
}

func (e *E2B) Connect(ctx context.Context, sandboxID string) (Sandbox, error) {
	var sandbox Sandbox
	err := e.apiJSON(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(sandboxID)+"/connect",
		map[string]any{"timeout": int(e.timeout.Seconds())}, &sandbox)
	return sandbox, err
}

func (e *E2B) Get(ctx context.Context, sandboxID string) (Sandbox, error) {
	var sandbox Sandbox
	err := e.apiJSON(ctx, http.MethodGet, "/sandboxes/"+url.PathEscape(sandboxID), nil, &sandbox)
	return sandbox, err
}

func (e *E2B) Kill(ctx context.Context, sandboxID string) error {
	return e.apiJSON(ctx, http.MethodDelete, "/sandboxes/"+url.PathEscape(sandboxID), nil, nil)
}

func (e *E2B) PreviewURL(sandboxID string) string {
	return fmt.Sprintf("https://5173-%s.e2b.app", sandboxID)
}

func (e *E2B) WriteFile(ctx context.Context, sandboxID, accessToken, path string, content []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	endpoint := e.envdURL(sandboxID, "/files")
	query := endpoint.Query()
	query.Set("path", path)
	query.Set("username", "user")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	e.setEnvdHeaders(req, sandboxID, accessToken)
	return e.doOK(req, nil)
}

func (e *E2B) ReadFile(ctx context.Context, sandboxID, accessToken, path string) ([]byte, error) {
	endpoint := e.envdURL(sandboxID, "/files")
	query := endpoint.Query()
	query.Set("path", path)
	query.Set("username", "user")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	e.setEnvdHeaders(req, sandboxID, accessToken)
	var content []byte
	if err := e.doOK(req, &content); err != nil {
		return nil, err
	}
	return content, nil
}

func (e *E2B) RemoveFile(ctx context.Context, sandboxID, accessToken, path string) error {
	return e.envdJSON(ctx, sandboxID, accessToken, "/filesystem.Filesystem/Remove", map[string]any{"path": path}, nil)
}

func (e *E2B) MoveFile(ctx context.Context, sandboxID, accessToken, oldPath, newPath string) error {
	return e.envdJSON(ctx, sandboxID, accessToken, "/filesystem.Filesystem/Move",
		map[string]any{"source": oldPath, "destination": newPath}, nil)
}

func (e *E2B) ListDir(ctx context.Context, sandboxID, accessToken, path string, depth int) ([]DirEntry, error) {
	var result struct {
		Entries []DirEntry `json:"entries"`
	}
	err := e.envdJSON(ctx, sandboxID, accessToken, "/filesystem.Filesystem/ListDir",
		map[string]any{"path": path, "depth": depth}, &result)
	return result.Entries, err
}

func (e *E2B) Run(ctx context.Context, sandboxID, accessToken, command, cwd string) (CommandResult, error) {
	payload, err := json.Marshal(map[string]any{
		"process": map[string]any{
			"cmd":  "/bin/bash",
			"args": []string{"-lc", command},
			"envs": map[string]string{},
			"cwd":  cwd,
		},
		"stdin": false,
	})
	if err != nil {
		return CommandResult{}, err
	}
	framed := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(framed[1:5], uint32(len(payload)))
	copy(framed[5:], payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.envdURL(sandboxID, "/process.Process/Start").String(), bytes.NewReader(framed))
	if err != nil {
		return CommandResult{}, err
	}
	e.setEnvdHeaders(req, sandboxID, accessToken)
	req.Header.Set("Content-Type", "application/connect+json")
	res, err := e.client.Do(req)
	if err != nil {
		return CommandResult{}, fmt.Errorf("e2b command request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return CommandResult{}, fmt.Errorf("e2b command status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return decodeCommandStream(res.Body)
}

func (e *E2B) apiJSON(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, e2bAPIBase+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", e.apiKey)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return e.doOK(req, output)
}

func (e *E2B) envdJSON(ctx context.Context, sandboxID, accessToken, path string, input, output any) error {
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.envdURL(sandboxID, path).String(), bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	e.setEnvdHeaders(req, sandboxID, accessToken)
	req.Header.Set("Content-Type", "application/json")
	return e.doOK(req, output)
}

func (e *E2B) setEnvdHeaders(req *http.Request, sandboxID, accessToken string) {
	req.Header.Set("E2b-Sandbox-Id", sandboxID)
	req.Header.Set("E2b-Sandbox-Port", fmt.Sprint(envdPort))
	req.Header.Set("X-Access-Token", accessToken)
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:")))
}

func (e *E2B) envdURL(sandboxID, path string) *url.URL {
	return &url.URL{Scheme: "https", Host: "sandbox.e2b.app", Path: path}
}

func (e *E2B) doOK(req *http.Request, output any) error {
	res, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return &StatusError{StatusCode: res.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	if output == nil {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}
	if target, ok := output.(*[]byte); ok {
		content, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		if err == nil {
			*target = content
		}
		return err
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(output)
}

func decodeCommandStream(reader io.Reader) (CommandResult, error) {
	all, err := io.ReadAll(io.LimitReader(reader, 32<<20))
	if err != nil {
		return CommandResult{}, err
	}
	var payloads [][]byte
	trimmed := bytes.TrimSpace(all)
	if len(trimmed) == 0 {
		return CommandResult{}, errors.New("e2b command returned an empty stream")
	}
	if trimmed[0] == '{' {
		scanner := bufio.NewScanner(bytes.NewReader(trimmed))
		for scanner.Scan() {
			payloads = append(payloads, bytes.Clone(scanner.Bytes()))
		}
	} else {
		for len(all) >= 5 {
			length := int(binary.BigEndian.Uint32(all[1:5]))
			if length < 0 || length > len(all)-5 {
				break
			}
			payload := bytes.Clone(all[5 : 5+length])
			if all[0]&0x02 != 0 {
				var trailer struct {
					Error *struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}
				if json.Unmarshal(payload, &trailer) == nil && trailer.Error != nil {
					return CommandResult{}, fmt.Errorf("e2b command %s: %s", trailer.Error.Code, trailer.Error.Message)
				}
			} else {
				payloads = append(payloads, payload)
			}
			all = all[5+length:]
		}
	}
	result := CommandResult{}
	for _, payload := range payloads {
		var event any
		if json.Unmarshal(payload, &event) != nil {
			continue
		}
		collectCommandEvent(event, &result)
	}
	return result, nil
}

func collectCommandEvent(value any, result *CommandResult) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch key {
			case "stdout":
				if text, ok := child.(string); ok {
					if decoded, err := base64.StdEncoding.DecodeString(text); err == nil {
						result.Stdout += string(decoded)
					} else {
						result.Stdout += text
					}
				}
			case "stderr":
				if text, ok := child.(string); ok {
					if decoded, err := base64.StdEncoding.DecodeString(text); err == nil {
						result.Stderr += string(decoded)
					} else {
						result.Stderr += text
					}
				}
			case "exitCode", "exit_code":
				if number, ok := child.(float64); ok {
					result.ExitCode = int(number)
				}
			default:
				collectCommandEvent(child, result)
			}
		}
	case []any:
		for _, child := range typed {
			collectCommandEvent(child, result)
		}
	}
}

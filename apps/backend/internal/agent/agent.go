package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/provider"
	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

const appRoot = "/home/user/react-app"

type EventSender func(event map[string]any) error

type Runner struct {
	store    *store.Store
	model    *provider.OpenRouter
	sandbox  *provider.E2B
	maxSteps int
}

type session struct {
	project     store.Project
	sandboxID   string
	accessToken string
	send        EventSender
}

func NewRunner(database *store.Store, model *provider.OpenRouter, sandbox *provider.E2B, maxSteps int) *Runner {
	return &Runner{store: database, model: model, sandbox: sandbox, maxSteps: maxSteps}
}

func (r *Runner) Run(ctx context.Context, userID, projectID uuid.UUID, prompt string, send EventSender) error {
	project, err := r.store.Project(ctx, userID, projectID)
	if err != nil {
		return err
	}
	s := &session{project: project, send: send}
	if err := send(event("agent_started", "message", "Starting Cutable agent...")); err != nil {
		return err
	}
	if err := send(stage("initializing", "Preparing development environment", 5)); err != nil {
		return err
	}
	if err := r.ensureSandbox(ctx, s); err != nil {
		_ = send(event("stage_error", "message", err.Error()))
		return err
	}
	if err := send(stage("planning", "Creating an implementation plan", 15)); err != nil {
		return err
	}
	plan, err := r.createPlan(ctx, prompt)
	if err != nil {
		_ = send(event("plan_error", "message", err.Error()))
		return err
	}
	if err := send(map[string]any{"e": "plan_generated", "plan": plan}); err != nil {
		return err
	}
	if err := send(stage("executing", "Building your application", 25)); err != nil {
		return err
	}

	messages := []provider.Message{
		{Role: "system", Content: systemPrompt()},
		{Role: "user", Content: prompt},
	}
	tools := toolDefinitions()

	for step := 0; step < r.maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := send(event("agent_thinking", "message", "Working on the application...")); err != nil {
			return err
		}
		response, err := r.model.Complete(ctx, messages, tools)
		if err != nil {
			_ = send(event("agent_error", "message", err.Error()))
			return err
		}
		messages = append(messages, response)
		if len(response.ToolCalls) == 0 {
			final := strings.TrimSpace(response.Content)
			if final == "" {
				final = "Application completed."
			}
			_, _ = r.store.AddConversation(ctx, projectID, "AGENT", "TEXT_MESSAGE", final, nil)
			_ = send(stage("complete", "Application ready", 100))
			_ = send(event("agent_final_response", "message", final))
			_ = send(event("agent_completed", "message", "Completed"))
			return nil
		}
		for _, call := range response.ToolCalls {
			displayName := displayToolName(call.Function.Name)
			_ = send(map[string]any{"e": "tool_started", "tool": displayName, "input": json.RawMessage(call.Function.Arguments)})
			output, toolErr := r.executeTool(ctx, s, call.Function.Name, call.Function.Arguments)
			if toolErr != nil {
				output = "ERROR: " + toolErr.Error()
				_ = send(map[string]any{"e": "tool_error", "tool": displayName, "error": toolErr.Error()})
			} else {
				_ = send(map[string]any{"e": "tool_completed", "tool": displayName, "output": truncate(output, 1200)})
			}
			messages = append(messages, provider.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    output,
			})
		}
		progress := 25 + ((step + 1) * 60 / r.maxSteps)
		_ = send(stage("executing", "Building your application", progress))
	}
	err = fmt.Errorf("agent reached the maximum of %d tool steps", r.maxSteps)
	_ = send(event("agent_error", "message", err.Error()))
	return err
}

func (r *Runner) createPlan(ctx context.Context, prompt string) ([]string, error) {
	response, err := r.model.Complete(ctx, []provider.Message{
		{Role: "system", Content: "Create a concise 3-6 step implementation plan for a React web application. Return only numbered steps."},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, err
	}
	var plan []string
	re := regexp.MustCompile(`^\s*(?:\d+[\.\)]|[-*])\s*`)
	for _, line := range strings.Split(response.Content, "\n") {
		line = strings.TrimSpace(re.ReplaceAllString(line, ""))
		if line != "" {
			plan = append(plan, line)
		}
	}
	if len(plan) == 0 {
		return []string{"Understand the request", "Build the interface", "Verify the production build"}, nil
	}
	if len(plan) > 6 {
		plan = plan[:6]
	}
	return plan, nil
}

func (r *Runner) ensureSandbox(ctx context.Context, s *session) error {
	if s.project.SandboxID != nil && *s.project.SandboxID != "" {
		connected, err := r.sandbox.Connect(ctx, *s.project.SandboxID)
		if err == nil && connected.EnvdAccessToken != nil {
			s.sandboxID = connected.SandboxID
			s.accessToken = *connected.EnvdAccessToken
			return nil
		}
	}
	created, err := r.sandbox.Create(ctx, s.project.ID.String())
	if err != nil {
		return fmt.Errorf("create E2B sandbox: %w", err)
	}
	s.sandboxID = created.SandboxID
	s.accessToken = *created.EnvdAccessToken
	preview := r.sandbox.PreviewURL(created.SandboxID)
	if err := r.store.SaveSandbox(ctx, s.project.ID, created.SandboxID, preview); err != nil {
		_ = r.sandbox.Kill(context.Background(), created.SandboxID)
		return err
	}
	return s.send(map[string]any{"e": "sandbox_created", "sandboxId": created.SandboxID, "previewUrl": preview})
}

func (r *Runner) executeTool(ctx context.Context, s *session, name, rawArguments string) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(rawArguments), &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}
	switch name {
	case "write_file":
		filePath, err := safePath(stringArg(args, "path"))
		if err != nil {
			return "", err
		}
		content := stringArg(args, "content")
		if err := r.sandbox.WriteFile(ctx, s.sandboxID, s.accessToken, filePath, []byte(content)); err != nil {
			return "", err
		}
		relative := strings.TrimPrefix(filePath, appRoot+"/")
		if err := r.store.UpsertFile(ctx, s.project.ID, relative, content); err != nil {
			return "", err
		}
		_ = s.send(event("file_created", "filepath", relative))
		return "Wrote " + relative, nil

	case "write_multiple_files":
		items, ok := args["files"].([]any)
		if !ok || len(items) == 0 {
			return "", errors.New("files must be a non-empty array")
		}
		var written []string
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				return "", errors.New("each file must be an object")
			}
			filePath, err := safePath(stringArg(entry, "path"))
			if err != nil {
				return "", err
			}
			content := stringArg(entry, "content")
			if err := r.sandbox.WriteFile(ctx, s.sandboxID, s.accessToken, filePath, []byte(content)); err != nil {
				return "", err
			}
			relative := strings.TrimPrefix(filePath, appRoot+"/")
			if err := r.store.UpsertFile(ctx, s.project.ID, relative, content); err != nil {
				return "", err
			}
			written = append(written, relative)
		}
		_ = s.send(map[string]any{"e": "files_created", "files": written, "count": len(written)})
		return fmt.Sprintf("Wrote %d files: %s", len(written), strings.Join(written, ", ")), nil

	case "read_file":
		filePath, err := safePath(stringArg(args, "path"))
		if err != nil {
			return "", err
		}
		content, err := r.sandbox.ReadFile(ctx, s.sandboxID, s.accessToken, filePath)
		if err != nil {
			return "", err
		}
		relative := strings.TrimPrefix(filePath, appRoot+"/")
		_ = s.send(event("file_read", "filepath", relative))
		return truncate(string(content), 20000), nil

	case "delete_file":
		filePath, err := safePath(stringArg(args, "path"))
		if err != nil {
			return "", err
		}
		if err := r.sandbox.RemoveFile(ctx, s.sandboxID, s.accessToken, filePath); err != nil {
			return "", err
		}
		relative := strings.TrimPrefix(filePath, appRoot+"/")
		_ = r.store.DeleteFile(ctx, s.project.ID, relative)
		_ = s.send(event("file_deleted", "filepath", relative))
		return "Deleted " + relative, nil

	case "rename_file":
		oldPath, err := safePath(stringArg(args, "old_path"))
		if err != nil {
			return "", err
		}
		newPath, err := safePath(stringArg(args, "new_path"))
		if err != nil {
			return "", err
		}
		if err := r.sandbox.MoveFile(ctx, s.sandboxID, s.accessToken, oldPath, newPath); err != nil {
			return "", err
		}
		oldRelative := strings.TrimPrefix(oldPath, appRoot+"/")
		newRelative := strings.TrimPrefix(newPath, appRoot+"/")
		_ = r.store.RenameFile(ctx, s.project.ID, oldRelative, newRelative)
		_ = s.send(map[string]any{"e": "file_renamed", "oldPath": oldRelative, "newPath": newRelative})
		return fmt.Sprintf("Renamed %s to %s", oldRelative, newRelative), nil

	case "list_directories":
		entries, err := r.sandbox.ListDir(ctx, s.sandboxID, s.accessToken, appRoot, 4)
		if err != nil {
			return "", err
		}
		encoded, _ := json.Marshal(entries)
		return string(encoded), nil

	case "execute_command":
		command := strings.TrimSpace(stringArg(args, "command"))
		if command == "" {
			return "", errors.New("command is required")
		}
		return r.runCommand(ctx, s, command)

	case "add_dependency":
		packages := strings.TrimSpace(stringArg(args, "packages"))
		if packages == "" {
			return "", errors.New("packages is required")
		}
		return r.runCommand(ctx, s, "npm install "+packages)

	case "test_build":
		_ = s.send(event("build_started", "message", "Running production build"))
		output, err := r.runCommand(ctx, s, "npm run build")
		if err != nil {
			_ = s.send(event("build_test_failed", "message", err.Error()))
			return output, err
		}
		_ = s.send(event("build_test_success", "message", "Build successful"))
		return output, nil

	case "check_missing_dependencies":
		return r.runCommand(ctx, s, "npm install --dry-run")

	case "start_dev_server":
		result, err := r.sandbox.Run(ctx, s.sandboxID, s.accessToken,
			"pkill -f '[v]ite.*--port 5173' >/dev/null 2>&1 || true; nohup npm run dev -- --host 0.0.0.0 --port 5173 >/tmp/cutable-vite.log 2>&1 </dev/null & sleep 3; curl -fsS http://127.0.0.1:5173 >/dev/null",
			appRoot)
		if err != nil || result.ExitCode != 0 {
			return result.Stdout + result.Stderr, fmt.Errorf("start preview failed: %s", strings.TrimSpace(result.Stderr))
		}
		return "Preview running at " + r.sandbox.PreviewURL(s.sandboxID), nil
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func (r *Runner) runCommand(ctx context.Context, s *session, command string) (string, error) {
	result, err := r.sandbox.Run(ctx, s.sandboxID, s.accessToken, command, appRoot)
	output := strings.TrimSpace(result.Stdout + result.Stderr)
	if err != nil {
		return output, err
	}
	if result.ExitCode != 0 {
		return output, fmt.Errorf("command exited with code %d: %s", result.ExitCode, truncate(output, 3000))
	}
	return truncate(output, 12000), nil
}

func safePath(input string) (string, error) {
	input = strings.TrimSpace(strings.ReplaceAll(input, "\\", "/"))
	if input == "" {
		return "", errors.New("path is required")
	}
	if strings.HasPrefix(input, appRoot+"/") {
		input = strings.TrimPrefix(input, appRoot+"/")
	}
	input = strings.TrimPrefix(input, "/")
	clean := path.Clean(input)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("path must stay inside the React application")
	}
	return path.Join(appRoot, clean), nil
}

func stringArg(args map[string]any, name string) string {
	value, _ := args[name].(string)
	return value
}

func displayToolName(name string) string {
	return strings.ReplaceAll(name, "_", "-")
}

func event(name string, key string, value any) map[string]any {
	return map[string]any{"e": name, key: value}
}

func stage(name, message string, progress int) map[string]any {
	return map[string]any{"e": "stage_update", "stage": name, "message": message, "progress": progress}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "\n...[truncated]"
}

func systemPrompt() string {
	return `You are Cutable, an AI coding agent. Build polished React + TypeScript applications in the existing Vite project at /home/user/react-app.

Rules:
- Never scaffold a new application. The project already exists.
- Inspect existing files before editing when useful.
- Use write_file or write_multiple_files for source changes.
- Keep all paths inside /home/user/react-app.
- Use Tailwind CSS v4 and semantic, accessible styling.
- Do not expose secrets or add server-side credentials to generated client code.
- Run test_build after changes.
- Fix any build errors, then call start_dev_server.
- Finish with a concise summary only after the production build and preview succeed.`
}

func toolDefinitions() []provider.ToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	stringProp := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	return []provider.ToolDefinition{
		{Type: "function", Function: provider.ToolFunction{Name: "write_file", Description: "Create or overwrite one file in the React app.", Parameters: object(map[string]any{"path": stringProp("Path relative to the React app"), "content": stringProp("Complete file contents")}, "path", "content")}},
		{Type: "function", Function: provider.ToolFunction{Name: "write_multiple_files", Description: "Create or overwrite multiple files.", Parameters: object(map[string]any{"files": map[string]any{"type": "array", "items": object(map[string]any{"path": stringProp("Relative path"), "content": stringProp("Complete contents")}, "path", "content")}}, "files")}},
		{Type: "function", Function: provider.ToolFunction{Name: "read_file", Description: "Read a file from the React app.", Parameters: object(map[string]any{"path": stringProp("Relative path")}, "path")}},
		{Type: "function", Function: provider.ToolFunction{Name: "delete_file", Description: "Delete a file.", Parameters: object(map[string]any{"path": stringProp("Relative path")}, "path")}},
		{Type: "function", Function: provider.ToolFunction{Name: "rename_file", Description: "Rename or move a file.", Parameters: object(map[string]any{"old_path": stringProp("Existing relative path"), "new_path": stringProp("New relative path")}, "old_path", "new_path")}},
		{Type: "function", Function: provider.ToolFunction{Name: "list_directories", Description: "List files and directories in the React app.", Parameters: object(map[string]any{})}},
		{Type: "function", Function: provider.ToolFunction{Name: "execute_command", Description: "Run a shell command in the isolated React app directory.", Parameters: object(map[string]any{"command": stringProp("Command to execute")}, "command")}},
		{Type: "function", Function: provider.ToolFunction{Name: "add_dependency", Description: "Install one or more npm packages.", Parameters: object(map[string]any{"packages": stringProp("Space-separated package specifications")}, "packages")}},
		{Type: "function", Function: provider.ToolFunction{Name: "check_missing_dependencies", Description: "Check npm dependency resolution.", Parameters: object(map[string]any{})}},
		{Type: "function", Function: provider.ToolFunction{Name: "test_build", Description: "Run the production Vite build.", Parameters: object(map[string]any{})}},
		{Type: "function", Function: provider.ToolFunction{Name: "start_dev_server", Description: "Start and verify the Vite preview server. Call only after a successful build.", Parameters: object(map[string]any{})}},
	}
}

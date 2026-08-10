package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/provider"
	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func newTestSession(sandbox *fakeSandbox) *session {
	return &session{
		project:     store.Project{ID: uuid.New()},
		sandboxID:   "sandbox-1",
		accessToken: "token-1",
		send:        func(map[string]any) error { return nil },
	}
}

func newTestRunner(sandbox *fakeSandbox, database *fakeStore, model *fakeModel) *Runner {
	return NewRunner(database, model, sandbox, 10, testLogger())
}

func toolCallArgs(t *testing.T, args map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return string(encoded)
}

func TestExecuteToolWriteFile(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	_, send := collectEvents()
	s := newTestSession(sandbox)
	s.send = send
	s.project = database.project

	output, err := runner.executeTool(context.Background(), s, "write_file", toolCallArgs(t, map[string]any{
		"path": "src/App.tsx", "content": "export default function App() {}",
	}))
	if err != nil {
		t.Fatalf("executeTool(write_file) error = %v", err)
	}
	if output != "Wrote src/App.tsx" {
		t.Fatalf("output = %q", output)
	}
	if sandbox.files[appRoot+"/src/App.tsx"] == nil {
		t.Fatal("expected file written to sandbox")
	}
	if database.upsertedFiles["src/App.tsx"] != "export default function App() {}" {
		t.Fatalf("expected file persisted to store, got %#v", database.upsertedFiles)
	}
}

func TestExecuteToolWriteFileRejectsPathTraversal(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	_, err := runner.executeTool(context.Background(), s, "write_file", toolCallArgs(t, map[string]any{
		"path": "../../etc/passwd", "content": "pwned",
	}))
	if err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
	if len(sandbox.files) != 0 {
		t.Fatalf("expected no files written, got %#v", sandbox.files)
	}
}

func TestExecuteToolWriteMultipleFiles(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	output, err := runner.executeTool(context.Background(), s, "write_multiple_files", toolCallArgs(t, map[string]any{
		"files": []any{
			map[string]any{"path": "a.ts", "content": "a"},
			map[string]any{"path": "b.ts", "content": "b"},
		},
	}))
	if err != nil {
		t.Fatalf("executeTool(write_multiple_files) error = %v", err)
	}
	if len(database.upsertedFiles) != 2 {
		t.Fatalf("expected 2 files persisted, got %#v", database.upsertedFiles)
	}
	if output == "" {
		t.Fatal("expected non-empty summary output")
	}
}

func TestExecuteToolWriteMultipleFilesRequiresNonEmpty(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "write_multiple_files", toolCallArgs(t, map[string]any{
		"files": []any{},
	})); err == nil {
		t.Fatal("expected error for empty files array")
	}
}

func TestExecuteToolReadFile(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.files[appRoot+"/src/App.tsx"] = []byte("hello world")
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	output, err := runner.executeTool(context.Background(), s, "read_file", toolCallArgs(t, map[string]any{"path": "src/App.tsx"}))
	if err != nil {
		t.Fatalf("executeTool(read_file) error = %v", err)
	}
	if output != "hello world" {
		t.Fatalf("output = %q, want %q", output, "hello world")
	}
}

func TestExecuteToolReadFileMissing(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "read_file", toolCallArgs(t, map[string]any{"path": "missing.ts"})); err == nil {
		t.Fatal("expected error reading a nonexistent file")
	}
}

func TestExecuteToolDeleteFile(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.files[appRoot+"/old.ts"] = []byte("x")
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "delete_file", toolCallArgs(t, map[string]any{"path": "old.ts"})); err != nil {
		t.Fatalf("executeTool(delete_file) error = %v", err)
	}
	if _, ok := sandbox.files[appRoot+"/old.ts"]; ok {
		t.Fatal("expected file removed from sandbox")
	}
	if len(database.deletedFiles) != 1 || database.deletedFiles[0] != "old.ts" {
		t.Fatalf("expected store deletion recorded, got %#v", database.deletedFiles)
	}
}

func TestExecuteToolRenameFile(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.files[appRoot+"/old.ts"] = []byte("x")
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	output, err := runner.executeTool(context.Background(), s, "rename_file", toolCallArgs(t, map[string]any{
		"old_path": "old.ts", "new_path": "new.ts",
	}))
	if err != nil {
		t.Fatalf("executeTool(rename_file) error = %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if database.renamedFiles["old.ts"] != "new.ts" {
		t.Fatalf("expected rename recorded, got %#v", database.renamedFiles)
	}
	if _, ok := sandbox.files[appRoot+"/new.ts"]; !ok {
		t.Fatal("expected file present at new path in sandbox")
	}
}

func TestExecuteToolListDirectories(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.files[appRoot+"/src/App.tsx"] = []byte("x")
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	output, err := runner.executeTool(context.Background(), s, "list_directories", toolCallArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("executeTool(list_directories) error = %v", err)
	}
	var entries []provider.DirEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestExecuteToolExecuteCommandRequiresCommand(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "execute_command", toolCallArgs(t, map[string]any{"command": "  "})); err == nil {
		t.Fatal("expected error for blank command")
	}
}

func TestExecuteToolExecuteCommandSuccess(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.runResult = provider.CommandResult{Stdout: "ok", ExitCode: 0}
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	output, err := runner.executeTool(context.Background(), s, "execute_command", toolCallArgs(t, map[string]any{"command": "ls"}))
	if err != nil {
		t.Fatalf("executeTool(execute_command) error = %v", err)
	}
	if output != "ok" {
		t.Fatalf("output = %q", output)
	}
}

func TestExecuteToolExecuteCommandNonZeroExit(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.runResult = provider.CommandResult{Stderr: "boom", ExitCode: 1}
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "execute_command", toolCallArgs(t, map[string]any{"command": "false"})); err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
}

func TestExecuteToolTestBuildEmitsSuccessEvent(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.runResult = provider.CommandResult{Stdout: "built", ExitCode: 0}
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	events, send := collectEvents()
	s := newTestSession(sandbox)
	s.send = send
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "test_build", toolCallArgs(t, map[string]any{})); err != nil {
		t.Fatalf("executeTool(test_build) error = %v", err)
	}
	types := eventTypes(*events)
	if len(types) != 2 || types[0] != "build_started" || types[1] != "build_test_success" {
		t.Fatalf("event sequence = %v, want [build_started build_test_success]", types)
	}
}

func TestExecuteToolTestBuildEmitsFailureEvent(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.runResult = provider.CommandResult{Stderr: "type error", ExitCode: 1}
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	events, send := collectEvents()
	s := newTestSession(sandbox)
	s.send = send
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "test_build", toolCallArgs(t, map[string]any{})); err == nil {
		t.Fatal("expected build failure to return an error")
	}
	types := eventTypes(*events)
	if len(types) != 2 || types[0] != "build_started" || types[1] != "build_test_failed" {
		t.Fatalf("event sequence = %v, want [build_started build_test_failed]", types)
	}
}

func TestExecuteToolStartDevServerSuccess(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.runResult = provider.CommandResult{ExitCode: 0}
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	output, err := runner.executeTool(context.Background(), s, "start_dev_server", toolCallArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("executeTool(start_dev_server) error = %v", err)
	}
	if output == "" {
		t.Fatal("expected preview URL in output")
	}
}

func TestExecuteToolUnknownTool(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "not_a_real_tool", "{}"); err == nil {
		t.Fatal("expected error for unknown tool name")
	}
}

func TestExecuteToolInvalidJSONArguments(t *testing.T) {
	sandbox := newFakeSandbox()
	database := newFakeStore(store.Project{ID: uuid.New()})
	runner := newTestRunner(sandbox, database, &fakeModel{})
	s := newTestSession(sandbox)
	s.project = database.project

	if _, err := runner.executeTool(context.Background(), s, "write_file", "{not json"); err == nil {
		t.Fatal("expected error for invalid JSON arguments")
	}
}

// --- Run() control flow ---

func TestRunCompletesWithoutToolCalls(t *testing.T) {
	sandbox := newFakeSandbox()
	sandbox.runResult = provider.CommandResult{ExitCode: 0}
	project := store.Project{ID: uuid.New()}
	database := newFakeStore(project)
	model := &fakeModel{
		responses: []provider.Message{
			{Role: "assistant", Content: "1. Plan step"}, // createPlan
			{Role: "assistant", Content: "All done!"},    // final response, no tool calls
		},
	}
	runner := newTestRunner(sandbox, database, model)
	events, send := collectEvents()

	if err := runner.Run(context.Background(), uuid.New(), project.ID, "build me an app", send); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	types := eventTypes(*events)
	if types[len(types)-1] != "agent_completed" {
		t.Fatalf("expected run to end with agent_completed, got %v", types)
	}
	if len(database.conversations) != 1 || database.conversations[0].Contents != "All done!" {
		t.Fatalf("expected final response persisted to conversation history, got %#v", database.conversations)
	}
}

func TestRunExecutesToolCallThenCompletes(t *testing.T) {
	sandbox := newFakeSandbox()
	project := store.Project{ID: uuid.New()}
	database := newFakeStore(project)
	toolArgs := toolCallArgs(t, map[string]any{"path": "src/App.tsx", "content": "hi"})
	model := &fakeModel{
		responses: []provider.Message{
			{Role: "assistant", Content: "1. Write the app"},
			{Role: "assistant", ToolCalls: []provider.ToolCall{
				{ID: "call-1", Function: provider.FunctionCall{Name: "write_file", Arguments: toolArgs}},
			}},
			{Role: "assistant", Content: "Finished writing the app."},
		},
	}
	runner := newTestRunner(sandbox, database, model)
	events, send := collectEvents()

	if err := runner.Run(context.Background(), uuid.New(), project.ID, "build me an app", send); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	types := eventTypes(*events)
	if !containsString(types, "tool_started") || !containsString(types, "tool_completed") {
		t.Fatalf("expected tool_started/tool_completed events, got %v", types)
	}
	if sandbox.files[appRoot+"/src/App.tsx"] == nil {
		t.Fatal("expected the tool call to actually write the file")
	}
}

func TestRunHandlesToolError(t *testing.T) {
	sandbox := newFakeSandbox()
	project := store.Project{ID: uuid.New()}
	database := newFakeStore(project)
	badArgs := toolCallArgs(t, map[string]any{"path": "missing.ts"})
	model := &fakeModel{
		responses: []provider.Message{
			{Role: "assistant", Content: "1. Read a file"},
			{Role: "assistant", ToolCalls: []provider.ToolCall{
				{ID: "call-1", Function: provider.FunctionCall{Name: "read_file", Arguments: badArgs}},
			}},
			{Role: "assistant", Content: "Handled the error."},
		},
	}
	runner := newTestRunner(sandbox, database, model)
	events, send := collectEvents()

	if err := runner.Run(context.Background(), uuid.New(), project.ID, "read a file", send); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !containsString(eventTypes(*events), "tool_error") {
		t.Fatalf("expected a tool_error event, got %v", eventTypes(*events))
	}
}

func TestRunReturnsErrorWhenModelFails(t *testing.T) {
	sandbox := newFakeSandbox()
	project := store.Project{ID: uuid.New()}
	database := newFakeStore(project)
	model := &fakeModel{
		responses: []provider.Message{{Role: "assistant", Content: "1. Plan"}},
		errs:      []error{nil, errors.New("model unavailable")},
	}
	runner := newTestRunner(sandbox, database, model)
	events, send := collectEvents()

	err := runner.Run(context.Background(), uuid.New(), project.ID, "build me an app", send)
	if err == nil {
		t.Fatal("expected Run to return an error when the model call fails")
	}
	if !containsString(eventTypes(*events), "agent_error") {
		t.Fatalf("expected an agent_error event, got %v", eventTypes(*events))
	}
}

func TestRunStopsAtMaxSteps(t *testing.T) {
	sandbox := newFakeSandbox()
	project := store.Project{ID: uuid.New()}
	database := newFakeStore(project)
	loopArgs := toolCallArgs(t, map[string]any{"command": "echo hi"})
	sandbox.runResult = provider.CommandResult{Stdout: "hi", ExitCode: 0}
	// Every model response after the plan keeps requesting a tool call, so
	// the loop never naturally terminates and must be stopped by maxSteps.
	loopingResponse := provider.Message{Role: "assistant", ToolCalls: []provider.ToolCall{
		{ID: "call-1", Function: provider.FunctionCall{Name: "execute_command", Arguments: loopArgs}},
	}}
	model := &fakeModel{responses: []provider.Message{
		{Role: "assistant", Content: "1. Loop forever"},
		loopingResponse,
	}}
	runner := NewRunner(database, model, sandbox, 2, testLogger())
	events, send := collectEvents()

	err := runner.Run(context.Background(), uuid.New(), project.ID, "loop", send)
	if err == nil {
		t.Fatal("expected an error when the agent exhausts maxSteps")
	}
	if !containsString(eventTypes(*events), "agent_error") {
		t.Fatalf("expected a final agent_error event, got %v", eventTypes(*events))
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

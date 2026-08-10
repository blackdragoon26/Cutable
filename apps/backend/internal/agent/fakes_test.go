package agent

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/blackdragoon26/cutable/apps/backend/internal/provider"
	"github.com/blackdragoon26/cutable/apps/backend/internal/store"
)

// fakeSandbox is an in-memory stand-in for *provider.E2B, letting agent-loop
// tests exercise every executeTool branch without a real E2B environment.
type fakeSandbox struct {
	files map[string][]byte

	createErr  error
	connectErr error
	killCalls  []string
	runResult  provider.CommandResult
	runErr     error
}

func newFakeSandbox() *fakeSandbox {
	return &fakeSandbox{files: map[string][]byte{}}
}

func (f *fakeSandbox) Connect(_ context.Context, sandboxID string) (provider.Sandbox, error) {
	if f.connectErr != nil {
		return provider.Sandbox{}, f.connectErr
	}
	token := "reconnect-token"
	return provider.Sandbox{SandboxID: sandboxID, EnvdAccessToken: &token}, nil
}

func (f *fakeSandbox) Create(_ context.Context, projectID string) (provider.Sandbox, error) {
	if f.createErr != nil {
		return provider.Sandbox{}, f.createErr
	}
	token := "fake-token"
	return provider.Sandbox{SandboxID: "sandbox-" + projectID, EnvdAccessToken: &token}, nil
}

func (f *fakeSandbox) Kill(_ context.Context, sandboxID string) error {
	f.killCalls = append(f.killCalls, sandboxID)
	return nil
}

func (f *fakeSandbox) PreviewURL(sandboxID string) string {
	return "https://preview.example/" + sandboxID
}

func (f *fakeSandbox) WriteFile(_ context.Context, _, _, path string, content []byte) error {
	f.files[path] = content
	return nil
}

func (f *fakeSandbox) ReadFile(_ context.Context, _, _, path string) ([]byte, error) {
	content, ok := f.files[path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return content, nil
}

func (f *fakeSandbox) RemoveFile(_ context.Context, _, _, path string) error {
	delete(f.files, path)
	return nil
}

func (f *fakeSandbox) MoveFile(_ context.Context, _, _, oldPath, newPath string) error {
	content, ok := f.files[oldPath]
	if !ok {
		return errors.New("file not found")
	}
	delete(f.files, oldPath)
	f.files[newPath] = content
	return nil
}

func (f *fakeSandbox) ListDir(_ context.Context, _, _, _ string, _ int) ([]provider.DirEntry, error) {
	entries := make([]provider.DirEntry, 0, len(f.files))
	for path := range f.files {
		entries = append(entries, provider.DirEntry{Name: path, Type: "file", Path: path})
	}
	return entries, nil
}

func (f *fakeSandbox) Run(_ context.Context, _, _, _, _ string) (provider.CommandResult, error) {
	return f.runResult, f.runErr
}

// fakeStore is an in-memory stand-in for *store.Store.
type fakeStore struct {
	project       store.Project
	projectErr    error
	files         []store.ProjectFile
	filesErr      error
	savedSandbox  bool
	upsertedFiles map[string]string
	deletedFiles  []string
	renamedFiles  map[string]string
	conversations []store.Conversation
}

func newFakeStore(project store.Project) *fakeStore {
	return &fakeStore{
		project:       project,
		upsertedFiles: map[string]string{},
		renamedFiles:  map[string]string{},
	}
}

func (f *fakeStore) Project(_ context.Context, _, _ uuid.UUID) (store.Project, error) {
	if f.projectErr != nil {
		return store.Project{}, f.projectErr
	}
	return f.project, nil
}

func (f *fakeStore) AddConversation(_ context.Context, projectID uuid.UUID, from, messageType, contents string, toolCall *string) (store.Conversation, error) {
	conversation := store.Conversation{ID: uuid.New(), ProjectID: projectID, From: from, Type: messageType, Contents: contents, ToolCall: toolCall}
	f.conversations = append(f.conversations, conversation)
	return conversation, nil
}

func (f *fakeStore) Files(_ context.Context, _ uuid.UUID) ([]store.ProjectFile, error) {
	if f.filesErr != nil {
		return nil, f.filesErr
	}
	return f.files, nil
}

func (f *fakeStore) SaveSandbox(_ context.Context, _ uuid.UUID, _, _ string) error {
	f.savedSandbox = true
	return nil
}

func (f *fakeStore) UpsertFile(_ context.Context, _ uuid.UUID, path, content string) error {
	f.upsertedFiles[path] = content
	return nil
}

func (f *fakeStore) DeleteFile(_ context.Context, _ uuid.UUID, path string) error {
	f.deletedFiles = append(f.deletedFiles, path)
	return nil
}

func (f *fakeStore) RenameFile(_ context.Context, _ uuid.UUID, oldPath, newPath string) error {
	f.renamedFiles[oldPath] = newPath
	return nil
}

// fakeModel is an in-memory stand-in for *provider.OpenRouter. responses is
// consumed in order, one per Complete call; the last response repeats if
// Complete is called more times than there are queued responses.
type fakeModel struct {
	responses []provider.Message
	errs      []error
	calls     int
}

func (f *fakeModel) Complete(_ context.Context, _ []provider.Message, _ []provider.ToolDefinition) (provider.Message, error) {
	call := f.calls
	f.calls++
	if call < len(f.errs) && f.errs[call] != nil {
		return provider.Message{}, f.errs[call]
	}
	index := call
	if index >= len(f.responses) {
		index = len(f.responses) - 1
	}
	return f.responses[index], nil
}

// collectEvents returns an EventSender that appends every event to the
// returned slice pointer, for asserting on the sequence of "e" values a run
// emitted.
func collectEvents() (*[]map[string]any, EventSender) {
	events := &[]map[string]any{}
	return events, func(event map[string]any) error {
		*events = append(*events, event)
		return nil
	}
}

func eventTypes(events []map[string]any) []string {
	types := make([]string, len(events))
	for i, event := range events {
		types[i], _ = event["e"].(string)
	}
	return types
}

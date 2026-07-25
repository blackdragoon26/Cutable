package provider

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestE2BLiveFilesystemAndCommand(t *testing.T) {
	apiKey := os.Getenv("E2B_API_KEY")
	sandboxID := os.Getenv("E2B_LIVE_SANDBOX_ID")
	if apiKey == "" || sandboxID == "" {
		t.Skip("set E2B_API_KEY and E2B_LIVE_SANDBOX_ID to run the live provider test")
	}
	client := NewE2B(apiKey, os.Getenv("E2B_TEMPLATE_ALIAS"), 15*time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sandbox, err := client.Connect(ctx, sandboxID)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if sandbox.EnvdAccessToken == nil {
		t.Fatal("Connect() returned no envd access token")
	}
	token := *sandbox.EnvdAccessToken
	const (
		filePath = "/home/user/react-app/cutable-provider-smoke.txt"
		content  = "cutable-e2b-ok"
	)
	if err := client.WriteFile(ctx, sandboxID, token, filePath, []byte(content)); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Cleanup(func() {
		_ = client.RemoveFile(context.Background(), sandboxID, token, filePath)
	})
	read, err := client.ReadFile(ctx, sandboxID, token, filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(read) != content {
		t.Fatalf("ReadFile() = %q, want %q", read, content)
	}
	result, err := client.Run(ctx, sandboxID, token, "printf cutable-command-ok", "/home/user/react-app")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 || !strings.Contains(result.Stdout, "cutable-command-ok") {
		t.Fatalf("Run() = %+v", result)
	}
}

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jeanedlune/Societas-AI/internal/config"
	storage "github.com/Jeanedlune/Societas-AI/internal/storage/sqlite"
)

func TestInitWritesValidSecretFreeWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "workspace.yaml")
	var stdout, stderr bytes.Buffer
	if err := Execute(context.Background(), []string{"init", "--config", path, "--name", "Sample"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("generated config is invalid: %v", err)
	}
	if cfg.Workspace.Name != "Sample" || len(cfg.Agents) == 0 {
		t.Fatalf("unexpected generated workspace: %#v", cfg.Workspace)
	}
	if strings.Contains(string(data), "API_KEY") || strings.Contains(string(data), "secret") {
		t.Fatalf("generated config contains a secret: %s", data)
	}
}

func TestServeReportsInvalidConfigPath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "workspace.yaml")
	if err := os.WriteFile(configPath, []byte("version: \"invalid\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Execute(context.Background(), []string{"serve", "--config", configPath}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("serve error=%v, want actionable config path", err)
	}
}

func TestSyncWorkspacePersistsSafeAgentMetadata(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	yamlData, err := config.DefaultWorkspaceYAML("sample")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := configuredAgents(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncWorkspace(ctx, db, cfg, ids); err != nil {
		t.Fatal(err)
	}
	var workspaceCount, agentCount int
	if err := db.QueryRow(`SELECT count(*) FROM workspaces WHERE workspace_id='ws_local'`).Scan(&workspaceCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM agents WHERE workspace_id='ws_local'`).Scan(&agentCount); err != nil {
		t.Fatal(err)
	}
	if workspaceCount != 1 || agentCount != len(ids) {
		t.Fatalf("workspace rows=%d agents=%d want agents=%d", workspaceCount, agentCount, len(ids))
	}
	var stored string
	if err := db.QueryRow(`SELECT config_json FROM agents WHERE workspace_id='ws_local' AND agent_id='coordinator'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "system_prompt") || strings.Contains(stored, "credentials") {
		t.Fatalf("sensitive agent config was persisted: %s", stored)
	}
}

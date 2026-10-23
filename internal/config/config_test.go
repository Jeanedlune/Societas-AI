package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadExample(t *testing.T) {
	cfg, err := Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workspace.Name != "general-content-workspace" {
		t.Fatalf("workspace name = %q", cfg.Workspace.Name)
	}
	if len(cfg.Agents) != 5 {
		t.Fatalf("agent count = %d, want 5", len(cfg.Agents))
	}
}

func TestDocumentedWorkspaceExampleMatchesSchema(t *testing.T) {
	doc, err := os.ReadFile("../../docs/07-dashboard-config-storage.md")
	if err != nil {
		t.Fatal(err)
	}
	marker := strings.Index(string(doc), "<!-- example:workspace.yaml -->")
	if marker < 0 {
		t.Fatal("workspace example marker is missing")
	}
	fence := strings.Index(string(doc)[marker:], "```yaml\n")
	if fence < 0 {
		t.Fatal("workspace YAML fence is missing")
	}
	start := marker + fence + len("```yaml\n")
	relativeEnd := strings.Index(string(doc)[start:], "\n```")
	if relativeEnd < 0 {
		t.Fatal("workspace YAML fence is not closed")
	}
	if _, err := Parse(doc[start : start+relativeEnd]); err != nil {
		t.Fatalf("documented workspace example is invalid: %v", err)
	}
}

func TestParseAppliesFiniteDefaultsAndDenyByDefault(t *testing.T) {
	cfg, err := Parse([]byte(`
version: "1"
models:
  default: "mock/default"
agents:
  content_reviewer:
    role: "Custom Reviewer"
    system_prompt: "Review content against the supplied brief."
    model: "mock/default"
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Budget.MaxModelCalls <= 0 || cfg.Budget.MaxTimeSeconds <= 0 || cfg.Budget.MaxCostUSD <= 0 {
		t.Fatal("budget defaults must be finite and positive")
	}
	if got := cfg.Agents["content_reviewer"].Permission("filesystem").Mode; got != "deny" {
		t.Fatalf("unspecified permission = %q, want deny", got)
	}
	if cfg.Agents["content_reviewer"].Role != "Custom Reviewer" {
		t.Fatal("custom role label was not preserved")
	}
}

func TestValidationErrorIncludesFieldPath(t *testing.T) {
	_, err := Parse([]byte(`
version: "1"
server:
  port: 70000
models:
  default: "mock/default"
`))
	if err == nil || !strings.Contains(err.Error(), "server.port") {
		t.Fatalf("error = %v, want a path containing server.port", err)
	}
}

func TestUnknownYAMLFieldIncludesPath(t *testing.T) {
	_, err := Parse([]byte(`
version: "1"
models:
  default: "mock/default"
agents:
  editor:
    role: "Editor"
    system_prompt: "Review content against the supplied brief."
    model: "mock/default"
    permisions:
      filesystem: read
`))
	if err == nil || !strings.Contains(err.Error(), "agents.editor.permisions") {
		t.Fatalf("error = %v, want the unknown YAML field path", err)
	}
}

func TestDuplicateAgentIDIsRejected(t *testing.T) {
	_, err := Parse([]byte(`
version: "1"
models:
  default: "mock/default"
agents:
  editor:
    role: "Editor"
    system_prompt: "Review the draft."
    model: "mock/default"
  editor:
    role: "Second editor"
    system_prompt: "Review another draft."
    model: "mock/default"
`))
	if err == nil || !strings.Contains(err.Error(), "editor") {
		t.Fatalf("error = %v, want duplicate agent ID rejection", err)
	}
}

func TestCredentialRequiresAndResolvesEnvironmentReference(t *testing.T) {
	cfg, err := Parse([]byte(`
version: "1"
providers:
  demo:
    credentials:
      api_key: "$DEMO_API_KEY"
models:
  default: "demo/model"
`))
	if err != nil {
		t.Fatal(err)
	}
	value, err := cfg.Credential("demo", "api_key", func(string) (string, bool) {
		return "", false
	})
	if err == nil || value != "" {
		t.Fatalf("missing environment variable should fail safely, value=%q error=%v", value, err)
	}

	value, err = cfg.Credential("demo", "api_key", func(name string) (string, bool) {
		if name == "DEMO_API_KEY" {
			return "test-secret", true
		}
		return "", false
	})
	if err != nil || value != "test-secret" {
		t.Fatalf("resolved value=%q error=%v", value, err)
	}
}

func TestLiteralCredentialIsRejected(t *testing.T) {
	_, err := Parse([]byte(`
version: "1"
providers:
  demo:
    credentials:
      api_key: "not-a-reference"
models:
  default: "demo/model"
`))
	if err == nil || !strings.Contains(err.Error(), "providers.demo.credentials.api_key") {
		t.Fatalf("error = %v, want provider credential path", err)
	}
}

func TestMultipleYAMLDocumentsAreRejected(t *testing.T) {
	_, err := Parse([]byte(`
version: "1"
models:
  default: "mock/default"
---
version: "1"
`))
	if err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("error = %v, want multiple document error", err)
	}
}

package agent

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Jeanedlune/Societas-AI/internal/config"
)

func TestRegistryBuildsSortedSnapshotAndSafeRoutingList(t *testing.T) {
	cfg, err := config.Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(cfg)
	if err != nil {
		t.Fatal(err)
	}

	wantIDs := []string{"content_creator", "coordinator", "editor", "market_researcher", "sales_analyst"}
	if got := registry.IDs(); !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("IDs() = %#v, want %#v", got, wantIDs)
	}
	if !registry.Has("market_researcher") || registry.Has("Market_Researcher") {
		t.Fatal("agent lookup must be exact and case-sensitive")
	}
	if got := registry.EscalationLead(); got != "human" {
		t.Fatalf("escalation lead = %q, want human", got)
	}

	routing := registry.RoutingAgents()
	if len(routing) != len(wantIDs) {
		t.Fatalf("routing options count = %d", len(routing))
	}
	for i, id := range wantIDs {
		if routing[i].ID != id {
			t.Fatalf("routing option[%d].ID = %q, want %q", i, routing[i].ID, id)
		}
		if routing[i].Role == "" {
			t.Fatalf("routing option[%d] has no role", i)
		}
	}
	if _, exists := reflect.TypeOf(routing[0]).FieldByName("SystemPrompt"); exists {
		t.Fatal("routing options must not contain system prompts")
	}
}

func TestRegistryReturnsDetachedAgentCopies(t *testing.T) {
	cfg, err := config.Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(cfg)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Agents["market_researcher"] = config.AgentConfig{Role: "Changed after snapshot"}
	agent, ok := registry.Get("market_researcher")
	if !ok || agent.Role != "Market Researcher" {
		t.Fatalf("snapshot changed after source config mutation: %#v, %t", agent, ok)
	}
	agent.Tools[0] = "tampered"
	agent.Permissions["web"] = config.PermissionConfig{Mode: "allow_all"}
	agent, _ = registry.Get("market_researcher")
	if agent.Tools[0] != "browser.search" || agent.Permissions["web"].Mode != "approved_sources" {
		t.Fatal("Get returned a mutable reference into the registry")
	}
}

func TestRegistryRejectsInvalidAgentReferences(t *testing.T) {
	cfg, err := config.Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agents["market_researcher"] = config.AgentConfig{
		Role:         "Researcher",
		SystemPrompt: "Find sources.",
		Model:        "mock/research",
		Tools:        []string{"browser.shell"},
	}
	_, err = NewRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "agents.market_researcher.tools[0]") {
		t.Fatalf("error = %v, want unknown tool capability path", err)
	}
}

func TestRegistryRejectsEmptySquadAndNilConfig(t *testing.T) {
	if _, err := NewRegistry(nil); err == nil {
		t.Fatal("nil config must be rejected")
	}
	cfg, err := config.Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agents = map[string]config.AgentConfig{}
	if _, err := NewRegistry(cfg); err == nil || !strings.Contains(err.Error(), "at least one active agent") {
		t.Fatalf("empty registry error = %v", err)
	}
}

func TestRegistryRejectsEmptyPrompt(t *testing.T) {
	cfg, err := config.Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	definition := cfg.Agents["editor"]
	definition.SystemPrompt = ""
	cfg.Agents["editor"] = definition
	_, err = NewRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "system_prompt") {
		t.Fatalf("invalid config error = %v", err)
	}
}

func TestRegistryRejectsUnknownEscalationLead(t *testing.T) {
	cfg, err := config.Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.EscalationLead = "missing_agent"
	_, err = NewRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "escalation_lead") {
		t.Fatalf("invalid config error = %v", err)
	}
}

func TestRegistryRejectsMalformedPermission(t *testing.T) {
	cfg, err := config.Load("../../config/workspace.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	definition := cfg.Agents["market_researcher"]
	definition.Permissions["shell"] = config.PermissionConfig{Mode: "allowlist"}
	cfg.Agents["market_researcher"] = definition
	_, err = NewRegistry(cfg)
	if err == nil || !strings.Contains(err.Error(), "permissions.shell.allowed") {
		t.Fatalf("invalid permission error = %v", err)
	}
}

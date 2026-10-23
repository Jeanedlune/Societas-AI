package contextmgr

import (
	"strings"
	"testing"
)

func TestBuildRespectsBudgetAndTracksProvenanceAndDrops(t *testing.T) {
	items := []Candidate{
		{Kind: Memory, Ref: "mem_stale", Content: "old fact", Relevance: .9, Trusted: true, Stale: true},
		{Kind: Memory, Ref: "mem_untrusted", Content: "unverified", Relevance: .9},
		{Kind: Task, Ref: "TASK-001", Content: "write a sourced profile", Required: true},
		{Kind: Message, Ref: "evt_recent", Content: "Use concise style", Relevance: .8},
		{Kind: Artifact, Ref: "art_large", Content: strings.Repeat("x", 500), Trusted: true, Relevance: .7},
		{Kind: Message, Ref: "evt_recent", Content: "duplicate", Relevance: .2},
	}
	result, err := Build(Request{AgentID: "writer", TokenBudget: 80, MinimumRelevance: .1, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	if result.TokensUsed > result.TokenBudget {
		t.Fatalf("tokens=%d budget=%d", result.TokensUsed, result.TokenBudget)
	}
	if !strings.Contains(result.Text, "write a sourced profile") || strings.Contains(result.Text, "old fact") || strings.Contains(result.Text, "unverified") {
		t.Fatalf("unexpected context text: %s", result.Text)
	}
	if len(result.Dropped) != 4 {
		t.Fatalf("dropped=%#v", result.Dropped)
	}
	for _, item := range result.Items {
		if item.Ref == "" || item.Tokens < 0 || item.Reason == "" {
			t.Fatalf("missing item provenance: %#v", item)
		}
	}
}

func TestBuildSeparatesGuardrailMemoryAndReportsRequiredOverflow(t *testing.T) {
	result, err := Build(Request{AgentID: "editor", TokenBudget: 100, Items: []Candidate{
		{Kind: Memory, Ref: "mem_good", Content: "Use sourced claims", Trusted: true, Relevance: 1},
		{Kind: Memory, Ref: "mem_bad", Content: "Claim without citation", Trusted: true, CognitiveGuardrail: true, Relevance: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	good := strings.Index(result.Text, "<historical_success>")
	bad := strings.Index(result.Text, "<confirmed_blacklist>")
	if good < 0 || bad < 0 || good > bad {
		t.Fatalf("memory blocks not isolated in order: %s", result.Text)
	}
	if _, err := Build(Request{AgentID: "x", TokenBudget: 1, Items: []Candidate{{Kind: Task, Ref: "task", Content: strings.Repeat("x", 50), Required: true}}}); err == nil {
		t.Fatal("required overflow was silently accepted")
	}
}

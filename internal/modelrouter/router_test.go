package modelrouter

import (
	"errors"
	"testing"
	"time"

	"github.com/Jeanedlune/Societas-AI/internal/agent"
	"github.com/Jeanedlune/Societas-AI/internal/config"
	"github.com/Jeanedlune/Societas-AI/internal/provider"
)

func TestCriticalRiskCannotRouteBelowStrongAndOnlyConfiguredFallbacksAreUsed(t *testing.T) {
	r := newRouter(t)
	route, err := r.Select(Request{AgentID: "coordinator", Complexity: "low", Risk: "critical", EstimatedTokens: 50}, time.Unix(1, 0))
	if err != nil || route.Tier != Strong || route.ModelID != "strong-model" {
		t.Fatalf("critical route=%#v err=%v", route, err)
	}
	r.Models["strong-model"] = Model{ID: "strong-model", ProviderID: "missing-provider", Tier: Strong, Enabled: true}
	r.Models["strong-backup"] = Model{ID: "strong-backup", ProviderID: "mock", Tier: Strong, Enabled: true}
	r.Fallbacks["strong-model"] = []string{"strong-backup"}
	route, err = r.Select(Request{AgentID: "coordinator", Complexity: "low", Risk: "critical", EstimatedTokens: 50}, time.Unix(2, 0))
	if err != nil || route.ModelID != "strong-backup" || !route.Fallback {
		t.Fatalf("fallback route=%#v err=%v", route, err)
	}
	r.Fallbacks["strong-model"] = []string{"unconfigured-model"}
	if _, err := r.Select(Request{AgentID: "coordinator", Risk: "critical"}, time.Unix(3, 0)); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("unconfigured fallback error=%v", err)
	}
}

func TestDisabledModelAndRateLimitReturnStableErrors(t *testing.T) {
	r := newRouter(t)
	r.Models["strong-model"] = Model{ID: "strong-model", ProviderID: "mock", Tier: Strong, Enabled: false}
	if _, err := r.Select(Request{AgentID: "coordinator", Risk: "critical"}, time.Unix(1, 0)); !errors.Is(err, ErrModelDisabled) {
		t.Fatalf("disabled error=%v", err)
	}
	r = newRouter(t)
	r.Limiter = NewRateLimiter(map[string]Limits{"mock": {RequestsPerMinute: 1, TokensPerMinute: 100}})
	request := Request{AgentID: "coordinator", Complexity: "medium", EstimatedTokens: 50}
	now := time.Unix(100, 0)
	if _, err := r.Select(request, now); err != nil {
		t.Fatalf("first route: %v", err)
	}
	_, err := r.Select(request, now.Add(time.Second))
	var limited *RateLimitError
	if !errors.As(err, &limited) || limited.RetryAfter <= 0 || !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate limit error=%v", err)
	}
}

func newRouter(t *testing.T) *Router {
	t.Helper()
	data, err := config.DefaultWorkspaceYAML("test")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := agent.NewRegistry(cfg)
	if err != nil {
		t.Fatal(err)
	}
	providers := provider.NewRegistry()
	if err := providers.Register(&provider.Mock{Name: "mock"}); err != nil {
		t.Fatal(err)
	}
	models := map[string]Model{"cheap-model": {ID: "cheap-model", ProviderID: "mock", Tier: Cheap, Enabled: true}, "medium-model": {ID: "medium-model", ProviderID: "mock", Tier: Medium, Enabled: true}, "strong-model": {ID: "strong-model", ProviderID: "mock", Tier: Strong, Enabled: true}}
	return &Router{Agents: registry, Providers: providers, Models: models, AgentModels: map[string]string{"coordinator": "cheap-model"}, TierModels: map[Tier]string{Cheap: "cheap-model", Medium: "medium-model", Strong: "strong-model"}, Fallbacks: map[string][]string{"cheap-model": {"medium-model"}, "medium-model": {"strong-model"}}}
}

package budget

import (
	"errors"
	"sync"
	"testing"

	"github.com/Jeanedlune/Societas-AI/internal/config"
)

func TestEmptyBudgetUsesFiniteDefaults(t *testing.T) {
	manager := NewManager(Budget{})
	if err := manager.OpenRoot("run", Budget{}); err != nil {
		t.Fatal(err)
	}
	got, _, err := manager.Allocate("run", "task", Budget{})
	if err != nil {
		t.Fatal(err)
	}
	want := FromConfig(config.DefaultBudget())
	if got != want {
		t.Fatalf("defaults = %#v, want %#v", got, want)
	}
	if got.TotalTokens <= 0 || got.ModelCalls <= 0 || got.TimeSeconds <= 0 || got.CostUSD <= 0 {
		t.Fatalf("defaults must be finite positive limits: %#v", got)
	}
}

func TestDefaultsForEachScopeAreFiniteAndConfigurable(t *testing.T) {
	for _, scope := range []Scope{ScopeWorkspace, ScopeRun, ScopeTask, ScopeAgent} {
		got, err := DefaultsForScope(scope, config.BudgetConfig{})
		if err != nil {
			t.Fatalf("scope %s: %v", scope, err)
		}
		if got.TotalTokens <= 0 || got.CostUSD <= 0 || got.ModelCalls <= 0 || got.ToolCalls <= 0 || got.TimeSeconds <= 0 {
			t.Fatalf("scope %s has non-finite defaults: %#v", scope, got)
		}
	}
	got, err := DefaultsForScope(ScopeTask, config.BudgetConfig{MaxInputTokens: 7200, MaxOutputTokens: 1800, MaxTotalTokens: 9000, MaxCostUSD: .25})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalTokens != 9000 || got.CostUSD != .25 {
		t.Fatalf("configured scope defaults = %#v", got)
	}
	if _, err := DefaultsForScope("unknown", config.BudgetConfig{}); err == nil {
		t.Fatal("unknown scope accepted")
	}
}

func TestAllocationChecksTokensAndCostAndSettlesOnce(t *testing.T) {
	manager := NewManager(Budget{})
	root := Budget{InputTokens: 100, OutputTokens: 100, TotalTokens: 200, ModelCalls: 8, ToolCalls: 8, AgentMessages: 8, TimeSeconds: 60, CostUSD: 2, ParallelAgents: 2, Rebuttals: 2}
	if err := manager.OpenRoot("root", root); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Allocate("root", "too-many-tokens", Budget{InputTokens: 101}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("token allocation error = %v", err)
	}
	if _, _, err := manager.Allocate("root", "too-much-cost", Budget{CostUSD: 2.01}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("cost allocation error = %v", err)
	}
	child := Budget{InputTokens: 50, OutputTokens: 40, TotalTokens: 90, ModelCalls: 3, ToolCalls: 2, AgentMessages: 3, TimeSeconds: 20, CostUSD: 1, ParallelAgents: 1, Rebuttals: 1}
	if _, duplicate, err := manager.Allocate("root", "child", child); err != nil || duplicate {
		t.Fatalf("first allocation: duplicate=%v err=%v", duplicate, err)
	}
	if _, duplicate, err := manager.Allocate("root", "child", child); err != nil || !duplicate {
		t.Fatalf("retry allocation: duplicate=%v err=%v", duplicate, err)
	}
	if _, _, err := manager.Allocate("root", "child", Budget{CostUSD: .5}); !errors.Is(err, ErrAllocationConflict) {
		t.Fatalf("conflicting retry error = %v", err)
	}
	actual := Usage{InputTokens: 12, OutputTokens: 8, ModelCalls: 1, TimeSeconds: 2, CostUSD: .1}
	if err := manager.Settle("root", "child", actual); err != nil {
		t.Fatal(err)
	}
	if err := manager.Settle("root", "child", actual); !errors.Is(err, ErrAllocationConflict) {
		t.Fatalf("second settlement error = %v", err)
	}
	remaining, err := manager.Remaining("root")
	if err != nil {
		t.Fatal(err)
	}
	if remaining.InputTokens != 88 || remaining.OutputTokens != 92 || remaining.TotalTokens != 180 || remaining.CostUSD < 1.89 || remaining.CostUSD > 1.91 {
		t.Fatalf("remaining after settlement = %#v", remaining)
	}
}

func TestConcurrentAllocationsCannotOversubscribe(t *testing.T) {
	manager := NewManager(Budget{})
	if err := manager.OpenRoot("root", Budget{InputTokens: 100, OutputTokens: 100, TotalTokens: 200, ModelCalls: 10, ToolCalls: 10, AgentMessages: 10, TimeSeconds: 100, CostUSD: 1, ParallelAgents: 10, Rebuttals: 10}); err != nil {
		t.Fatal(err)
	}
	child := Budget{InputTokens: 60, OutputTokens: 60, TotalTokens: 120, ModelCalls: 1, ToolCalls: 1, AgentMessages: 1, TimeSeconds: 10, CostUSD: .6, ParallelAgents: 1, Rebuttals: 1}
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := manager.Allocate("root", string(rune('a'+i)), child)
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, ErrBudgetExceeded) {
				t.Errorf("allocation error = %v", err)
			}
		}(i)
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successful concurrent grants = %d, want 1", successes)
	}
	remaining, err := manager.Remaining("root")
	if err != nil {
		t.Fatal(err)
	}
	if remaining.InputTokens != 40 || remaining.CostUSD < .39 || remaining.CostUSD > .41 {
		t.Fatalf("remaining = %#v", remaining)
	}
}

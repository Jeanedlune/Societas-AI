package delegation

import (
	"context"
	"errors"
	"testing"
)

type registry struct{ ids []string }

func (r registry) IDs() []string { return r.ids }
func (r registry) Has(id string) bool {
	for _, candidate := range r.ids {
		if candidate == id {
			return true
		}
	}
	return false
}

type policy map[string]bool

func (p policy) CanDelegate(_, target string) bool { return p[target] }

type orchestrator struct{ decisions []Decision }

func (o *orchestrator) CreateDelegatedTask(_ context.Context, d Decision) error {
	o.decisions = append(o.decisions, d)
	return nil
}

func TestExplicitAndBlindDelegationRouteThroughOrchestrator(t *testing.T) {
	orch := &orchestrator{}
	service := testService(orch)
	explicit := "researcher"
	decision, err := service.Delegate(context.Background(), request(&explicit))
	if err != nil || decision.Assignee != "researcher" || decision.Blind {
		t.Fatalf("explicit decision=%#v err=%v", decision, err)
	}
	blind, err := service.Delegate(context.Background(), request(nil))
	if err != nil || blind.Assignee != "editor" || !blind.Blind {
		t.Fatalf("blind decision=%#v err=%v", blind, err)
	}
	if len(orch.decisions) != 2 {
		t.Fatalf("orchestrator created %d tasks", len(orch.decisions))
	}
}

func TestDelegationRejectsUnknownDeniedAndLimitExceededBeforeCreation(t *testing.T) {
	orch := &orchestrator{}
	service := testService(orch)
	unknown := "not-active"
	if _, err := service.Delegate(context.Background(), request(&unknown)); !errors.Is(err, ErrUnknownAssignee) {
		t.Fatalf("unknown assignee error=%v", err)
	}
	denied := "editor"
	service.Policy = policy{"researcher": true}
	if _, err := service.Delegate(context.Background(), request(&denied)); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("permission error=%v", err)
	}
	service.Policy = policy{"researcher": true, "editor": true}
	r := request(nil)
	r.Depth = 4
	if _, err := service.Delegate(context.Background(), r); !errors.Is(err, ErrDepthLimit) {
		t.Fatalf("depth error=%v", err)
	}
	r = request(nil)
	r.CurrentFanOut = 2
	if _, err := service.Delegate(context.Background(), r); !errors.Is(err, ErrFanOutLimit) {
		t.Fatalf("fanout error=%v", err)
	}
	r = request(nil)
	r.RequestedBudget.CostUSD = 2
	if _, err := service.Delegate(context.Background(), r); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("budget error=%v", err)
	}
	if len(orch.decisions) != 0 {
		t.Fatalf("rejected requests created %d tasks", len(orch.decisions))
	}
}

func testService(orch Orchestrator) *Service {
	return &Service{Registry: registry{ids: []string{"coordinator", "researcher", "editor", "disabled"}}, Policy: policy{"researcher": true, "editor": true}, Orchestrator: orch, Agents: map[string]Agent{"researcher": {ID: "researcher", Active: true, ConcurrentTasks: 1}, "editor": {ID: "editor", Active: true, ConcurrentTasks: 0}, "disabled": {ID: "disabled", Active: false}}, Limits: Limits{MaxDepth: 4, MaxFanOut: 2, MaxChildTasks: 5, MaxParallelAgents: 2}}
}
func request(assignee *string) Request {
	return Request{Requester: "coordinator", RequestedAssignee: assignee, Title: "Research", Goal: "Find sources", Rationale: "Need evidence", Priority: "normal", Depth: 1, CurrentFanOut: 0, ParentRemaining: Budget{Tokens: 1000, CostUSD: 1, ModelCalls: 10, ToolCalls: 10, ParallelAgents: 2}, RequestedBudget: Budget{Tokens: 500, CostUSD: .25, ModelCalls: 4, ToolCalls: 3, ParallelAgents: 1}}
}

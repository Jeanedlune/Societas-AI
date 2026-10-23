package scheduler

import (
	"errors"
	"testing"

	"github.com/Jeanedlune/Societas-AI/internal/taskstate"
)

func TestBuildPlanRespectsDependenciesPriorityFairnessAndParallelLimit(t *testing.T) {
	tasks := []Task{
		{ID: "root", Status: taskstate.Completed},
		{ID: "a", Dependencies: []string{"root"}, Status: taskstate.Ready, Priority: "normal", OwnerAgentID: "agent-a", AgentAvailable: true, BudgetReserved: true, CreatedSequence: 1, LastScheduledSequence: 5},
		{ID: "b", Dependencies: []string{"root"}, Status: taskstate.Ready, Priority: "high", OwnerAgentID: "agent-b", AgentAvailable: true, BudgetReserved: true, CreatedSequence: 2, LastScheduledSequence: 4},
		{ID: "waiting", Dependencies: []string{"a"}, Status: taskstate.Ready, Priority: "critical", OwnerAgentID: "agent-c", AgentAvailable: true, BudgetReserved: true, CreatedSequence: 3},
	}
	plan, err := BuildPlan(tasks, Limits{MaxDepth: 3, MaxFanOut: 3, MaxParallel: 1}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 1 || plan.Selected[0].ID != "b" {
		t.Fatalf("selected=%v want high-priority b", plan.Selected)
	}
	if reasonFor(plan, "waiting") != "dependency_pending" || reasonFor(plan, "a") != "parallelism_limit" || reasonFor(plan, "b") != "ready" {
		t.Fatalf("decisions=%#v", plan.Decisions)
	}
	plan, err = BuildPlan(tasks, Limits{MaxDepth: 3, MaxFanOut: 3, MaxParallel: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 0 || reasonFor(plan, "b") != "parallelism_limit" {
		t.Fatalf("saturated plan=%#v", plan)
	}
}

func TestBuildPlanRejectsCycleUnknownDependencyFanOutAndDepth(t *testing.T) {
	base := Limits{MaxDepth: 2, MaxFanOut: 1, MaxParallel: 2}
	if _, err := BuildPlan([]Task{{ID: "a", Dependencies: []string{"b"}}, {ID: "b", Dependencies: []string{"a"}}}, base, 0); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle error=%v", err)
	}
	if _, err := BuildPlan([]Task{{ID: "a", Dependencies: []string{"missing"}}}, base, 0); !errors.Is(err, ErrUnknownDependency) {
		t.Fatalf("unknown dependency error=%v", err)
	}
	if _, err := BuildPlan([]Task{{ID: "root"}, {ID: "a", Dependencies: []string{"root"}}, {ID: "b", Dependencies: []string{"root"}}}, base, 0); !errors.Is(err, ErrFanOutExceeded) {
		t.Fatalf("fan-out error=%v", err)
	}
	if _, err := BuildPlan([]Task{{ID: "a"}, {ID: "b", Dependencies: []string{"a"}}, {ID: "c", Dependencies: []string{"b"}}, {ID: "d", Dependencies: []string{"c"}}}, base, 0); !errors.Is(err, ErrDepthExceeded) {
		t.Fatalf("depth error=%v", err)
	}
}

func TestBuildPlanDoesNotRunFailedDependencyOrUnavailableWork(t *testing.T) {
	tasks := []Task{{ID: "failed", Status: taskstate.Failed}, {ID: "child", Dependencies: []string{"failed"}, Status: taskstate.Ready, OwnerAgentID: "a", AgentAvailable: true, BudgetReserved: true}, {ID: "unfunded", Status: taskstate.Ready, OwnerAgentID: "a", AgentAvailable: true}}
	plan, err := BuildPlan(tasks, Limits{MaxDepth: 2, MaxFanOut: 2, MaxParallel: 2}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 0 || reasonFor(plan, "child") != "dependency_failed" || reasonFor(plan, "unfunded") != "budget_not_reserved" {
		t.Fatalf("plan=%#v", plan)
	}
	if CanCancel(taskstate.Completed) || CanCancel(taskstate.Failed) || !CanCancel(taskstate.Ready) {
		t.Fatal("cancel predicate violated terminal behavior")
	}
	if CanReschedule(taskstate.Interrupted, 1) == false || CanReschedule(taskstate.Completed, 1) || CanReschedule(taskstate.Running, 0) {
		t.Fatal("reschedule predicate violated retry constraints")
	}
}

func reasonFor(plan Plan, id string) string {
	for _, decision := range plan.Decisions {
		if decision.TaskID == id {
			return decision.Reason
		}
	}
	return "missing"
}

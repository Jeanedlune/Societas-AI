// Package scheduler validates task DAGs and deterministically selects runnable work.
package scheduler

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Jeanedlune/Societas-AI/internal/taskstate"
)

var (
	ErrDependencyCycle   = errors.New("task dependency graph contains a cycle")
	ErrUnknownDependency = errors.New("task dependency does not exist in this run")
	ErrFanOutExceeded    = errors.New("task fan-out limit exceeded")
	ErrDepthExceeded     = errors.New("task depth limit exceeded")
)

type Task struct {
	ID                    string
	Dependencies          []string
	Status                taskstate.Status
	Priority              string
	OwnerAgentID          string
	AgentAvailable        bool
	BudgetReserved        bool
	Depth                 int
	CreatedSequence       uint64
	LastScheduledSequence uint64
}
type Limits struct {
	MaxDepth    int
	MaxFanOut   int
	MaxParallel int
}
type Decision struct {
	TaskID   string
	Runnable bool
	Reason   string
}
type Plan struct {
	Selected  []Task
	Decisions []Decision
}

// BuildPlan validates graph limits and selects the highest-priority eligible
// ready tasks. Older scheduling sequence wins ties to prevent starvation.
func BuildPlan(tasks []Task, limits Limits, inFlight int) (Plan, error) {
	if limits.MaxDepth < 1 || limits.MaxFanOut < 1 || limits.MaxParallel < 1 || inFlight < 0 {
		return Plan{}, fmt.Errorf("invalid scheduler limits")
	}
	byID := make(map[string]Task, len(tasks))
	for _, task := range tasks {
		if task.ID == "" {
			return Plan{}, fmt.Errorf("task ID is required")
		}
		if _, exists := byID[task.ID]; exists {
			return Plan{}, fmt.Errorf("duplicate task ID %s", task.ID)
		}
		byID[task.ID] = task
	}
	children := make(map[string]int, len(tasks))
	for _, task := range tasks {
		seen := map[string]bool{}
		for _, dependency := range task.Dependencies {
			if _, ok := byID[dependency]; !ok {
				return Plan{}, fmt.Errorf("%w: %s -> %s", ErrUnknownDependency, task.ID, dependency)
			}
			if dependency == task.ID {
				return Plan{}, fmt.Errorf("%w: %s", ErrDependencyCycle, task.ID)
			}
			if seen[dependency] {
				return Plan{}, fmt.Errorf("task %s repeats dependency %s", task.ID, dependency)
			}
			seen[dependency] = true
			children[dependency]++
			if children[dependency] > limits.MaxFanOut {
				return Plan{}, fmt.Errorf("%w: %s", ErrFanOutExceeded, dependency)
			}
		}
	}
	if err := checkCycles(byID); err != nil {
		return Plan{}, err
	}
	for id, task := range byID {
		depth, err := graphDepth(id, byID, map[string]int{})
		if err != nil {
			return Plan{}, err
		}
		if depth > limits.MaxDepth {
			return Plan{}, fmt.Errorf("%w: %s depth %d", ErrDepthExceeded, id, depth)
		}
		if task.Depth > limits.MaxDepth {
			return Plan{}, fmt.Errorf("%w: %s depth %d", ErrDepthExceeded, id, task.Depth)
		}
	}
	decisions := make([]Decision, 0, len(tasks))
	eligible := make([]Task, 0)
	for _, task := range tasks {
		reason := ""
		if task.Status != taskstate.Ready {
			reason = "not_ready"
		} else if !task.BudgetReserved {
			reason = "budget_not_reserved"
		} else if !task.AgentAvailable || task.OwnerAgentID == "" {
			reason = "agent_unavailable"
		} else {
			for _, dependency := range task.Dependencies {
				status := byID[dependency].Status
				if status == taskstate.Failed || status == taskstate.Cancelled {
					reason = "dependency_failed"
					break
				}
				if status != taskstate.Completed {
					reason = "dependency_pending"
					break
				}
			}
		}
		if reason == "" {
			eligible = append(eligible, task)
			decisions = append(decisions, Decision{TaskID: task.ID, Runnable: false, Reason: "parallelism_limit"})
		} else {
			decisions = append(decisions, Decision{TaskID: task.ID, Runnable: false, Reason: reason})
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if priorityRank(a.Priority) != priorityRank(b.Priority) {
			return priorityRank(a.Priority) > priorityRank(b.Priority)
		}
		if a.LastScheduledSequence != b.LastScheduledSequence {
			return a.LastScheduledSequence < b.LastScheduledSequence
		}
		if a.CreatedSequence != b.CreatedSequence {
			return a.CreatedSequence < b.CreatedSequence
		}
		return a.ID < b.ID
	})
	capacity := limits.MaxParallel - inFlight
	if capacity < 0 {
		capacity = 0
	}
	if capacity > len(eligible) {
		capacity = len(eligible)
	}
	selected := append([]Task(nil), eligible[:capacity]...)
	selectedIDs := make(map[string]bool, len(selected))
	for _, task := range selected {
		selectedIDs[task.ID] = true
	}
	for i := range decisions {
		if selectedIDs[decisions[i].TaskID] {
			decisions[i].Runnable = true
			decisions[i].Reason = "ready"
		}
	}
	return Plan{Selected: selected, Decisions: decisions}, nil
}

func checkCycles(tasks map[string]Task) error {
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("%w: %s", ErrDependencyCycle, id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, dep := range tasks[id].Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range tasks {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
func graphDepth(id string, tasks map[string]Task, memo map[string]int) (int, error) {
	if depth, ok := memo[id]; ok {
		return depth, nil
	}
	depth := 0
	for _, dep := range tasks[id].Dependencies {
		depDepth, err := graphDepth(dep, tasks, memo)
		if err != nil {
			return 0, err
		}
		if depDepth+1 > depth {
			depth = depDepth + 1
		}
	}
	memo[id] = depth
	return depth, nil
}
func priorityRank(priority string) int {
	switch priority {
	case "critical":
		return 4
	case "high":
		return 3
	case "normal":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func CanCancel(status taskstate.Status) bool {
	return status != taskstate.Completed && status != taskstate.Failed && status != taskstate.Cancelled
}
func CanReschedule(status taskstate.Status, retriesRemaining int) bool {
	return retriesRemaining > 0 && (status == taskstate.Running || status == taskstate.Interrupted)
}

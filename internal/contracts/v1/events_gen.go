// Code generated from docs/10-contracts-mvp-roadmap.md; DO NOT EDIT.
package v1

import "fmt"

const (
	EventWorkspaceCreated       EventType = "workspace.created"
	EventAgentCreated           EventType = "agent.created"
	EventAgentStarted           EventType = "agent.started"
	EventAgentStopped           EventType = "agent.stopped"
	EventAgentStatusChanged     EventType = "agent.status_changed"
	EventRunCreated             EventType = "run.created"
	EventRunStopped             EventType = "run.stopped"
	EventTaskDelegateRequested  EventType = "task.delegate_requested"
	EventTaskDelegateRejected   EventType = "task.delegate_rejected"
	EventTaskCreated            EventType = "task.created"
	EventTaskAssigned           EventType = "task.assigned"
	EventTaskStarted            EventType = "task.started"
	EventTaskBlocked            EventType = "task.blocked"
	EventTaskSemanticConflict   EventType = "task.semantic_conflict"
	EventTaskPaused             EventType = "task.paused"
	EventTaskResumed            EventType = "task.resumed"
	EventTaskInterrupted        EventType = "task.interrupted"
	EventTaskContractChanged    EventType = "task.contract_changed"
	EventTaskCompleted          EventType = "task.completed"
	EventTaskFailed             EventType = "task.failed"
	EventTaskCancelled          EventType = "task.cancelled"
	EventTaskRetryScheduled     EventType = "task.retry_scheduled"
	EventMessageSent            EventType = "message.sent"
	EventMessageReceived        EventType = "message.received"
	EventContextBuilt           EventType = "context.built"
	EventModelSelected          EventType = "model.selected"
	EventModelCallStarted       EventType = "model.call_started"
	EventModelCallCompleted     EventType = "model.call_completed"
	EventModelCallFailed        EventType = "model.call_failed"
	EventBudgetReserved         EventType = "budget.reserved"
	EventBudgetSettled          EventType = "budget.settled"
	EventBudgetWarning          EventType = "budget.warning"
	EventBudgetExceeded         EventType = "budget.exceeded"
	EventPolicyEvaluated        EventType = "policy.evaluated"
	EventToolCallRequested      EventType = "tool.call_requested"
	EventToolCallStarted        EventType = "tool.call_started"
	EventToolCallCompleted      EventType = "tool.call_completed"
	EventToolCallFailed         EventType = "tool.call_failed"
	EventArtifactCreated        EventType = "artifact.created"
	EventArtifactUpdated        EventType = "artifact.updated"
	EventSummaryCreated         EventType = "summary.created"
	EventReviewRequested        EventType = "review.requested"
	EventReviewCompleted        EventType = "review.completed"
	EventApprovalRequested      EventType = "approval.requested"
	EventApprovalGranted        EventType = "approval.granted"
	EventApprovalRejected       EventType = "approval.rejected"
	EventApprovalInvalidated    EventType = "approval.invalidated"
	EventApprovalBatchSubmitted EventType = "approval.batch_submitted"
)

var eventDefinitions = [...]EventDefinition{
	{Type: EventWorkspaceCreated, PayloadSchemaID: "urn:societas:1:workspace_created", From: "system:*", To: "topic:all"},
	{Type: EventAgentCreated, PayloadSchemaID: "urn:societas:1:agent_lifecycle", From: "system:orchestrator", To: "topic:all"},
	{Type: EventAgentStarted, PayloadSchemaID: "urn:societas:1:agent_lifecycle", From: "system:scheduler", To: "topic:all"},
	{Type: EventAgentStopped, PayloadSchemaID: "urn:societas:1:agent_lifecycle", From: "system:scheduler", To: "topic:all"},
	{Type: EventAgentStatusChanged, PayloadSchemaID: "urn:societas:1:agent_status_changed", From: "system:orchestrator", To: "topic:all"},
	{Type: EventRunCreated, PayloadSchemaID: "urn:societas:1:run_created", From: "human", To: "system:orchestrator"},
	{Type: EventRunStopped, PayloadSchemaID: "urn:societas:1:run_stopped", From: "system:orchestrator", To: "human"},
	{Type: EventTaskDelegateRequested, PayloadSchemaID: "urn:societas:1:delegate_requested", From: "agent", To: "system:orchestrator"},
	{Type: EventTaskDelegateRejected, PayloadSchemaID: "urn:societas:1:delegate_rejected", From: "system:orchestrator", To: "agent"},
	{Type: EventTaskCreated, PayloadSchemaID: "urn:societas:1:task_created", From: "system:orchestrator", To: "agent (owner)"},
	{Type: EventTaskAssigned, PayloadSchemaID: "urn:societas:1:task_assigned", From: "system:scheduler", To: "agent"},
	{Type: EventTaskStarted, PayloadSchemaID: "urn:societas:1:task_started", From: "agent", To: "system:orchestrator"},
	{Type: EventTaskBlocked, PayloadSchemaID: "urn:societas:1:task_blocked", From: "agent", To: "system:orchestrator"},
	{Type: EventTaskSemanticConflict, PayloadSchemaID: "urn:societas:1:task_semantic_conflict", From: "system:orchestrator", To: "agent (owner) + agent (escalation_lead) / human:user"},
	{Type: EventTaskPaused, PayloadSchemaID: "urn:societas:1:task_paused", From: "phase=requested: human:user", To: "system:orchestrator; phase=completed: system:orchestrator \u2192 topic:all"},
	{Type: EventTaskResumed, PayloadSchemaID: "urn:societas:1:task_resumed", From: "system:orchestrator", To: "agent"},
	{Type: EventTaskInterrupted, PayloadSchemaID: "urn:societas:1:task_interrupted", From: "system:orchestrator", To: "topic:all"},
	{Type: EventTaskContractChanged, PayloadSchemaID: "urn:societas:1:task_contract_changed", From: "system:orchestrator", To: "agent (owner)"},
	{Type: EventTaskCompleted, PayloadSchemaID: "urn:societas:1:task_completed", From: "agent", To: "system:orchestrator"},
	{Type: EventTaskFailed, PayloadSchemaID: "urn:societas:1:task_failed", From: "system:orchestrator", To: "topic:all"},
	{Type: EventTaskCancelled, PayloadSchemaID: "urn:societas:1:task_cancelled", From: "system:orchestrator", To: "topic:all"},
	{Type: EventTaskRetryScheduled, PayloadSchemaID: "urn:societas:1:task_retry_scheduled", From: "system:orchestrator", To: "agent"},
	{Type: EventMessageSent, PayloadSchemaID: "urn:societas:1:message_sent", From: "agent", To: "agent (dimediasi Event Bus)"},
	{Type: EventMessageReceived, PayloadSchemaID: "urn:societas:1:message_received", From: "agent", To: "system:event_bus"},
	{Type: EventContextBuilt, PayloadSchemaID: "urn:societas:1:context_built", From: "system:context", To: "agent"},
	{Type: EventModelSelected, PayloadSchemaID: "urn:societas:1:model_selected", From: "system:router", To: "agent"},
	{Type: EventModelCallStarted, PayloadSchemaID: "urn:societas:1:model_call_started", From: "system:router", To: "topic:all"},
	{Type: EventModelCallCompleted, PayloadSchemaID: "urn:societas:1:model_call_completed", From: "system:router", To: "agent"},
	{Type: EventModelCallFailed, PayloadSchemaID: "urn:societas:1:model_call_failed", From: "system:router", To: "agent"},
	{Type: EventBudgetReserved, PayloadSchemaID: "urn:societas:1:budget_reserved", From: "system:budget", To: "topic:all"},
	{Type: EventBudgetSettled, PayloadSchemaID: "urn:societas:1:budget_settled", From: "system:budget", To: "topic:all"},
	{Type: EventBudgetWarning, PayloadSchemaID: "urn:societas:1:budget_warning", From: "system:budget", To: "human"},
	{Type: EventBudgetExceeded, PayloadSchemaID: "urn:societas:1:budget_exceeded", From: "system:budget", To: "system:orchestrator"},
	{Type: EventPolicyEvaluated, PayloadSchemaID: "urn:societas:1:policy_evaluated", From: "system:policy", To: "topic:all"},
	{Type: EventToolCallRequested, PayloadSchemaID: "urn:societas:1:tool_call_requested", From: "agent", To: "system:tool_runtime"},
	{Type: EventToolCallStarted, PayloadSchemaID: "urn:societas:1:tool_call_started", From: "system:tool_runtime", To: "topic:all"},
	{Type: EventToolCallCompleted, PayloadSchemaID: "urn:societas:1:tool_call_completed", From: "system:tool_runtime", To: "agent"},
	{Type: EventToolCallFailed, PayloadSchemaID: "urn:societas:1:tool_call_failed", From: "system:tool_runtime", To: "agent"},
	{Type: EventArtifactCreated, PayloadSchemaID: "urn:societas:1:artifact_created", From: "agent", To: "topic:all"},
	{Type: EventArtifactUpdated, PayloadSchemaID: "urn:societas:1:artifact_updated", From: "agent", To: "topic:all"},
	{Type: EventSummaryCreated, PayloadSchemaID: "urn:societas:1:summary_created", From: "system:summarizer", To: "topic:all"},
	{Type: EventReviewRequested, PayloadSchemaID: "urn:societas:1:review_requested", From: "agent", To: "system:orchestrator"},
	{Type: EventReviewCompleted, PayloadSchemaID: "urn:societas:1:review_completed", From: "agent", To: "system:orchestrator"},
	{Type: EventApprovalRequested, PayloadSchemaID: "urn:societas:1:approval_requested", From: "system:policy", To: "human"},
	{Type: EventApprovalGranted, PayloadSchemaID: "urn:societas:1:approval_granted", From: "human", To: "system:policy"},
	{Type: EventApprovalRejected, PayloadSchemaID: "urn:societas:1:approval_rejected", From: "human", To: "system:policy"},
	{Type: EventApprovalInvalidated, PayloadSchemaID: "urn:societas:1:approval_invalidated", From: "system:orchestrator", To: "human:user"},
	{Type: EventApprovalBatchSubmitted, PayloadSchemaID: "urn:societas:1:approval_batch_submitted", From: "human:user", To: "system:orchestrator"},
}

// EventDefinitions returns the active wire event registry in source order.
func EventDefinitions() []EventDefinition {
	return append([]EventDefinition(nil), eventDefinitions[:]...)
}

// Definition returns the unique definition for an event type.
func Definition(eventType EventType) (EventDefinition, bool) {
	for _, definition := range eventDefinitions {
		if definition.Type == eventType {
			return definition, true
		}
	}
	return EventDefinition{}, false
}

// NewPayload allocates the schema-specific generated payload type.
func NewPayload(eventType EventType) (any, error) {
	switch eventType {
	case EventWorkspaceCreated:
		return &WorkspaceCreated{}, nil
	case EventAgentCreated:
		return &AgentLifecycle{}, nil
	case EventAgentStarted:
		return &AgentLifecycle{}, nil
	case EventAgentStopped:
		return &AgentLifecycle{}, nil
	case EventAgentStatusChanged:
		return &AgentStatusChanged{}, nil
	case EventRunCreated:
		return &RunCreated{}, nil
	case EventRunStopped:
		return &RunStopped{}, nil
	case EventTaskDelegateRequested:
		return &DelegateRequested{}, nil
	case EventTaskDelegateRejected:
		return &DelegateRejected{}, nil
	case EventTaskCreated:
		return &TaskCreated{}, nil
	case EventTaskAssigned:
		return &TaskAssigned{}, nil
	case EventTaskStarted:
		return &TaskStarted{}, nil
	case EventTaskBlocked:
		return &TaskBlocked{}, nil
	case EventTaskSemanticConflict:
		return &TaskSemanticConflict{}, nil
	case EventTaskPaused:
		return &TaskPaused{}, nil
	case EventTaskResumed:
		return &TaskResumed{}, nil
	case EventTaskInterrupted:
		return &TaskInterrupted{}, nil
	case EventTaskContractChanged:
		return &TaskContractChanged{}, nil
	case EventTaskCompleted:
		return &TaskCompleted{}, nil
	case EventTaskFailed:
		return &TaskFailed{}, nil
	case EventTaskCancelled:
		return &TaskCancelled{}, nil
	case EventTaskRetryScheduled:
		return &TaskRetryScheduled{}, nil
	case EventMessageSent:
		return &MessageSent{}, nil
	case EventMessageReceived:
		return &MessageReceived{}, nil
	case EventContextBuilt:
		return &ContextBuilt{}, nil
	case EventModelSelected:
		return &ModelSelected{}, nil
	case EventModelCallStarted:
		return &ModelCallStarted{}, nil
	case EventModelCallCompleted:
		return &ModelCallCompleted{}, nil
	case EventModelCallFailed:
		return &ModelCallFailed{}, nil
	case EventBudgetReserved:
		return &BudgetReserved{}, nil
	case EventBudgetSettled:
		return &BudgetSettled{}, nil
	case EventBudgetWarning:
		return &BudgetWarning{}, nil
	case EventBudgetExceeded:
		return &BudgetExceeded{}, nil
	case EventPolicyEvaluated:
		return &PolicyEvaluated{}, nil
	case EventToolCallRequested:
		return &ToolCallRequested{}, nil
	case EventToolCallStarted:
		return &ToolCallStarted{}, nil
	case EventToolCallCompleted:
		return &ToolCallCompleted{}, nil
	case EventToolCallFailed:
		return &ToolCallFailed{}, nil
	case EventArtifactCreated:
		return &ArtifactCreated{}, nil
	case EventArtifactUpdated:
		return &ArtifactUpdated{}, nil
	case EventSummaryCreated:
		return &SummaryCreated{}, nil
	case EventReviewRequested:
		return &ReviewRequested{}, nil
	case EventReviewCompleted:
		return &ReviewCompleted{}, nil
	case EventApprovalRequested:
		return &ApprovalRequested{}, nil
	case EventApprovalGranted:
		return &ApprovalGranted{}, nil
	case EventApprovalRejected:
		return &ApprovalRejected{}, nil
	case EventApprovalInvalidated:
		return &ApprovalInvalidated{}, nil
	case EventApprovalBatchSubmitted:
		return &ApprovalBatchSubmitted{}, nil
	default:
		return nil, fmt.Errorf("unknown event type %q", eventType)
	}
}

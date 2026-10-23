# Task-037 - Implementasikan Domain Review Evaluator

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P1
- **Depends on:** 023, 029
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Review artifact menurut goal, sources, metrics, assumptions, audience, and policy.

## Scope
Implementasikan criteria input, verdict schema, findings severity, evidence references, and blocked escalation path.

## Acceptance criteria
- Verdict hanya `approve`, `request_changes`, atau `reject`.
- Findings merujuk artifact version yang direview.
- Missing evidence dapat dilaporkan sebagai finding, bukan diisi asumsi.
- Reviewer tidak dapat mengubah policy atau memberi approval.
- Tests mencakup approve, revisions, reject, and insufficient evidence.

## Handoff
Review criteria per workflow disediakan lewat config/task input.

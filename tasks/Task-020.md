# Task-020 - Terapkan Budget Defaults dan Inheritance

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 003, 005
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Terapkan finite defaults dan validasi child budget terhadap remaining parent budget.

## Scope
Implementasikan defaults per scope, reserve estimates, child allocation, and max-call/time limits.

## Acceptance criteria
- Budget kosong memakai defaults finite dari config.
- Child granted limits tidak melebihi sisa parent.
- Concurrent child allocations tidak oversubscribe parent.
- Exceeded request ditolak dengan `BUDGET_EXCEEDED`.
- Tests mencakup tokens, cost, concurrent grants, dan default behavior.

## Handoff
Jelaskan unit dan currency untuk seluruh limit.

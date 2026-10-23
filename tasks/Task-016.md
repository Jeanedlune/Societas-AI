# Task-016 - Implementasikan DAG Scheduler

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 015
- **Source:** [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 08](../docs/08-runtime-architecture.md)

## Goal
Jalankan task sesuai dependencies, priority, agent availability, budget, dan concurrency limits.

## Scope
Implementasikan cycle detection, ready queue, fairness, limits, cancellation, dan reschedule.

## Acceptance criteria
- Dependency cycle ditolak saat task dibuat.
- Task hanya runnable setelah seluruh dependency terpenuhi.
- Fan-out, depth, dan parallelism limit ditegakkan.
- Scheduler tidak menjalankan task terminal.
- Deterministic tests mencakup DAG paralel, dependency gagal, dan saturation.

## Handoff
Scheduler decisions dapat dijelaskan melalui event/diagnostic reason.

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

## Hasil
- `internal/scheduler.BuildPlan` menolak dependency hilang, duplikat, cycle, fan-out berlebih, dan depth berlebih sebelum memilih kerja.
- Ready queue hanya memilih task `ready` dengan dependency completed, budget reserved, dan owner agent aktif; priority didahulukan, lalu waktu penjadwalan lama dan ID sebagai tie-breaker.
- Parallelism limit, reason keputusan, dependency failure, cancellation, dan reschedule eligibility tersedia secara deterministic dengan tests DAG paralel/saturation.

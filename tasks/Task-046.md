# Task-046 - Bangun Task Board

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P1
- **Depends on:** 015, 016, 045
- **Source:** [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Tampilkan task berdasarkan state, owner, dependency, priority, risk, dan progress.

## Scope
Implementasikan backlog/running/blocked/approval/completed views, task detail, pause/resume/cancel commands.

## Acceptance criteria
- State/transitions sesuai backend contract.
- Dependency dan blocker dapat ditelusuri.
- Pause/resume/cancel meminta confirmation bila diperlukan dan menampilkan result.
- UI tidak mengizinkan transisi ilegal.
- UI tests mencakup state transitions dan concurrent refresh.

## Handoff
Task board tetap usable saat event stream putus sementara.

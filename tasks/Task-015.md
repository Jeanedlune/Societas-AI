# Task-015 - Implementasikan Task State Machine

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 005, 007, 010
- **Source:** [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Terapkan status dan transition task kanonik dari kontrak aktif.

## Scope
Implementasikan status lifecycle, CAS transition, terminal behavior, ownership, timestamps, dan state events.

## Acceptance criteria
- Transition yang tidak diizinkan ditolak.
- Task terminal tidak dibuka kembali.
- Dua concurrent state update tidak menimpa satu sama lain.
- Task status dapat direkonstruksi dari persisted state/events.
- State-machine tests mencakup pause, approval, blocked, retry, dan terminal state.

## Handoff
Jangan menambah status yang tidak ada di kontrak tanpa keputusan dan perubahan schema.

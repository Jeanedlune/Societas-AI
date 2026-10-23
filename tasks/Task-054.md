# Task-054 - Tambahkan Task dan Delegation Tests

- **Phase:** 6 - Validasi dan rilis
- **Priority:** P0
- **Depends on:** 015, 016, 017
- **Source:** [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Uji state machine, dependency DAG, routing, and fan-out behavior secara deterministik.

## Scope
Tambahkan unit/integration fixtures untuk success, blocked, failure, retry, cancellation, and concurrent scheduling.

## Acceptance criteria
- Semua transition valid diterima dan transition invalid ditolak.
- Terminal task tidak dibuka ulang.
- Dependency cycle dan unknown owner ditolak.
- Limits enforced di concurrent delegation.
- Replay menghasilkan state yang sama dengan event sequence.

## Handoff
Tests tidak memanggil provider nyata.

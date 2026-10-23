# Task-008 - Implementasikan Event Store

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P0
- **Depends on:** 007
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 08](../docs/08-runtime-architecture.md)

## Goal
Simpan event append-only secara durable dengan urutan monoton per run.

## Scope
Implementasikan append, read by run/sequence, event lookup by ID, dan transactional persistence API.

## Acceptance criteria
- Append memberi sequence monoton pada concurrent writers.
- Event tersimpan sebelum dapat dibroadcast.
- Duplicate event/idempotency key tidak membuat record kedua.
- Read/replay menghasilkan urutan yang sama setelah restart.
- Tests mencakup transaction rollback dan concurrent append.

## Handoff
Jangan tambahkan business logic agent ke Event Store.

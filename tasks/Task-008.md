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

## Hasil
- `internal/eventstore` memvalidasi envelope sebelum persistence, menetapkan sequence monoton per workspace/run di dalam transaksi, dan baru mengembalikan hasil sesudah commit sukses.
- Duplicate event ID atau idempotency key per run mengembalikan record awal tanpa membuat row baru. Read tersedia lewat run + sequence, sequence tepat, dan event ID; replay diurutkan dengan sequence.
- Migration 0003 mengubah unique idempotency scope menjadi `(workspace_id, run_id, idempotency_key)` serta menolak update/delete event melalui trigger SQLite.
- Tests mencakup restart/replay, concurrent writers dengan sequence contiguous, deduplication, rollback tanpa sequence gap, dan penolakan perubahan row tersimpan.

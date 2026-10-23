# Task-011 - Tambahkan Idempotency dan Replay

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P1
- **Depends on:** 008, 010
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Buat command/event retry aman dan sediakan replay state per run.

## Scope
Implementasikan idempotency key storage, duplicate response behavior, replay reader, dan event-to-state projection.

## Acceptance criteria
- Duplicate request dengan key sama tidak menjalankan side effect dua kali.
- Key sama dengan payload berbeda ditolak.
- Replay dari event store membangun state run/task yang sama.
- Event historis tetap immutable.
- Tests membuktikan duplicate delivery, restart, dan replay determinism.

## Handoff
Jangan menganggap event idempotency sebagai jaminan deduplikasi tool eksternal.

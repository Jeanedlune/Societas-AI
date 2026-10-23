# Task-055 - Tambahkan Event Persistence Tests

- **Phase:** 6 - Validasi dan rilis
- **Priority:** P0
- **Depends on:** 008, 009, 011
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 08](../docs/08-runtime-architecture.md)

## Goal
Buktikan durability, ordering, outbox delivery, replay, dan idempotency event.

## Scope
Tambahkan test crash-point harness, concurrent append, subscriber restart, and duplicate delivery.

## Acceptance criteria
- Event committed tidak hilang setelah simulated process restart.
- Event uncommitted tidak terkirim.
- Sequence monoton per run.
- Duplicate delivery tidak menggandakan projection/side effect internal.
- Outbox retry akhirnya delivered atau masuk diagnosable failed state.

## Handoff
Gunakan temporary database dan deterministic clock bila diperlukan.

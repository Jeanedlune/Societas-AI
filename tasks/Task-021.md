# Task-021 - Implementasikan Budget Ledger

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 007, 020
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Catat reserve, settle, refund, dan usage agar crash tidak menggandakan atau menghilangkan budget.

## Scope
Implementasikan ledger append-only, reservation leases, settlement transaction, expiry recovery, and reconciliation.

## Acceptance criteria
- Reserve dan lease tersimpan atomik.
- Settle mencatat usage aktual dan mengembalikan selisih.
- Expired lease dipulihkan idempotently.
- Saldo per scope dapat direkonstruksi dari ledger.
- Integration tests mencakup crash antara reserve dan settle.

## Handoff
Semua operasi ledger harus idempotent pada reservation ID.

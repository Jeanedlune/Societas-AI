# Task-056 - Tambahkan Budget dan Retry Tests

- **Phase:** 6 - Validasi dan rilis
- **Priority:** P0
- **Depends on:** 020, 021, 030
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Buktikan finite budget, parent-child allocation, settle/refund, and safe retry behavior.

## Scope
Test cost/token boundaries, expired reservations, duplicate settlement, idempotent tools, and unknown outcomes.

## Acceptance criteria
- Overspend dan oversubscription ditolak deterministically.
- Duplicate reserve/settle tidak merusak ledger.
- Crash recovery mengembalikan atau merekonsiliasi lease sesuai policy.
- Non-idempotent unknown outcome tidak di-retry otomatis.
- Ledger totals cocok dengan usage fixtures.

## Handoff
Gunakan provider/tool mock dan numeric values yang reproducible.

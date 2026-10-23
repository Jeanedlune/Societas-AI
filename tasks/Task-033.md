# Task-033 - Implementasikan Digest Approval

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P1
- **Depends on:** 032, 023
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Kelompokkan tampilan request high-risk tanpa menggabungkan otorisasi item.

## Scope
Implementasikan immutable manifest, ordered item IDs, partial results, durable item receipts, and revalidation.

## Acceptance criteria
- Critical tidak pernah masuk digest.
- Manifest berubah jika membership/order/input berubah.
- Setiap item mendapat revalidation dan grant terpisah.
- Duplicate submit tidak menggandakan grant atau aksi.
- Tests mencakup partial grant, stale item, duplicate, dan malformed manifest.

## Handoff
Summary/teks bantu LLM tidak boleh menambah item approval.

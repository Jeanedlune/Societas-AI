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

## Hasil

- Implementasi `internal/budget` menyediakan grant, reserve atomik dengan TTL lease, settle/refund, recovery lease kedaluwarsa, saldo scope, dan rekonstruksi saldo dari ledger.
- Recovery dapat menerima `RecoveryEventAppender`; event `budget.settled` dipersist bersama refund dan perubahan lease di transaksi SQLite yang sama. Runtime wajib menghubungkan appender ke Event Store.
- Migration 0005 membuat ledger append-only melalui trigger update/delete. Tes menutup retry/crash recovery, event rollback atomik, oversubscription paralel, rekonstruksi saldo, dan immutability.
- Verifikasi: `go test ./internal/budget ./internal/storage/sqlite` lulus.

# Task-007 - Siapkan SQLite Migrations

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P0
- **Depends on:** 003, 005
- **Source:** [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Siapkan operational database lokal untuk workspace, agent, run, task, event, artifact metadata, approval, memory, dan usage.

**Baseline:** [storage operasional dan artifact](../docs/00-mvp-implementation-baseline.md).

## Scope
Buat migration runner, schema versi, transaction helper, dan fixture database test.

## Acceptance criteria
- Database baru dapat dibuat dari nol melalui migrations.
- Upgrade dari versi sebelumnya deterministic dan transactional.
- Foreign key dan index utama aktif.
- SQLite busy/error handling tidak menghilangkan data.
- Integration test membuka database sementara dan memvalidasi tabel inti.

## Handoff
File artifact disimpan terpisah; SQLite menyimpan metadata dan references.

## Hasil
- `internal/storage/sqlite` menyediakan `Open`, `Migrate`, dan `WithTx`. Migration embedded berurutan, checksum SHA-256, dan mengubah `PRAGMA user_version` dalam transaksi yang sama dengan DDL.
- Migration 0001 menyiapkan workspace, agent, run, task, event, message, artifact metadata, approval, memory, dan usage; migration 0002 menambahkan budget ledger dan reservation lease; migration 0003 memperbaiki scope idempotency event ke workspace/run dan mengunci event append-only.
- Foreign key aktif per koneksi; database file memakai WAL, `synchronous=FULL`, dan busy timeout 5 detik. Busy/commit errors selalu dikembalikan agar caller tidak melaporkan write sukses.
- Integration tests memakai database sementara untuk inisialisasi, upgrade dari schema versi 1, rollback migration gagal, foreign key/status/index, transaksi, dan lock contention.

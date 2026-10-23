# Task-007 - Siapkan SQLite Migrations

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P0
- **Depends on:** 003, 005
- **Source:** [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Siapkan operational database lokal untuk workspace, agent, run, task, event, artifact metadata, approval, memory, dan usage.

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

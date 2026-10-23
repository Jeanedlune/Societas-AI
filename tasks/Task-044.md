# Task-044 - Lindungi Aksi Publikasi Eksternal

- **Phase:** 4 - Workflow general
- **Priority:** P0
- **Depends on:** 029, 032, 043
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Pastikan publish/send/export eksternal tidak terjadi sebelum approval manusia yang mengikat konten dan target.

**Baseline:** [workflow strategi konten dan approval publikasi](../docs/00-mvp-implementation-baseline.md).

## Scope
Implementasikan publish request adapter, normalized target/content snapshot, permission gate, approval dispatch, and receipt.

## Acceptance criteria
- Tidak ada dispatch sebelum grant valid.
- Grant mengikat artifact version dan target/kanal.
- Reject, stale artifact, expiry, atau policy change menghasilkan nol side effect.
- Replay/double submit tidak menghasilkan publish kedua.
- Tests menggunakan mock publisher dan memeriksa jumlah side effect.

## Handoff
MVP boleh memulai dengan mock/paper publish bila provider nyata belum disepakati.

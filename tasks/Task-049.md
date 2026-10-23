# Task-049 - Bangun Approval Center

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P0
- **Depends on:** 032, 033, 045
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Tampilkan approval detail, risk, action, snapshot/source versions, expiry, dan partial batch outcomes.

## Scope
Implementasikan inspect, approve once, reject, batch validate, stale refresh, and outcome display.

## Acceptance criteria
- User dapat memeriksa payload aksi dan resource sebelum approve.
- Critical tidak dapat masuk digest/bulk approval.
- Stale/expired approval tidak bisa di-grant.
- Batch result menampilkan granted/skipped per item dengan reason.
- UI tests membuktikan click/retry tidak menggandakan command.

## Handoff
Tombol UI hanya mengirim command; backend tetap source of truth.

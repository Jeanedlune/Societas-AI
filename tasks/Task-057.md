# Task-057 - Tambahkan Approval Security Tests

- **Phase:** 6 - Validasi dan rilis
- **Priority:** P0
- **Depends on:** 029, 031, 032, 033
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 09](../docs/09-security-sandbox-observability.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Uji binding, principal auth, once consumption, expiry, risk rules, and external-action protection.

## Scope
Tambahkan attack-oriented tests untuk altered snapshot, stale resources, forged actor, race, and duplicate batch.

## Acceptance criteria
- Altered snapshot/hash tidak dapat grant.
- Forged principal atau wrong workspace/run ditolak.
- One-time grant hanya menghasilkan satu side effect saat race.
- Critical approval tidak masuk bulk digest.
- Reject/expired approval menghasilkan nol external dispatch.

## Handoff
Tidak perlu menguji exploit terhadap provider nyata; gunakan mock boundary.

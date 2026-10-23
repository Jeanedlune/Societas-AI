# Task-017 - Implementasikan Delegation dan Routing

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 004, 016
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Terima task delegation request dari agent, validasi, pilih owner, dan buat task melalui Orchestrator.

## Scope
Validasi assignee terhadap registry, policy, dependency, budget, depth, fan-out, dan route fallback.

## Acceptance criteria
- Agent hanya meminta delegation dan tidak membuat task langsung.
- Unknown assignee ditolak sebelum task dibuat.
- Router memilih dari agent aktif saja.
- Rejection mengembalikan code dan reason stabil.
- Tests mencakup valid assignee, blind delegation, permission deny, dan limit exceeded.

## Handoff
Simpan rationale delegation pada task/event untuk audit keputusan routing.

## Hasil
- `internal/delegation` menerima request delegasi tervalidasi dan hanya memanggil interface Orchestrator untuk membuat task; agent tidak memperoleh jalur pembuatan task langsung.
- Assignee eksplisit harus ada di registry, aktif, di bawah parallelism limit, dan diizinkan policy. Blind routing memilih agent aktif dengan beban terendah, lalu ID sebagai tie-breaker.
- Depth, fan-out, child count, dan seluruh dimensi budget diperiksa sebelum task dibuat. Rejection membawa code stabil; rationale dan alasan routing diteruskan dalam `Decision` untuk disimpan Orchestrator.
- Unit tests mencakup assignee valid/tidak dikenal, blind route, permission deny, limit/budget exceeded, dan memastikan Orchestrator hanya dipanggil untuk request valid.

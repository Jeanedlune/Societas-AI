# Task-045 - Bangun Organization dan Agent View

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P1
- **Depends on:** 012, 014
- **Source:** [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 01](../docs/01-vision-and-core.md)

## Goal
Tampilkan agent terdaftar, role, status, model, task aktif, usage, dan permission ringkas.

## Scope
Implementasikan organization view, agent detail, empty/error/loading states, and live status updates.

## Acceptance criteria
- Status agent berasal dari backend, bukan state client-only.
- Secret dan internal prompt tidak ditampilkan.
- View berfungsi untuk jumlah agent kecil maupun banyak tanpa overlap.
- Permission/error state jelas.
- UI tests mencakup idle, working, blocked, unavailable, dan empty workspace.

## Handoff
Hormati role names dari workspace config.

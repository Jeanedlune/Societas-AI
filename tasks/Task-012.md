# Task-012 - Sediakan API Workspace dan Health

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P1
- **Depends on:** 002, 007
- **Source:** [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 09](../docs/09-security-sandbox-observability.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Sediakan API lokal untuk membaca workspace dan status service.

## Scope
Implementasikan endpoint workspace, agents, runs, health, readiness, dan versioned API response.

## Acceptance criteria
- Health membedakan service hidup dari dependency siap.
- Workspace/config tidak menampilkan secret.
- API errors memakai bentuk konsisten dan request ID.
- Endpoint read-only tidak membuat side effect.
- API integration tests mencakup valid, unauthorized, missing, dan unhealthy dependency.

## Handoff
Gunakan address/bind defaults yang tidak mengekspos service ke jaringan tanpa konfigurasi.

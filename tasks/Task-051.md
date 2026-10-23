# Task-051 - Bangun Workspace Settings

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P1
- **Depends on:** 003, 029, 045
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Kelola agent roles, provider references, tool capabilities, budget defaults, risk policy, dan session options.

**Baseline:** [workspace config dan secret handling](../docs/00-mvp-implementation-baseline.md).

## Scope
Implementasikan settings read/edit/validate, diff preview, save behavior, and secret redaction.

## Acceptance criteria
- Config change divalidasi sebelum disimpan.
- Risk/permission changes ditampilkan sebelum apply.
- Provider secrets tidak pernah diprefill atau ditampilkan.
- Invalid config dapat dibatalkan tanpa merusak active settings.
- Tests mencakup validation errors, persistence, and redaction.

## Handoff
Perubahan policy membatalkan approval yang binding-nya terdampak.

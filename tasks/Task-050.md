# Task-050 - Bangun Usage dan Budget Dashboard

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P1
- **Depends on:** 019, 021, 045
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Tampilkan token, cost estimate, model calls, tool calls, remaining budget, dan per-agent breakdown.

## Scope
Implementasikan time range, aggregation queries, warnings, exhausted state, and usage drilldown.

## Acceptance criteria
- Angka berasal dari persisted usage/ledger.
- Workspace/run/task totals konsisten dengan ledger.
- Estimasi diberi label dan tidak diklaim sebagai invoice final.
- Budget warning/exceeded muncul sesuai threshold.
- Tests mencakup empty usage, refunds, and concurrent settlement.

## Handoff
Jangan memanggil provider eksternal untuk merender dashboard.

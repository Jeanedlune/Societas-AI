# Task-039 - Implementasikan Coordinator Intake

- **Phase:** 4 - Workflow general
- **Priority:** P0
- **Depends on:** 017, 019, 022
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Ubah request bahasa natural menjadi brief dan task DAG dengan inputs, expected outputs, constraints, dan risk hint.

## Scope
Implementasikan intake schema, ambiguity detection, clarifying-question path, agent assignment, and task planning.

## Acceptance criteria
- Brief memisahkan goal, sources, period, audience, expected output, constraints.
- Missing critical input menghasilkan pertanyaan, bukan asumsi diam-diam.
- DAG hanya memakai agent aktif.
- Delegation dan budget melalui Orchestrator.
- Tests mencakup typo, ambiguity, missing data source, dan clear request.

## Handoff
Default demo adalah strategi konten dari tren pasar dan performa penjualan.

# Task-036 - Implementasikan Memory Retrieval dan Admission

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P1
- **Depends on:** 035, 031
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Retrieve memory yang relevan dan valid, lalu cegah hasil stale masuk ke model context.

## Scope
Implementasikan metadata filtering, recency/freshness, scope isolation, admission guard, tombstone, and provenance checks.

## Acceptance criteria
- Retrieval mengembalikan reference IDs, bukan menganggap vector row sebagai source of truth.
- Setiap ID dicek ulang ke operational store.
- Stale, superseded, unapproved, dan cross-scope items dikeluarkan.
- Retrieval mencatat reason/score dan tunduk pada context budget.
- Tests memakai dataset sintetis dengan conflict dan stale versions.

## Handoff
Similarity tidak boleh dianggap bukti kebenaran.

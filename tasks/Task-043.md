# Task-043 - Implementasikan Editor Review Workflow

- **Phase:** 4 - Workflow general
- **Priority:** P0
- **Depends on:** 037, 042
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Editor memeriksa draft terhadap brief, evidence, audience, dan policy sebelum hasil diserahkan.

**Baseline:** [workflow strategi konten MVP](../docs/00-mvp-implementation-baseline.md).

## Scope
Implementasikan criteria presets, structured findings, verdict, revision request, and artifact binding.

## Acceptance criteria
- Review menunjukkan draft artifact ID/version yang diperiksa.
- Setiap finding punya severity, explanation, dan evidence reference.
- Request changes dapat menghasilkan siklus revision/re-review terbatas.
- Review tidak melakukan external publish atau memberikan approval.
- Integration tests mencakup approve, revise, reject, and missing source.

## Handoff
Approval eksternal adalah tahap terpisah setelah review.

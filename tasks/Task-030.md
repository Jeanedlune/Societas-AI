# Task-030 - Tangani Retry dan Outcome Unknown

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P0
- **Depends on:** 011, 025, 029
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Pastikan tool yang mungkin sudah menimbulkan side effect tidak diulang secara buta.

## Scope
Implementasikan outcome classes, operation key policy, reconciliation hook, retry budget, and human escalation path.

## Acceptance criteria
- `not_started`, `completed`, dan `outcome_unknown` dibedakan.
- Unknown outcome pada tool non-idempotent tidak auto-retry.
- Retry memakai argumen dan operation key yang sama.
- Duplicate command mengembalikan receipt sebelumnya.
- Tests mencakup lost response sesudah dispatch dan reconciliation result.

## Handoff
Retry policy berlaku lintas provider tools, bukan satu connector saja.

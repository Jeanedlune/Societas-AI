# Task-006 - Bangun Schema Validation

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P0
- **Depends on:** 005
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Validasi envelope dan payload di boundary publish serta receive.

## Scope
Tambahkan registry event, version routing, validation errors, dan strict unknown-field behavior.

## Acceptance criteria
- Event type dan payload valid diterima.
- Missing principal, field wajib, atau payload invalid ditolak.
- Unknown event type/field ditolak sebelum persistence.
- Validation error mengembalikan code stabil tanpa membocorkan secret.
- Unit tests mencakup minimal satu event dari setiap kategori MVP.

## Handoff
Gunakan contract package dari Task-005; jangan membuat schema duplikat.

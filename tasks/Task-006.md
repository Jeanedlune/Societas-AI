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

## Hasil
- `internal/contracts/v1.Validator` memvalidasi strict envelope dan payload memakai schema §72A yang di-embed; event serta versi protocol/schema dirutekan sebelum payload diterima.
- Kode error stabil adalah `INVALID_JSON`, `INVALID_ENVELOPE`, `UNSUPPORTED_VERSION`, `UNKNOWN_EVENT_TYPE`, `INVALID_PAYLOAD`, dan `PAYLOAD_TOO_LARGE`; error tidak membawa nilai field atau isi payload.
- Payload dihitung sebagai byte UTF-8 JCS RFC 8785 setelah validasi schema: maksimum 8.000 byte, atau 16.000 untuk `tool.call_completed`. Pengujian mencakup batas tepat dan batas + 1.
- Unit tests mencakup event dari kategori workspace, control, run, task, communication, model, budget, policy, tool, artifact, summary, review, dan approval, serta missing principal, required field, unknown event/field, versi tak didukung, payload invalid, dan error redaction.

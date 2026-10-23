# Task-053 - Tambahkan Contract Tests

- **Phase:** 6 - Validasi dan rilis
- **Priority:** P0
- **Depends on:** 005, 006
- **Source:** [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Pastikan schema, registry, envelope, payload size, and compatibility contract tervalidasi di CI.

## Scope
Tambahkan positive/negative fixtures untuk semua event types dan schema version handling.

## Acceptance criteria
- Setiap event registry entry punya payload schema.
- Semua canonical examples valid terhadap envelope+payload schema.
- Unknown type/field, missing principal, dan malformed JSON ditolak.
- Payload batas size diuji pada limit dan limit+1.
- CI gagal dengan output yang menunjuk schema/event/fixture.

## Handoff
Fixtures memakai data sintetis, tanpa customer information atau credentials.

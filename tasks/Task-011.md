# Task-011 - Tambahkan Idempotency dan Replay

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P1
- **Depends on:** 008, 010
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Buat command/event retry aman dan sediakan replay state per run.

## Scope
Implementasikan idempotency key storage, duplicate response behavior, replay reader, dan event-to-state projection.

## Acceptance criteria
- Duplicate request dengan key sama tidak menjalankan side effect dua kali.
- Key sama dengan payload berbeda ditolak.
- Replay dari event store membangun state run/task yang sama.
- Event historis tetap immutable.
- Tests membuktikan duplicate delivery, restart, dan replay determinism.

## Handoff
Jangan menganggap event idempotency sebagai jaminan deduplikasi tool eksternal.

## Hasil
- Event Store membandingkan identitas command kanonik (workspace/run/task, jenis event, routing, principal, correlation, key, dan payload JCS). Retry identik dengan event ID baru mengembalikan event tersimpan; key/ID yang dipakai untuk request berbeda ditolak sebagai `IDEMPOTENCY_CONFLICT`.
- `internal/replay` membaca stream persisten berhalaman dan memproyeksikan state run/task lifecycle. Snapshot kanonik mengurutkan task secara deterministik.
- Tests mencakup conflict key, replay task deterministik, dan pemulihan state sesudah database ditutup/dibuka kembali.
- Keputusan ini memperjelas I7 aktif: duplikasi hanya aman bila isi request sama; tool side effect eksternal tetap memerlukan deduplikasi provider terpisah.

# Task-009 - Implementasikan Transactional Outbox

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P0
- **Depends on:** 008
- **Source:** [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Pastikan event yang committed akan dikirim ke subscriber setelah restart tanpa kehilangan atau pengiriman ganda yang terlihat.

## Scope
Implementasikan outbox insert dalam transaksi domain, dispatcher, delivery state, retry, dan recovery loop.

## Acceptance criteria
- Event dan outbox row committed atomically.
- Dispatcher hanya mengirim row committed.
- Event yang sudah persisted dapat dipulihkan saat restart.
- Subscriber menerima event dengan idempotency identity yang sama saat retry.
- Tests mensimulasikan crash di antara persistence dan delivery.

## Handoff
Batasi retry dengan backoff dan observable error state.

## Hasil
- Migration SQLite 0004 membuat `event_outbox`, indeks kesiapan/stream, dan backfill untuk event yang sudah tersimpan.
- `EventStore.Append` menulis event dan row `pending` dalam transaksi SQLite yang sama. Dispatcher hanya membaca outbox, mengambil event persisted by ID, lalu mengirimnya.
- Lease berakhir dapat dipulihkan saat restart. Retry memakai ID event/idempotency yang sama, exponential backoff dengan batas, status `dead` dan error code stabil; `RetryDead` membuka retry operator.
- Integration tests menutup database tepat setelah persistence, membuka kembali, lalu membuktikan event berhasil terkirim; retry juga diuji hingga dead-letter.

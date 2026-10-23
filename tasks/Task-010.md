# Task-010 - Implementasikan Event Bus

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P0
- **Depends on:** 006, 009
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 08](../docs/08-runtime-architecture.md)

## Goal
Sediakan komunikasi publish/subscribe internal yang dimediasi Control Plane.

## Scope
Implementasikan topic routing, subscriber lifecycle, bounded queues, backpressure, dan outbox delivery adapter.

## Acceptance criteria
- Publish memvalidasi event sebelum persistence.
- Subscriber menerima event setelah persistence saja.
- Agent tidak dapat memanggil agent lain secara langsung.
- Slow subscriber tidak memblokir event writer tanpa batas.
- Integration tests mencakup ordering, unsubscribe, retry, dan backpressure.

## Handoff
Jaga interface transport-neutral agar implementasi internal dapat berkembang.

## Hasil
- `eventbus.Bus.Publish` memakai validator/Event Store sebelum commit. Event hanya diteruskan melalui adapter `Deliver` yang dipanggil dispatcher atas row committed.
- Subscriber memakai alamat envelope/topic, filter workspace/run opsional, channel bounded, lifecycle `Close`, dan replay historis berurutan.
- Queue penuh melaporkan `BACKPRESSURE` tanpa memblokir writer; retry event mempertahankan event ID dan deduplikasi per subscription aktif. Tidak ada API pengiriman agent-ke-agent langsung.
- Integration tests mencakup publish-persist-deliver, urutan per run, unsubscribe, backpressure, dan retry tanpa duplikasi yang terlihat.

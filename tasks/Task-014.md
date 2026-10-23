# Task-014 - Implementasikan Agent Lifecycle

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 004, 010
- **Source:** [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 08](../docs/08-runtime-architecture.md)

## Goal
Kelola lifecycle agent dan expose state aktivitasnya.

## Scope
Implementasikan create/load, idle, working, waiting, blocked, stop, dan state-to-event mapping.

## Acceptance criteria
- State valid ditransisikan hanya oleh Orchestrator.
- State agent dibedakan dari status task.
- Restart memulihkan state dari durable records, bukan RAM saja.
- Perubahan state menghasilkan event terdaftar.
- Unit tests mencakup transisi valid dan invalid.

## Handoff
Agent berhenti atau idle setelah task park/terminal dan tidak memegang worker kosong.

## Hasil
- `agent.Lifecycle` memvalidasi transisi dari graph lifecycle di Doc 03, membedakan status agent dari status task, dan membaca status durable dari SQLite.
- Event kanonik `agent.status_changed` ditambahkan ke §72A.7 agar setiap transisi status dapat direplay, bukan menafsirkan `agent.started/stopped` untuk status lain.
- CAS status, event, dan outbox row commit dalam satu transaksi; event retry identik idempotent dan transisi ilegal ditolak.

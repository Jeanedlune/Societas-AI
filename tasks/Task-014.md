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

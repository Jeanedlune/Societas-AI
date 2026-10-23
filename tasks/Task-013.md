# Task-013 - Sediakan CLI Init dan Run

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P1
- **Depends on:** 002, 003, 012
- **Source:** [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Beri pengguna jalur terminal sederhana untuk membuat workspace dan memulai service lokal.

**Baseline:** [deployment lokal dan config workspace](../docs/00-mvp-implementation-baseline.md).

## Scope
Implementasikan `init`, `serve`, `start`, `status`, dan `stop` sesuai keputusan Task-001.

## Acceptance criteria
- `init` menghasilkan config valid tanpa secret literal.
- `serve` menampilkan alamat dan status startup yang jelas.
- Invalid config menghasilkan petunjuk field yang bisa diperbaiki.
- CLI tidak menelan error server atau exit code.
- Smoke test mencakup init lalu serve lalu shutdown.

## Handoff
Jangan menambah command implementasi engineering/development workflow.

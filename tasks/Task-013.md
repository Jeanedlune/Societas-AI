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

## Hasil
- CLI menyediakan `init`, `serve`, `start`, `status`, dan `stop`. `init` menghasilkan YAML minimal yang tervalidasi, finite-budget, dan tidak mengandung credential literal.
- `serve` memberi error config dengan path field, membuka/migrasi SQLite, menyinkronkan workspace dan safe agent metadata, lalu menjalankan API/readiness. `start` membuat proses background dengan PID/instance identity; `stop` hanya mengirim interrupt setelah identitas service cocok dengan readiness endpoint.
- Process smoke test menggunakan config hasil init dan memeriksa startup, health, serta shutdown graceful. Unit tests memeriksa config secret-free, bootstrap DB, dan petunjuk config invalid.
- Jalur pemakaian dan command dicatat di README.

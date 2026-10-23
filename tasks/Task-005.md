# Task-005 - Tetapkan Kontrak Domain Kanonik

- **Phase:** 1 - Kontrak, storage, dan event
- **Priority:** P0
- **Depends on:** 001
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Turunkan envelope, task, artifact, tool, review, approval, budget, dan event ke package contract versioned.

## Scope
Samakan tipe backend dengan kontrak aktif Doc 10. Jangan mengimpor definisi dari archive.

## Acceptance criteria
- Setiap event MVP memiliki satu payload type.
- Principal, correlation, causation, workspace/run, dan task identity terdefinisi.
- Enum task/action/status cocok dengan Doc 10 aktif.
- Test fixtures umum mencakup konten, riset, sales, approval, dan invalid payload.
- Contract package hanya memuat event dan action yang termasuk scope MVP aktif.

## Handoff
Jika kontrak aktif kurang detail, catat pertanyaan; jangan menyalin kontrak arsip.

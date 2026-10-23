# Task-022 - Implementasikan Context Assembly

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P1
- **Depends on:** 019, 023, 035
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 05](../docs/05-artifacts-memory-tools.md)

## Goal
Rakit context per agent/task dari role, workspace brief, history, memory, tools, dan input terbaru dalam budget token.

## Scope
Implementasikan source ordering, relevance selection, token estimate, stale filtering, provenance tracking, and dropped-item reasons.

## Acceptance criteria
- Context tidak melebihi configured budget.
- Setiap item memiliki source/ref, token estimate, dan inclusion reason.
- Superseded/stale/untrusted memory dikeluarkan.
- Summary/artifact references dipakai sebelum memuat konten penuh.
- Deterministic tests mencakup budget overflow dan filtering.

## Handoff
Jangan memasukkan seluruh transcript ke semua agent.

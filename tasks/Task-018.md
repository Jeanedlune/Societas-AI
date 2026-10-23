# Task-018 - Buat Provider Interface

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 001, 003
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 02](../docs/02-ai-control-plane.md)

## Goal
Definisikan interface netral untuk model providers, usage, streaming, timeout, dan error.

## Scope
Implementasikan provider adapter boundary, request/response types, capability discovery, secret loading, dan test double.

## Acceptance criteria
- Router tidak bergantung pada SDK provider tertentu.
- Provider response memetakan usage dan finish reason ke tipe kanonik.
- Provider error dipetakan ke error catalog.
- Secret tidak ditulis ke log atau error.
- Mock provider dapat dipakai untuk deterministic integration tests.

## Handoff
Provider pertama ditentukan oleh keputusan Task-001.

# Task-025 - Definisikan Tool Capabilities

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P0
- **Depends on:** 003, 005
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Definisikan contract dan registry bagi tool yang dapat dipakai agent.

## Scope
Catat schema input/output, idempotency, operation key support, timeout, resource scope, dan permission required.

## Acceptance criteria
- Capability tidak dapat ditentukan atau diubah oleh agent.
- Unknown tool/capability ditolak default-deny.
- Idempotency menyatakan jaminan provider yang nyata.
- Schema validasi tool menghasilkan error terstruktur.
- Contract tests mencakup valid, invalid, timeout, dan unknown capability.

## Handoff
Tool catalog awal MVP fokus pada browser/search, spreadsheet, dan artifact files.

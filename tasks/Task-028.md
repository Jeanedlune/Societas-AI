# Task-028 - Implementasikan Artifact Filesystem Tool

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P1
- **Depends on:** 023, 025, 029
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 09](../docs/09-security-sandbox-observability.md)

## Goal
Sediakan tool baca/tulis artifact workspace tanpa akses file di luar scope.

## Scope
Implementasikan read, create, list-by-task, version update, path normalization, and isolation.

## Acceptance criteria
- Agent hanya dapat mengakses artifact sesuai workspace/task permission.
- Traversal, absolute path, symlink escape, dan oversize write ditolak.
- Content write menghasilkan immutable version/checksum.
- Tool error mengungkapkan path aman tanpa host secrets.
- Security tests mencakup traversal dan cross-workspace access.

## Handoff
Gunakan Artifact Service untuk metadata dan versioning.

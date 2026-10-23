# Task-035 - Implementasikan Memory Store

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P1
- **Depends on:** 007, 023
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Simpan agent, workspace, task, dan run memory dengan source, scope, version, freshness, dan supersede state.

## Scope
Implementasikan operational metadata, optional vector adapter, embed/retrieve interface, and admission hooks.

## Acceptance criteria
- Memory terstruktur tetap tersimpan di operational store.
- Vector storage hanya menerima memory yang lolos admission policy.
- Source/version/checksum tersedia untuk setiap admitted entry.
- Superseded/stale memory tidak ikut context.
- Unit tests mencakup scope isolation dan lifecycle.

## Handoff
Jangan embed seluruh chat atau data sensitif secara otomatis.

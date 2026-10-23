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

## Hasil

- `workspace.yaml` menerima schema input/output, timeout, resource scope, permission, idempotency, dan operation-key guarantee sebagai konfigurasi tepercaya.
- `internal/toolcap` membangun registry default-deny, compile JSON Schema lokal, memblokir remote `$ref`, memvalidasi request/response, membatasi timeout, dan memeriksa permission sebelum handler dipanggil.
- Retry hanya aman untuk capability idempotent atau operation key yang ditegakkan capability dan dijamin deduplicate oleh provider. Handler tool browser/spreadsheet/files belum dibuat; ini kontrak/runtime registry untuk Task 026–028.
- Tes mencakup schema valid/invalid, unknown capability, permission, output, timeout, dan retry safety. Verifikasi: `go test ./internal/toolcap ./internal/config` lulus.

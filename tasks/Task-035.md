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

## Hasil

- `internal/memory` menyimpan kandidat di SQLite dengan scope/owner, source artifact version/checksum, confidence, freshness, sensitivity, tag, dan supersede status.
- Hanya sumber artifact fresh yang terkait task `completed` dengan `result_status: success`, tidak sensitif, dan lolos policy yang dapat diteruskan ke vector adapter. Retrieval vector selalu diverifikasi ulang ke SQLite.
- Adapter vector adalah interface opsional; belum ada vendor/vector database bawaan. Admission gagal tertutup saat adapter tidak tersedia. Ini menjaga jalur provider tetap bisa dipilih kemudian tanpa mengirim chat atau data sensitif otomatis.
- Tes mencakup admission, isolasi scope, source freshness, supersede, sensitivitas, dan task belum sukses. Verifikasi: `go test ./internal/memory` lulus.

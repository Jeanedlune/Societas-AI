# Task-023 - Implementasikan Artifact Service

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 007, 015
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Simpan output agent sebagai artifact berversi yang dapat dibaca agent lain dan direferensikan event.

## Scope
Implementasikan content store, metadata, versioning, checksums, access scope, and supersede status.

## Acceptance criteria
- Artifact write menghasilkan ID, version, checksum, size, owner, dan task reference.
- Read memverifikasi checksum dan permission.
- Update membuat versi baru, bukan overwrite diam-diam.
- Konten besar tidak dikirim inline pada event.
- Tests mencakup access denial, version read, dan integrity mismatch.

## Handoff
Path fisik storage tidak bocor sebagai asumsi client API.

## Hasil

- `internal/artifacts` menyimpan konten privat per workspace/ID/versi dan metadata di SQLite; event artifact divalidasi dan ditulis dalam transaksi yang sama.
- Update menggunakan compare-and-swap versi baru; versi sebelumnya ditandai stale/superseded. Read memeriksa requester, ukuran, checksum, dan referensi sumber.
- API tidak mengembalikan path fisik. Konten ditulis melalui file sementara lalu rename; crash sesudah rename sebelum commit dapat meninggalkan file yatim yang dapat dibersihkan oleh GC mendatang.
- Tes mencakup izin, versioning, retry idempotent, traversal path, ketidakcocokan event, dan integritas konten. Verifikasi: `go test ./internal/artifacts` lulus.

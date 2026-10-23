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

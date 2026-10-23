# Task-048 - Bangun Artifact Explorer

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P1
- **Depends on:** 023, 045
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Telusuri artifact per run/task/agent dan lihat versi, sumber, checksum, dan review.

## Scope
Implementasikan list, search, preview/download, version comparison, source references, and permission checks.

## Acceptance criteria
- Versi lama dapat dibaca tanpa mengganti versi terbaru.
- Source references dan freshness terlihat.
- Artifact unavailable/corrupt memberi state error yang jelas.
- Akses lintas workspace ditolak.
- UI tests mencakup text, tabular, large, and unsupported artifact types.

## Handoff
Konten besar menggunakan pagination/streaming sesuai limit.

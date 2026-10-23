# Task-001 - Kunci Baseline Implementasi MVP

- **Phase:** 0 - Fondasi produk
- **Priority:** P0
- **Depends on:** none
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Tetapkan keputusan minimum yang diperlukan sebelum implementasi dimulai, tanpa menambah scope di luar MVP general.

**Hasil:** [Baseline Implementasi MVP](../docs/00-mvp-implementation-baseline.md) mencatat keputusan, owner, asumsi, dan pertanyaan terbuka.

## Scope
Catat bahasa/runtime, framework UI, provider model awal, deployment lokal, storage, dan mekanisme config. Tandai pilihan yang belum disepakati sebagai decision required.

## Acceptance criteria
- Keputusan yang sudah diketahui tercatat dengan alasan dan owner.
- Assumption register memisahkan fakta, asumsi, dan pertanyaan terbuka.
- Workflow strategi konten tetap use case end-to-end utama.
- Tidak ada fitur workflow implementasi software sebagai dependency produk.
- Semua task berikutnya mereferensikan keputusan ini bila relevan.

## Keputusan utama

- Backend Go, deployment lokal self-hosted, SQLite + filesystem artifact, dan config `workspace.yaml` ditetapkan di [baseline](../docs/00-mvp-implementation-baseline.md).
- Bootstrap dan workflow lokal memakai mock provider; provider nyata, framework UI, serta vector store tetap **Decision required**.
- Workflow strategi konten menjadi alur end-to-end utama; workflow implementasi software bukan dependency produk.

## Handoff
Jangan memilih provider berbayar atau menyimpan secret dalam config contoh.

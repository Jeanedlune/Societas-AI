# Task-002 - Bootstrap Aplikasi Lokal

- **Phase:** 0 - Fondasi produk
- **Priority:** P0
- **Depends on:** 001
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Buat aplikasi minimal yang bisa dijalankan lokal dari clean checkout sesuai keputusan Task-001.

## Scope
Siapkan struktur modul backend, frontend bila dipilih, konfigurasi environment, dan command run/dev.

## Acceptance criteria
- Setup fresh environment berhasil mengikuti README tanpa langkah tersembunyi.
- Command run menyalakan service dengan health endpoint.
- Config lokal tidak berisi kredensial nyata.
- Test smoke memverifikasi proses start dan shutdown.
- Struktur modul mengikuti dependency DAG Doc 10.

## Handoff
Jangan membangun modul bisnis di bootstrap ini.

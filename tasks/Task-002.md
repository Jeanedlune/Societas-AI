# Task-002 - Bootstrap Aplikasi Lokal

- **Phase:** 0 - Fondasi produk
- **Priority:** P0
- **Depends on:** 001
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Buat aplikasi minimal yang bisa dijalankan lokal dari clean checkout sesuai keputusan Task-001.

**Baseline:** [runtime, UI, deployment, storage, dan provider](../docs/00-mvp-implementation-baseline.md).

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

## Hasil

- Server Go lokal tersedia melalui `go run ./cmd/societas`, bind default `127.0.0.1:7443`, dengan `GET /health` dan graceful shutdown pada `Ctrl+C`.
- Alamat bind dapat diubah melalui `SOCIETAS_ADDR`; bootstrap tidak memerlukan file konfigurasi atau credential.
- Smoke test proses memulai binary test, memeriksa health endpoint, mengirim interrupt, dan memastikan proses berhenti dengan sukses.
- Verifikasi: `go test ./...` lulus.

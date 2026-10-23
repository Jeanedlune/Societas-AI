# Task-003 - Definisikan Workspace Config

- **Phase:** 0 - Fondasi produk
- **Priority:** P0
- **Depends on:** 001
- **Source:** [Doc 07](../docs/07-dashboard-config-storage.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Definisikan config workspace yang memuat agent, provider, budget, tool capabilities, risk, dan session policy.

**Baseline:** [mekanisme config dan aturan secret](../docs/00-mvp-implementation-baseline.md).

## Scope
Buat schema config, defaults, loader, validator, dan contoh general untuk riset, sales, konten, serta editor.

## Acceptance criteria
- Config invalid menghasilkan error dengan path field yang jelas.
- Secret dibaca dari environment/secret reference, bukan literal.
- Role dan tool dapat dikonfigurasi tanpa enum role produk yang kaku.
- Default permission adalah deny untuk capability yang belum diberikan.
- Contoh config lulus schema validation.

## Handoff
Provider-specific fields harus berada di adapter/config namespace terpisah.

## Hasil

- Schema Go dan strict YAML loader tersedia di `internal/config`; error validasi menyebut path field.
- Default budget finite dan default permission `deny`; role dan model agent tetap config-driven.
- Contoh general di `config/workspace.example.yaml` memuat coordinator, riset pasar, sales, content creator, dan editor tanpa provider berbayar atau secret literal.
- Credential provider berada di `providers.<id>.credentials` sebagai environment reference dan dibaca saat adapter meminta credential.
- Verifikasi: `go test ./...` lulus.

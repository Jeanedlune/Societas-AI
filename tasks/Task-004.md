# Task-004 - Definisikan Agent Registry

- **Phase:** 0 - Fondasi produk
- **Priority:** P0
- **Depends on:** 003
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Bangun registry runtime agent dari workspace config.

**Baseline:** [scope MVP dan role yang tetap config-driven](../docs/00-mvp-implementation-baseline.md).

## Scope
Validasi ID agent unik, role/prompt/model/tool references, autonomy, permissions, dan escalation lead.

## Acceptance criteria
- Agent aktif hanya berasal dari config tervalidasi.
- ID duplikat, role kosong, dan capability tak dikenal ditolak.
- Lookup berdasarkan ID konstan dan deterministic selama satu run.
- Registry dapat menghasilkan daftar agent yang aman untuk routing.
- Unit tests mencakup config valid dan invalid.

## Handoff
Jangan hardcode nama role selain contoh konfigurasi.

## Hasil

- `internal/agent.Registry` dibangun hanya dari config yang lolos validasi; ID disimpan dan dikembalikan dalam urutan deterministik.
- Snapshot registry tidak berubah bila config asal atau salinan hasil lookup dimutasi.
- Routing options hanya memuat ID dan role, tanpa prompt, permission, tools, atau provider.
- Config memvalidasi role, system prompt, model/provider reference, capability tool terdaftar, permission shape, dan escalation lead.
- Verifikasi: `go test ./...`, audit spesifikasi, dan 29 regression test audit lulus.

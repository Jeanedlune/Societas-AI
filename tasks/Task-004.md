# Task-004 - Definisikan Agent Registry

- **Phase:** 0 - Fondasi produk
- **Priority:** P0
- **Depends on:** 003
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Bangun registry runtime agent dari workspace config.

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

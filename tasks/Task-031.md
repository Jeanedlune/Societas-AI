# Task-031 - Implementasikan Approval Snapshot

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P0
- **Depends on:** 005, 023
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Buat snapshot immutable yang mengikat aksi, input versions, resource state, policy, dan expiry.

## Scope
Implementasikan canonical serialization, hash, persistence, versioned reference, and verification.

## Acceptance criteria
- Snapshot hash deterministik terhadap canonical JSON.
- Ref mengikat artifact ID/version/checksum.
- Perubahan argumen, source version, resource version, atau policy mengubah binding.
- Snapshot corrupt/missing/unsupported ditolak.
- Unit tests mencakup canonicalization dan tampering.

## Handoff
Alur tool.execute dan budget.increase memakai profile inputs yang berbeda.

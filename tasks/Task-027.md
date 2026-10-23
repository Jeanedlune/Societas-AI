# Task-027 - Implementasikan Spreadsheet Import

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P1
- **Depends on:** 025, 029
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Baca dataset penjualan yang diizinkan untuk analisis per periode, metrik, dan produk.

## Scope
Implementasikan import CSV/XLSX sesuai stack terpilih, schema inspection, type handling, and row/size limits.

## Acceptance criteria
- File invalid atau melebihi limit ditolak dengan error yang dapat ditindaklanjuti.
- Import menyimpan sumber, waktu, checksum, sheet, dan row count.
- Missing values/type coercion dilaporkan, bukan disembunyikan.
- PII/sensitive columns mengikuti policy.
- Tests memakai dataset sintetis termasuk malformed dan empty rows.

## Handoff
Tidak boleh menggunakan data pelanggan sungguhan dalam fixtures.

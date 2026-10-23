# Task-024 - Implementasikan Summary Manager

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P1
- **Depends on:** 018, 022, 023
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 05](../docs/05-artifacts-memory-tools.md)

## Goal
Buat ringkasan terikat ke artifact/version untuk menghemat context tanpa menghilangkan provenance.

## Scope
Implementasikan summary generation, source artifact references, token estimate, and summary persistence.

## Acceptance criteria
- Summary selalu menunjuk artifact ID dan version yang diringkas.
- Summary tidak mengubah sumber asli.
- Ringkasan memisahkan temuan, asumsi, risiko, dan rekomendasi bila relevan.
- Output summary mengikuti budget dan schema.
- Tests memverifikasi source binding dan provider failure handling.

## Handoff
Tidak ada ringkasan LLM yang dianggap approval atau fakta terverifikasi otomatis.

## Hasil

- `internal/summarizer` membaca versi artifact tertentu, meminta JSON terstruktur berisi text/findings/risks/recommendation/sources, lalu menyimpan summary yang terikat ke artifact ID/version dan estimasi token.
- Output invalid, melebihi batas, atau sumber yang berubah saat generasi tidak disimpan. Summary tidak mengubah artifact dan tidak menghasilkan approval/verifikasi.
- Tes memverifikasi source binding, batas output, JSON invalid, dan kegagalan provider. Verifikasi: `go test ./internal/summarizer` lulus.

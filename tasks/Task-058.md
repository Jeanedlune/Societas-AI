# Task-058 - Jalankan End-to-End Workflow MVP

- **Phase:** 6 - Validasi dan rilis
- **Priority:** P0
- **Depends on:** 040, 041, 042, 043, 044, 047, 048, 049
- **Source:** [Doc 10](../docs/10-contracts-mvp-roadmap.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Buktikan satu request strategi konten berjalan dari intake sampai approval publikasi dengan provider/tool mock.

## Scope
Jalankan market research dan sales analysis paralel; hasilnya menjadi input draft, review, dan approval.

## Acceptance criteria
- Event trace menunjukkan delegation, dependencies, tools, artifacts, review, approval, and final result.
- Market research artifact memiliki source references.
- Sales analysis artifact memiliki periode, definisi metrik, dan limitation.
- Draft terikat ke kedua artifact versions dan mendapat review.
- Publish side effect terjadi sekali hanya sesudah grant valid.

## Handoff
Simpan trace/artifacts sebagai CI output yang aman dan sintetis.

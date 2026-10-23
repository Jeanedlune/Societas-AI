# Task-041 - Implementasikan Sales Analysis Workflow

- **Phase:** 4 - Workflow general
- **Priority:** P0
- **Depends on:** 027, 038, 039
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Sales Analyst membandingkan performa penjualan pada periode dan metrik yang eksplisit.

**Baseline:** [workflow strategi konten MVP](../docs/00-mvp-implementation-baseline.md).

## Scope
Implementasikan dataset validation, metric definitions, period comparison, segmentation, and report artifact.

## Acceptance criteria
- Periode pembanding dan metric definition tertulis pada output.
- Missing values dan data insufficiency muncul sebagai limitation.
- Perhitungan deterministik untuk total, change, and group comparison.
- Semua chart/table dapat ditelusuri ke imported dataset version.
- Unit tests memakai data sintetis termasuk zero baseline dan missing values.

## Handoff
Jangan menyimpulkan causal effect dari korelasi tanpa evidence.

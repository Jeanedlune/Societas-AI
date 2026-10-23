# Task-038 - Susun Role Presets General

- **Phase:** 4 - Workflow general
- **Priority:** P1
- **Depends on:** 003, 004
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Sediakan konfigurasi awal untuk Coordinator, Market Researcher, Sales Analyst, Content Creator, dan Editor.

## Scope
Tulis prompts, tool capabilities, autonomy defaults, output kinds, and review expectations.

## Acceptance criteria
- Preset adalah config, bukan role enum di runtime.
- Setiap role memiliki responsibility dan permission minimum.
- Content Creator tidak dapat publish tanpa approval.
- Sales Analyst hanya membaca dataset yang diizinkan.
- Config preset lulus validation dan smoke run.

## Handoff
Role tambahan tetap dapat dibuat melalui workspace config.

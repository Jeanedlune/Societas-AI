# Task-040 - Implementasikan Market Research Workflow

- **Phase:** 4 - Workflow general
- **Priority:** P0
- **Depends on:** 026, 035, 038, 039
- **Source:** [Doc 01](../docs/01-vision-and-core.md), [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Market Researcher menemukan tren, target audience context, dan kompetitor dengan sumber yang dapat ditelusuri.

## Scope
Implementasikan research plan, search execution, source collection, comparison, and report artifact.

## Acceptance criteria
- Setiap claim penting punya source URL dan retrieval timestamp.
- Output memisahkan fact, inference, unknown, dan recommendation.
- Data tidak relevan/expired ditandai atau dikeluarkan.
- Report disimpan sebagai versioned artifact dan dapat dibaca task lain.
- Workflow test memakai mock search results dan menguji source attribution.

## Handoff
Jangan mengarang angka pasar bila sumber tidak tersedia.

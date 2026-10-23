# Task-026 - Implementasikan Browser dan Search

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P0
- **Depends on:** 025, 029
- **Source:** [Doc 05](../docs/05-artifacts-memory-tools.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Berikan market researcher akses pencarian sumber eksternal secara terkontrol.

## Scope
Implementasikan browser/search adapter, normalized queries, source URLs, fetch limits, and result schema.

## Acceptance criteria
- Result menyertakan title, URL, retrieved time, dan content excerpt.
- Timeout, rate limit, invalid URL, dan provider errors dipetakan konsisten.
- Akses domain/network mengikuti workspace policy.
- Tool tidak menyatakan konten sebagai fakta tanpa provenance sumber.
- Integration tests memakai mock server/data.

## Handoff
Jangan simpan credential atau cookies di artifact output.

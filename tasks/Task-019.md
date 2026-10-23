# Task-019 - Implementasikan Model Router

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 018, 004
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 06](../docs/06-permissions-approvals-roles.md)

## Goal
Pilih provider/model berdasarkan role, task complexity, risk, availability, dan policy.

## Scope
Implementasikan route config, tier mapping, fallback policy, rate limits, and reason logging.

## Acceptance criteria
- Model selalu terdaftar dan diizinkan untuk workspace.
- Fallback hanya ke provider/model yang dikonfigurasi.
- Critical risk tidak dapat menurunkan tier di bawah policy minimum.
- Rate limit menghasilkan retry metadata, bukan loop tanpa batas.
- Unit tests mencakup route success, fallback, disabled model, dan rate limit.

## Handoff
Tidak ada provider credential di prompt, event, atau UI.

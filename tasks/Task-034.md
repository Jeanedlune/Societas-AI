# Task-034 - Implementasikan Pause dan Recovery

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P1
- **Depends on:** 015, 007
- **Source:** [Doc 03](../docs/03-tasks-and-lifecycle.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Park task tanpa menahan worker, pulihkan checkpoint saat restart, dan rekonsiliasi in-flight call.

## Scope
Implementasikan pause request/completion, deadline, checkpoint, interruption, resume, dan unknown outcome handling.

## Acceptance criteria
- Pause cooperative menunggu call settle sebelum state paused.
- Restart memulihkan task melalui interrupted state.
- Resume tidak menggandakan non-idempotent side effect.
- TTL dipersist dan ditegakkan saat runtime/startup.
- Tests mencakup pause, crash, resume, expiry, dan in-flight call.

## Handoff
Usage tetap dicatat walau task di-pause atau di-interrupt.

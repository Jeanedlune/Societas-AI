# Task-020 - Terapkan Budget Defaults dan Inheritance

- **Phase:** 2 - Control plane dan runtime
- **Priority:** P0
- **Depends on:** 003, 005
- **Source:** [Doc 02](../docs/02-ai-control-plane.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Terapkan finite defaults dan validasi child budget terhadap remaining parent budget.

## Scope
Implementasikan defaults per scope, reserve estimates, child allocation, and max-call/time limits.

## Acceptance criteria
- Budget kosong memakai defaults finite dari config.
- Child granted limits tidak melebihi sisa parent.
- Concurrent child allocations tidak oversubscribe parent.
- Exceeded request ditolak dengan `BUDGET_EXCEEDED`.
- Tests mencakup tokens, cost, concurrent grants, dan default behavior.

## Handoff
Jelaskan unit dan currency untuk seluruh limit.

## Hasil
- `internal/budget` menerapkan finite defaults per workspace/run/task/agent sesuai matrix Doc 10; configured nonzero limits override fallback. Limit nol di config tetap mengikuti kontrak config: gunakan default, bukan unlimited.
- Child budget di-reserve secara atomik oleh manager yang mengunci sibling allocation; seluruh dimensi diperiksa sebelum grant. Request berlebih memakai `BUDGET_EXCEEDED`; retry allocation identik idempotent, sedangkan key yang sama dengan limit berbeda ditolak.
- Settlement memeriksa penggunaan aktual, melepas sisa reservasi, mengenakan usage pada parent satu kali, dan menurunkan pemakaian token total bila tidak diberikan sebagai jumlah input + output.
- Unit: token/model call/tool call/message/rebuttal/parallel agent adalah hitungan integer; waktu adalah detik; biaya adalah USD desimal (`float64`). Unit tests mencakup defaults finite, batas token/biaya, settlement, idempotency, dan alokasi sibling concurrent tanpa oversubscribe.

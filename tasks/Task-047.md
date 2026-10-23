# Task-047 - Bangun Live Event Stream

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P1
- **Depends on:** 010, 045
- **Source:** [Doc 04](../docs/04-communication-and-events.md), [Doc 07](../docs/07-dashboard-config-storage.md)

## Goal
Tampilkan event workspace/run secara real-time dengan filter dan replay-safe reconnect.

## Scope
Implementasikan event subscription, cursor/sequence, filters, reconnect, and delivery state.

## Acceptance criteria
- Event ditampilkan dalam urutan sequence.
- Reconnect mengambil event tertinggal tanpa duplikasi UI.
- Filter per run, task, agent, dan event type.
- Payload sensitif disensor menurut policy.
- Tests mencakup disconnect/reconnect dan sequence gaps.

## Handoff
Event stream adalah observability surface, bukan channel command.

# Task-052 - Tambahkan Observability dan Diagnostics

- **Phase:** 5 - Dashboard dan observability
- **Priority:** P1
- **Depends on:** 008, 010, 012
- **Source:** [Doc 09](../docs/09-security-sandbox-observability.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Sediakan structured logs, traces, health checks, and diagnostics untuk run/task/tool lifecycle.

## Scope
Implementasikan correlation IDs, redaction, event linkage, latency/error counters, and doctor command.

## Acceptance criteria
- Setiap log/event span terhubung ke workspace/run/task bila relevan.
- Prompt, auth token, dan sensitive dataset values disensor.
- Health check melaporkan subsystem readiness.
- Diagnostics memberi next action tanpa menampilkan secrets.
- Tests memverifikasi redaction dan correlation propagation.

## Handoff
Observability harus tetap bekerja saat provider atau tool gagal.

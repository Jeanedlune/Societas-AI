# Task-029 - Implementasikan Policy Enforcement

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P0
- **Depends on:** 003, 005, 025
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 09](../docs/09-security-sandbox-observability.md)

## Goal
Enforce tool, network, data, and external-action permissions di backend, bukan hanya di prompt.

## Scope
Implementasikan default deny, per-agent capabilities, risk classification, path/data scope, audit decision, and policy version.

## Acceptance criteria
- Allow/deny/require-approval selalu punya rule ID dan reason.
- Model output tidak dapat mengubah permission atau risk secara langsung.
- Policy change invalidates approval yang mengikat policy lama.
- Unknown action/resource ditolak.
- Tests mencakup role scoping, deny, approval requirement, dan policy update.

## Handoff
Policy dapat dievaluasi tanpa memanggil LLM untuk aturan deterministik.

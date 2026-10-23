# Task-032 - Implementasikan Approval Lifecycle

- **Phase:** 3 - Tools, permission, approval, dan memory
- **Priority:** P0
- **Depends on:** 007, 010, 029, 031
- **Source:** [Doc 06](../docs/06-permissions-approvals-roles.md), [Doc 08](../docs/08-runtime-architecture.md), [Doc 10](../docs/10-contracts-mvp-roadmap.md)

## Goal
Implementasikan request, grant, reject, invalidation, expiry, and once-consumption.

## Scope
Persist approval state, enforce principal, atomically consume grants, and resume parked task safely.

## Acceptance criteria
- Grant hanya sah untuk matching approval ID dan bound hash.
- Reject/expiry/invalidation tidak berubah menjadi grant.
- Once grant tidak dapat mengizinkan dua side effect concurrent.
- Dispatch memeriksa kembali policy, resources, dan expiry.
- Integration tests mencakup race, replay, expired, stale input, dan reject.

## Handoff
Approval state disimpan durable; UI bukan source of truth.

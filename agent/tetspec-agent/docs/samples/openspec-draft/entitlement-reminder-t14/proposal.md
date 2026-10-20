> DRAFT: see trace-manifest.json for blockers.

## Why

Subscription Entitlements are reminded at T-60, T-30, T-7 and T-0 only, and CR-PBOM-1 adds the T-14 reminder milestone.

## What Changes

- Modify FR-AST-13 so the system alerts subscription Entitlements at T-60, T-30, T-14, T-7, T-0.
- Change the code target `internal/entitlement/reminder.go::Milestones` (CS-001, acceptance AC-REM-14).

## Capabilities

### New Capabilities
- `entitlement-reminder-t14`: subscription Entitlement reminders including the T-14 milestone (FR-AST-13).

### Modified Capabilities

## Impact

- Owner: the pack does not state an owner for this capability; see the open question in design.md.
- Affected code: `internal/entitlement/reminder.go::Milestones`.

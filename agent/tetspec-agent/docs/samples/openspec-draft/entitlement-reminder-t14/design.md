> DRAFT: see trace-manifest.json for blockers.

## Context

FR-AST-13 moves from revision 1.0 to 2.0 and adds the T-14 milestone to the daily Entitlement reminder.
Each milestone is sent exactly once, tracked through `reminded_milestones`.
Renewal beyond 60 days returns the Entitlement to Active and clears the reminded milestones.

## Goals / Non-Goals

**Goals:**
- Send exactly one T-14 reminder when a subscription Entitlement expires in 14 days.

**Non-Goals:**
- No change to FR-AST-13 behaviour other than the added milestone.

## Decisions

- Add T-14 to the milestones in `internal/entitlement/reminder.go::Milestones` and keep the once-per-milestone rule of BRL-48.

## Risks / Trade-offs

- The pack lists no alternatives or risks for this change.

## Open Questions

- Who owns the entitlement-reminder-t14 capability? The pack does not say.
- Is a rollback or migration needed? The pack's TDD text gives none.

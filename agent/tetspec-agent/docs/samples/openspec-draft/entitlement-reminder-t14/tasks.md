> DRAFT: see trace-manifest.json for blockers.

## 1. Reminder milestones

- [ ] 1.1 CS-001: modify `internal/entitlement/reminder.go::Milestones` to add T-14; verify with a test of: GIVEN a subscription Entitlement that expires in 14 days WHEN the daily job runs THEN exactly one T-14 reminder is sent (AC-REM-14)

# TASK-AT-02-05: Scheduler cadence 1 minute → 30 seconds

**From Solution:** SOL-AT-02
**Priority:** P2 — low-risk, low-priority per BUG-AT-02's own framing
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/config/config.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BL-AT-02's main flow specifies a 30-second scheduler tick; the current
default is 1 minute. One-line config change, no other design needed.

## Changes to make

In `config.go`, change:

```go
defaultSchedulerInterval = time.Minute
```

to:

```go
defaultSchedulerInterval = 30 * time.Second
```

Check for any tests or docs asserting the old 1-minute default and update
them alongside this change.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/config/...
grep -rn "defaultSchedulerInterval\|time.Minute" backend-go/services/automation-service/internal/config/
```

Expected: clean build; no stale test asserting a 1-minute default remains.

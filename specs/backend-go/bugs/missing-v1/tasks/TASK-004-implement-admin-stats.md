# TASK-004: Implement `GetAdminStats`

**From Solution:** SOL-001
**Priority:** P2 — lowest-risk, no design ambiguity
**Service:** `auth-service`
**File:** `services/auth-service/internal/usecase/get_admin_stats.go` (new)
**Depends on:** TASK-001, TASK-003 (counts `access_policies`)
**Status:** `[x]` DONE — `GetAdminStats` usecase and the three `Count*` repository methods exist, wired into the gRPC server, build/vet clean.

---

## Context

`GET /admin/api/stats` has no TDD-specified RPC — this is a scope
addition SOL-001 proposed rather than skip a documented route entirely.
Cheapest useful implementation: 3 counts.

## Changes to make

```go
// internal/usecase/get_admin_stats.go
func (uc *AdminUseCase) GetAdminStats(ctx context.Context) (*domain.AdminStats, error) {
    totalUsers, err := uc.userRepo.Count(ctx)
    if err != nil {
        return nil, err
    }
    activeSessions, err := uc.sessionRepo.CountActive(ctx)
    if err != nil {
        return nil, err
    }
    totalPolicies, err := uc.policyRepo.CountDistinctIDs(ctx)
    if err != nil {
        return nil, err
    }
    return &domain.AdminStats{TotalUsers: totalUsers, ActiveSessions: activeSessions, TotalPolicies: totalPolicies}, nil
}
```

Add the 3 `Count*` repository methods (simple `SELECT COUNT(*)` /
`SELECT COUNT(*) WHERE expires_at > now()` / `SELECT COUNT(DISTINCT id)`)
next to the other repository methods added in TASK-002/TASK-003.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/auth-service
go build ./... && go vet ./...
```

# TASK-PRF-03-02: Add `DevServerHealthChecker`, `AuditPublisher`, `MemberNotifier`, `ProfileResolver` ports and `ListForMember`

**From Solution:** SOL-PRF-03
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/ports.go`
**Depends on:** TASK-PRF-03-01
**Status:** `[x]` DONE — DevServerHealthChecker/AuditPublisher/MemberNotifier/ProfileResolver ports + ListForMember added to ports.go; fakeProjectRepository.ListForMember added; go vet clean

---

## Context

`CreateProject`/`RebindDevServer`/`ListProjects` (TASK-PRF-03-04/05/06) need
four new outbound ports this task defines, plus a membership-scoped list
method on the existing `ProjectRepository`. Defining the ports first (with no
implementation yet) lets the usecase-edit tasks compile against fakes
independently of the adapter tasks.

## Changes to make

In `backend-go/services/project-service/internal/usecase/ports.go`, add:

```go
// DevServerHealthChecker is the outbound port toward infra-fleet-service's
// GetFleetHealth RPC — CreateProject/RebindDevServer's online/health guard.
// Genuinely new: infra-fleet-service.md §1 already documents fleet health
// monitoring, but no caller in this service used it before this task.
type DevServerHealthChecker interface {
	// IsReachable fails closed on error — a health-check outage must never
	// silently bind/rebind to a server that might be down.
	IsReachable(ctx context.Context, tenantID, devServerID string) (bool, error)
}

// AuditPublisher is the outbound port RebindDevServer calls after a
// successful rebind to emit a security-relevant audit event — outbox
// pattern (05-data-architecture.md), not a synchronous call to another
// service. A nil AuditPublisher is valid — callers must nil-check, same
// convention as tenant-service's CacheInvalidationPublisher.
type AuditPublisher interface {
	PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error
}

// MemberNotifier is the outbound port RebindDevServer calls after a
// successful rebind to notify every project member — best-effort, same
// outbox posture as AuditPublisher. A nil MemberNotifier is valid.
type MemberNotifier interface {
	NotifyDevServerChanged(ctx context.Context, tenantID string, userIDs []string, projectID, oldDevServerID, newDevServerID string) error
}

// ProfileResolver is the outbound port ListProjects uses to resolve the
// caller's ResolvedProfile for the fleet.allowedServerTags visibility
// filter — a NEW outbound edge from project-service to tenant-service
// (tenant-service.md §3/§7 already documents GetResolvedProfile as callable
// by any service, just not exercised by project-service before this task).
// DevServerTags resolves a dev server's tags via infra-fleet-service.ListDevServers.
type ProfileResolver interface {
	GetResolvedProfile(ctx context.Context, tenantID, userID string) (ResolvedProfileView, error)
	DevServerTags(ctx context.Context, tenantID, devServerID string) ([]string, error)
}

// ResolvedProfileView is the subset of tenant-service's ResolvedProfile this
// service actually reads — decoded from GetResolvedProfileResponse's
// resolved_settings_json by the adapter, not the raw JSON map, so usecase/
// code never touches encoding/json directly.
type ResolvedProfileView struct {
	allowedServerTags []string
	hasRestriction    bool
}

// NewResolvedProfileView constructs a ResolvedProfileView — called by
// internal/adapter/infrafleetclient.ProfileResolver's implementation (or
// wherever the JSON is decoded) after reading fleet.allowedServerTags out of
// the decoded resolved_settings_json. hasRestriction distinguishes "key
// absent" (false, unrestricted) from "key present, possibly empty" (true) —
// see domain/profile_resolution.go's mergeAllowedServerTags doc comment
// (tenant-service, SOL-PRF-02) for why this distinction must survive.
func NewResolvedProfileView(tags []string, hasRestriction bool) ResolvedProfileView {
	return ResolvedProfileView{allowedServerTags: tags, hasRestriction: hasRestriction}
}

// AllowedServerTags returns the tag allowlist and whether one is defined at
// all (false = unrestricted, filter nothing).
func (v ResolvedProfileView) AllowedServerTags() ([]string, bool) {
	return v.allowedServerTags, v.hasRestriction
}
```

Extend `ProjectRepository` with a membership-scoped list method (`List`
itself is kept — still potentially used by admin/cross-project tooling):

```go
type ProjectRepository interface {
	// ... existing methods unchanged ...

	// ListForMember returns only projects userID is a member of, within
	// tenantID — unlike List, which returns every tenant project regardless
	// of caller membership (a pre-existing gap this closes; ListProjects's
	// visibility filter is meaningless layered over an unscoped list). NEW.
	ListForMember(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.Project, string, error)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go vet ./services/project-service/internal/usecase/...
```

Full `go build`/`go test` for the package lands once TASK-PRF-03-03's
adapters and TASK-PRF-03-04/05/06's usecase edits satisfy these interfaces
and `ListForMember` gets a postgres implementation.

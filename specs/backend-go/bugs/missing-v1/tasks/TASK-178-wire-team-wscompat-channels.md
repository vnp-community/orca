# TASK-178: Wire all 5 `team.*` `wscompat` channels in `api-gateway`

**From Solution:** SOL-028
**Priority:** P1
**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels.go` (new `channels_team.go` recommended), `cmd/server/main.go`
**Depends on:** TASK-177
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

`team.create`/`team.addMember`/`team.listMembers` are already backed by
real RPCs (`CreateTeam`/`AddTeamMember`/`ListTeamMembers`) but have no
`wscompat` wrapper yet. `team.list`/`team.removeMember` need TASK-177's new
`ListTeams`/`RemoveTeamMember` RPCs. This task wires all 5 in one pass.
`api-gateway`'s `cmd/server/main.go` already dials `tenant-service`
(`tenantClient`, used by REST routes) — `RegisterRealChannels` just needs
it added as a new parameter.

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_team.go`

```go
// ── team.* (tenant-service) ──────────────────────────────────────────────
//
// Every handler below calls gatewaygrpc.AttachIdentity before invoking the
// client: tenant-service binds tenant_id from gRPC metadata for every
// mutating/scoped call per tenant-service.md's "every request carries
// tenant_id explicitly... never inferred from a nested resource ID" rule
// (§3) — same posture devServer.*/fleet.* already use in channels.go, for
// the same reason.
package wscompat

import (
	"context"
	"encoding/json"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerTeamChannels(r *Registry, client tenantv1.TenantServiceClient) {
	r.Register("team.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Name         string `json:"name"`
			SettingsJSON string `json:"settingsJson"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.CreateTeam(ctx, &tenantv1.CreateTeamRequest{
			CompanyId: id.TenantID, Name: in.Name, SettingsJson: in.SettingsJSON,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetTeam(), nil
	})

	r.Register("team.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ListTeams(ctx, &tenantv1.ListTeamsRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetTeams(), nil
	})

	r.Register("team.addMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type addMemberArgs struct {
			TeamID string `json:"teamId"`
			UserID string `json:"userId"`
			// Role has nowhere to go — AddTeamMemberRequest carries only
			// priority, role defaults to 'member' server-side (README
			// "Known gaps", cited by BUG-028). Decoded and silently dropped
			// here rather than erroring, matching this file's existing
			// best-effort convention (channels.go:6-14).
			Role     string `json:"role"`
			Priority int32  `json:"priority"`
		}
		in, err := decodeArg[addMemberArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := client.AddTeamMember(ctx, &tenantv1.AddTeamMemberRequest{
			TeamId: in.TeamID, UserId: in.UserID, Priority: in.Priority,
		}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("team.removeMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type removeMemberArgs struct {
			TeamID string `json:"teamId"`
			UserID string `json:"userId"`
		}
		in, err := decodeArg[removeMemberArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := client.RemoveTeamMember(ctx, &tenantv1.RemoveTeamMemberRequest{
			TeamId: in.TeamID, UserId: in.UserID,
		}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("team.listMembers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listMembersArgs struct {
			TeamID string `json:"teamId"`
		}
		in, err := decodeArg[listMembersArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ListTeamMembers(ctx, &tenantv1.ListTeamMembersRequest{TeamId: in.TeamID})
		if err != nil {
			return nil, err
		}
		return resp.GetMembers(), nil
	})
}
```

### `channels.go`: grow `RegisterRealChannels`

Add `tenantClient tenantv1.TenantServiceClient` as a new parameter and add
the `registerTeamChannels(r, tenantClient)` call:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
	tenantClient tenantv1.TenantServiceClient,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerTeamChannels(r, tenantClient)
}
```

Add the `tenantv1` import to `channels.go`'s import block (it is not
imported there today — `cmd/server/main.go` already imports it for
`tenantClient`, but `channels.go` itself does not yet):

```go
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
```

### `cmd/server/main.go`: pass `tenantClient` through

`tenantClient` is already dialed (`tenantConn`/`tenantClient` block, used
by `httpgateway.NewRouter`'s `Deps.TenantClient`). Find the existing call:

```go
	wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

Replace with:

```go
	wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter, tenantClient)
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

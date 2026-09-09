# TASK-WF-002-01: `TargetSpec` parsing + `ServerResolver`

**From Solution:** BE-SOL-002
**Priority:** P1
**Service:** `workflow-service` (calls out to `project-service` and `infra-fleet-service`; see the infra-fleet-service note below, which may require a small addition there too)
**File:** `backend-go/services/workflow-service/internal/domain/target_spec.go` (new), `backend-go/services/workflow-service/internal/usecase/resolve_target.go` (new), `backend-go/services/workflow-service/internal/usecase/ports.go` (client port additions), `backend-go/services/workflow-service/cmd/server/main.go` (wire `project-service`/`infra-fleet-service` clients)
**Depends on:** TASK-WF-001-01 (`agentExecParams`'s widened shape, so this task's callers have somewhere to put a resolved `connectionID`)
**Status:** `[ ]` TODO

---

## ⚠ Verify before implementing — `infra-fleet-service` gap, re-confirmed live

BE-SOL-002 already flags this, and it is still true as of this writing:
`infra-fleet-service`'s `infrafleet.proto` has **no `PickByTag` RPC** —
confirmed directly (`grep -n "rpc " proto/orca/infrafleet/v1/infrafleet.proto`
returns 50+ RPCs, none named `PickByTag` or anything selection-shaped).

Also re-confirmed: no "pick one server from a group" usecase already
exists under a different name to reuse. `infra-fleet-service`'s
`internal/usecase/` directory has `assign_dev_server_group.go`,
`create_dev_server_group.go`, `list_dev_server_groups.go`,
`grant_dev_server_group_access.go`, `revoke_dev_server_group_access.go`,
and `list_dev_server_group_grants.go` — every one of these is group
**management** (create/assign/list/grant/revoke), not selection. There is
no load-balancing or "give me one live server matching this tag/group"
primitive anywhere in that service today.

**This means implementing `TargetKindFleetTag` end-to-end requires a
genuinely new RPC on `infra-fleet-service`'s side** — this is not
optional scope creep, it's a hard blocker for that one branch of
`ServerResolver.Resolve`. Before writing the `fleet:tag:` case:

1. Confirm with whoever owns `infra-fleet-service` whether `PickByTag` (or
   an equivalently-named RPC) is being added concurrently — check that
   service's own CR/task backlog first to avoid duplicating the proto
   change.
2. If not, this task must add it there: a minimal `PickByTag(tag string)
   (connectionID string, err error)`-shaped RPC, with the actual
   load-balancing algorithm (round-robin, least-loaded, etc.) left as an
   implementation detail — BE-SOL-002 explicitly scopes the algorithm
   choice out ("interface only, algorithm choice deferred to whoever
   implements it").
3. `TargetKindProject` and `TargetKindServer` do NOT depend on this gap —
   they can land and be tested independently of whether `PickByTag` exists
   yet. Consider landing those two cases first and stubbing
   `TargetKindFleetTag` to return a clear "not yet supported" error until
   `PickByTag` lands, rather than blocking this whole task on the
   infra-fleet-service change.

## Context

Re-verified directly: `AgentStepConfig`/`ShellStepConfig`/
`NotificationStepConfig` (`internal/domain/step.go:64-87`) each carry a
bare `ConnectionID string \`json:"connectionId"\`` field. That field's own
doc comment (`step.go:58-63`) names the exact gap this task closes:
*"nothing in this scaffold previously identified *which*
infra-fleet-service connection (dev server + worktree binding) an
agent/shell/notification step should target"* — i.e. today `ConnectionID`
is expected to already BE a resolved connection ID, with no parsing or
resolution layer in front of it. Confirmed via direct search: no
`ServerResolver`, `resolveServer`, `TargetSpec`, or `fleet:tag` string
appears anywhere in `workflow-service`, `orchestration-service`, or
`infra-fleet-service` today.

`project-service`'s real RPC surface (`proto/orca/project/v1/project.proto`)
was read directly to ground this design against BE-SOL-002's sketch, and
found one divergence worth flagging:

- `GetProject(GetProjectRequest{id}) returns (GetProjectResponse)` exists
  exactly as BE-SOL-002 assumes.
- **But** `GetProjectResponse` wraps a `Project` message
  (`message GetProjectResponse { Project project = 1; }`,
  `project.proto:190-192`) — `dev_server_id` lives on the nested
  `Project` message (`project.proto:151-162`, field 4), not directly on
  `GetProjectResponse`. BE-SOL-002's sketch (`proj.GetDevServerId()`
  called directly on the RPC response) does not compile as written; the
  real accessor chain is `resp.GetProject().GetDevServerId()`.

## Changes to make

**1. `internal/domain/target_spec.go`** (new):

```go
package domain

import (
	"errors"
	"strings"
)

// TargetKind discriminates the three forms a step's ConnectionID field
// may now hold — see AgentStepConfig's doc comment (step.go) for why the
// field name stays "connectionId" while its semantics widen from "a
// literal, already-resolved connection ID" to "a target spec a
// ServerResolver must resolve first".
type TargetKind int

const (
	TargetKindProject TargetKind = iota // "project:<id>"
	TargetKindServer                    // "server:<id>"
	TargetKindFleetTag                  // "fleet:tag:<tag>"
)

// ErrUnknownTargetKind is returned by ParseTargetSpec for a raw string
// matching none of the three known prefixes.
var ErrUnknownTargetKind = errors.New("domain: unknown target spec kind")

// TargetSpec is a step config's ConnectionID field, parsed into its kind
// plus the remaining identifier/tag.
type TargetSpec struct {
	Kind TargetKind
	ID   string // set for TargetKindProject / TargetKindServer
	Tag  string // set for TargetKindFleetTag
}

// ParseTargetSpec parses "project:<id>" / "server:<id>" / "fleet:tag:<tag>".
// A raw string matching none of these three prefixes is a caller/config
// error, not a silent default — surfaced as ErrUnknownTargetKind.
func ParseTargetSpec(raw string) (TargetSpec, error) {
	switch {
	case strings.HasPrefix(raw, "project:"):
		return TargetSpec{Kind: TargetKindProject, ID: strings.TrimPrefix(raw, "project:")}, nil
	case strings.HasPrefix(raw, "server:"):
		return TargetSpec{Kind: TargetKindServer, ID: strings.TrimPrefix(raw, "server:")}, nil
	case strings.HasPrefix(raw, "fleet:tag:"):
		return TargetSpec{Kind: TargetKindFleetTag, Tag: strings.TrimPrefix(raw, "fleet:tag:")}, nil
	default:
		return TargetSpec{}, ErrUnknownTargetKind
	}
}
```

**2. `internal/usecase/ports.go`** — add two new client ports (thin gRPC
client interfaces, following this file's existing style for
`TemplateRepository`/`ExecutionRepository`):

```go
// ProjectClient resolves a project's bound dev server — used by
// ServerResolver to implement TargetKindProject.
type ProjectClient interface {
	GetProject(ctx context.Context, id string) (devServerID string, err error)
}

// InfraFleetPicker picks a live connection matching a fleet tag — used by
// ServerResolver to implement TargetKindFleetTag. See TASK-WF-002-01's
// Context for why this depends on a not-yet-existing infra-fleet-service
// RPC.
type InfraFleetPicker interface {
	PickByTag(ctx context.Context, tag string) (connectionID string, err error)
}
```

Keeping these as narrow, workflow-service-owned interfaces (rather than
handing `ServerResolver` the raw generated gRPC client types directly)
matches this codebase's existing port-per-dependency pattern and makes
`ServerResolver` trivially testable with fakes — no real network client
needed in unit tests.

**3. `internal/usecase/resolve_target.go`** (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ServerResolver resolves a parsed domain.TargetSpec down to a concrete
// infra-fleet-service connection ID — the missing layer step.go's
// ConnectionID doc comment names. One Resolve call per step dispatch (not
// cached across steps): a fleet:tag: target may deliberately return a
// different connection on each call once PickByTag's load-balancing is
// live.
type ServerResolver struct {
	project    ProjectClient
	infraFleet InfraFleetPicker
}

func NewServerResolver(project ProjectClient, infraFleet InfraFleetPicker) *ServerResolver {
	return &ServerResolver{project: project, infraFleet: infraFleet}
}

func (r *ServerResolver) Resolve(ctx context.Context, spec domain.TargetSpec) (string, error) {
	switch spec.Kind {
	case domain.TargetKindProject:
		return r.project.GetProject(ctx, spec.ID)
	case domain.TargetKindServer:
		return spec.ID, nil
	case domain.TargetKindFleetTag:
		return r.infraFleet.PickByTag(ctx, spec.Tag)
	default:
		return "", domain.ErrUnknownTargetKind
	}
}
```

**4. `ProjectClient`'s real gRPC-backed implementation** (adapter, e.g.
`internal/adapter/projectclient/project_client.go`, new) wraps
`projectv1.ProjectServiceClient.GetProject` and unwraps the nested
`Project` message correctly:

```go
func (c *Client) GetProject(ctx context.Context, id string) (string, error) {
	resp, err := c.client.GetProject(ctx, &projectv1.GetProjectRequest{Id: id})
	if err != nil {
		return "", err
	}
	return resp.GetProject().GetDevServerId(), nil // NOTE: nested under GetProject(), not resp.GetDevServerId() directly — see Context
}
```

**5. `cmd/server/main.go`** — dial `project-service` (new config field,
e.g. `cfg.ProjectServiceAddr`, following the exact `infrafleetclient.Dial`
pattern already used for `infra-fleet-service` at `main.go:81-86`) and
construct `ServerResolver`. `InfraFleetPicker`'s real implementation wraps
the same already-dialed `infraFleetClient` — add its `PickByTag` call once
that RPC exists (see the blocking note above); until then, wire a stub
that returns a clear "fleet tag targeting not yet supported" error.

## Test plan

- `ParseTargetSpec` on all 3 valid prefixes + an unrecognized string
  (`ErrUnknownTargetKind`).
- `ServerResolver.Resolve(TargetKindProject)` against a fake `ProjectClient`
  for a bound vs. unbound project (empty `dev_server_id`).
- `ServerResolver.Resolve(TargetKindServer)` is a pure passthrough — no
  client call at all.
- `ServerResolver.Resolve(TargetKindFleetTag)` against a fake
  `InfraFleetPicker` — once `PickByTag` exists; until then, assert the
  stub's explicit "not supported" error.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/domain/... -run TestParseTargetSpec -v
go test ./services/workflow-service/internal/usecase/... -run TestServerResolver -v
```

Expected: `TargetKindProject`/`TargetKindServer` paths compile and pass
independent of the `infra-fleet-service` `PickByTag` gap;
`TargetKindFleetTag` either passes against a real `PickByTag` (once added)
or against the documented stub error — never silently no-ops.

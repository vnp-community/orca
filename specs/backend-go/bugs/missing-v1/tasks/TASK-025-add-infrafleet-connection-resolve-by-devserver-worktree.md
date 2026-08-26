# TASK-025: Add `dev_server_id`/`worktree_id` alternate lookup keys to `ResolveConnection`

**From Solution:** SOL-005 (shared prerequisite; also required by SOL-006, see TASK-034)
**Priority:** P0 — SOL-005's `TestConnection` (TASK-028) and SOL-006's browser relay channels (TASK-034) both depend on this
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `backend-go/services/infra-fleet-service/internal/usecase/{ports.go,resolve_connection.go}`, `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go`
**Depends on:** none
**Status:** `[x]` DONE — `ResolveConnectionByDevServer`/`ResolveConnectionByWorktree` confirmed present in `infra-fleet-service/internal/usecase/ports.go` + `resolve_connection.go`, backed by matching postgres repository methods; verified build/test clean.

---

## Context

`ResolveConnection` today only resolves `connectionId -> DevServer`. Two
callers need the reverse/alternate direction:

- SOL-005's `TestConnection` (TASK-028) knows a `dev_server_id` (from
  `ProviderAccount.DevServerID`, added in TASK-026) and needs the dev
  server's currently-active `connectionId` to call `Relay` with.
- SOL-006's browser-pane relay channels (TASK-034) know a `worktree_id`
  (every `browser.*` call site passes a `worktree` selector, not a
  `connectionId`) and need the same kind of lookup.

Both are "resolve the current active connectionId for a dev server /
worktree" — the same shape, just a different key. This task adds both as
additive, mutually-exclusive alternate fields on `ResolveConnectionRequest`
in one pass (rather than two separate proto edits to the same message from
two different tasks), and implements the resolution logic for both at once
since it's the same `Execute` branch.

---

## Changes to make

### Step 1 — `infrafleet.proto`: additive fields on `ResolveConnectionRequest`

Current:

```protobuf
message ResolveConnectionRequest {
  string connection_id = 1;
}
```

Replace with:

```protobuf
message ResolveConnectionRequest {
  // Exactly one of connection_id, dev_server_id, worktree_id is set. Empty
  // connection_id with both alternates also empty resolves nothing
  // (ResolveConnectionOutput{Connected: false}), matching today's
  // empty-connectionId short-circuit.
  string connection_id = 1;
  // dev_server_id resolves the dev server's current active connectionId,
  // the reverse of connection_id's lookup direction — used by
  // ai-provider-service's TestConnection (see
  // specs/backend-go/bugs/missing-v1/tasks/TASK-028-implement-aiprovider-test-connection-usecase.md).
  string dev_server_id = 2;
  // worktree_id resolves the connectionId currently bound to a worktree —
  // used by api-gateway's browser.* relay channels (see
  // specs/backend-go/bugs/missing-v1/tasks/TASK-034-add-browser-pane-relay-wscompat-channels.md).
  // Mirrors CreateConnectionRequest's (dev_server_id, repo_path,
  // worktree_id) tuple in reverse.
  string worktree_id = 3;
}
```

### Step 2 — `ports.go`: extend `ConnectionResolver`

Current:

```go
type ConnectionResolver interface {
	// ResolveConnection looks up connectionID within tenantID's scope.
	// connected=false with a nil error means "no dev server owns this
	// connectionId" — the caller's cue to execute locally, not an error
	// condition. conn carries the per-connection metadata (RepoPath,
	// WorktreeID) callers like git-gateway-service's RelayExecutor need
	// alongside devServer — zero-value when connected is false.
	ResolveConnection(ctx context.Context, tenantID, connectionID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)
}
```

Add two new methods to the same interface (existing `ResolveConnection`
method is unchanged — additive only):

```go
type ConnectionResolver interface {
	// ResolveConnection looks up connectionID within tenantID's scope.
	// connected=false with a nil error means "no dev server owns this
	// connectionId" — the caller's cue to execute locally, not an error
	// condition. conn carries the per-connection metadata (RepoPath,
	// WorktreeID) callers like git-gateway-service's RelayExecutor need
	// alongside devServer — zero-value when connected is false.
	ResolveConnection(ctx context.Context, tenantID, connectionID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)

	// ResolveConnectionByDevServer finds the most recently created
	// connection row bound to devServerID within tenantID's scope — the
	// reverse lookup direction from ResolveConnection. Same
	// connected=false/nil-error "nothing bound yet" convention.
	ResolveConnectionByDevServer(ctx context.Context, tenantID, devServerID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)

	// ResolveConnectionByWorktree finds the connection row currently bound
	// to worktreeID within tenantID's scope. Same
	// connected=false/nil-error convention.
	ResolveConnectionByWorktree(ctx context.Context, tenantID, worktreeID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)
}
```

### Step 3 — `resolve_connection.go`: branch on which key is set

Replace `(uc *ResolveConnection) Execute`'s signature and body:

```go
// ResolveConnectionInput mirrors ResolveConnectionRequest 1:1 — exactly one
// of ConnectionID/DevServerID/WorktreeID is expected to be set (see
// infrafleet.proto's ResolveConnectionRequest doc comment).
type ResolveConnectionInput struct {
	ConnectionID string
	DevServerID  string
	WorktreeID   string
}

func (uc *ResolveConnection) Execute(ctx context.Context, in ResolveConnectionInput) (ResolveConnectionOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ResolveConnectionOutput{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	var (
		connected bool
		devServer domain.DevServer
		conn      domain.Connection
	)
	switch {
	case in.DevServerID != "":
		connected, devServer, conn, err = uc.resolver.ResolveConnectionByDevServer(ctx, tenantID, in.DevServerID)
	case in.WorktreeID != "":
		connected, devServer, conn, err = uc.resolver.ResolveConnectionByWorktree(ctx, tenantID, in.WorktreeID)
	case in.ConnectionID == "":
		// No key at all is not an error — it's the caller's own signal
		// that there's nothing to resolve (a connectionless, local-only
		// worktree or session). Short-circuit before the repository
		// round-trip.
		return ResolveConnectionOutput{Connected: false}, nil
	default:
		connected, devServer, conn, err = uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
	}
	if err != nil {
		return ResolveConnectionOutput{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", err)
	}
	return ResolveConnectionOutput{Connected: connected, DevServer: devServer, RepoPath: conn.RepoPath, WorktreeID: conn.WorktreeID}, nil
}
```

Note this changes `Execute`'s signature from `(ctx, connectionID string)` to
`(ctx, in ResolveConnectionInput)` — update its one call site in
`internal/adapter/grpc/server.go`'s `ResolveConnection` gRPC handler to
build `ResolveConnectionInput{ConnectionID: req.GetConnectionId(), DevServerID: req.GetDevServerId(), WorktreeID: req.GetWorktreeId()}` instead of passing
`req.GetConnectionId()` directly.

### Step 4 — `postgres/repository.go`: implement the two new resolver methods

Add alongside the existing `ResolveConnection` method, reusing the same
join query shape with a different `WHERE` clause and `ORDER BY created_at
DESC LIMIT 1` (a dev server/worktree can have had multiple connection rows
over time — resolve the most recent):

```go
// ResolveConnectionByDevServer is ResolveConnection's reverse-lookup
// counterpart — see usecase.ConnectionResolver's doc comment (TASK-025).
func (r *Repository) ResolveConnectionByDevServer(ctx context.Context, tenantID, devServerID string) (bool, domain.DevServer, domain.Connection, error) {
	const q = `
		SELECT c.id, c.dev_server_id, c.repo_path, c.worktree_id,
		       d.id, d.tenant_id, d.host, d.mode, d.ssh_target_id
		FROM infra.connections c
		JOIN infra.dev_servers d ON d.id = c.dev_server_id
		WHERE c.tenant_id = $1 AND c.dev_server_id = $2
		ORDER BY c.created_at DESC
		LIMIT 1`
	return r.scanConnectionRow(ctx, q, tenantID, devServerID)
}

// ResolveConnectionByWorktree is ResolveConnection's worktree-keyed
// counterpart — see usecase.ConnectionResolver's doc comment (TASK-025).
func (r *Repository) ResolveConnectionByWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.DevServer, domain.Connection, error) {
	const q = `
		SELECT c.id, c.dev_server_id, c.repo_path, c.worktree_id,
		       d.id, d.tenant_id, d.host, d.mode, d.ssh_target_id
		FROM infra.connections c
		JOIN infra.dev_servers d ON d.id = c.dev_server_id
		WHERE c.tenant_id = $1 AND c.worktree_id = $2
		ORDER BY c.created_at DESC
		LIMIT 1`
	return r.scanConnectionRow(ctx, q, tenantID, worktreeID)
}
```

`scanConnectionRow` is a small new private helper factoring out the
row-scan + "no rows means connected=false, not an error" handling that
`ResolveConnection`'s existing body already does inline — extract that
existing logic into this helper (same `pgx.ErrNoRows` → `(false,
domain.DevServer{}, domain.Connection{}, nil)` branch, same field scan
order) so all three methods share it instead of duplicating the scan.
Read `ResolveConnection`'s current body in `repository.go:177` before
extracting, to match its exact column order and error handling.

---

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
go build ./services/infra-fleet-service/...
go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... ./services/infra-fleet-service/internal/adapter/postgres/... -run ResolveConnection -v
```

Expected: clean build; existing `resolve_connection_test.go` cases for
plain `connectionId` resolution still pass unmodified (the `ConnectionID`-set
branch is behaviorally identical to before).

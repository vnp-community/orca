# SOL-FLEET-01: YAML fleet-inventory import — `orca fleet import/list/status`

**Resolves:** [BUG-FLEET-01](../BUG-FLEET-01-fleet-inventory-not-implemented.md)
**Service:** `infra-fleet-service` (domain/usecase/proto/postgres extensions) +
`api-gateway` (new REST routes) + a new `fleetctl` CLI binary
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go` (add `Project`, `Tags`)
- `backend-go/services/infra-fleet-service/migrations/0006_ssh_target_project_tags.up.sql` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend `SshTargetRepository` with `Upsert`)
- `backend-go/services/infra-fleet-service/internal/usecase/import_fleet_inventory.go` (new)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/ssh_target_store.go` (add `Upsert`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`SshTarget.project/tags`, new `ImportFleetInventory` RPC)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (wire the new RPC)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go` (new `/v1/infra/fleet/*` routes)
- `backend-go/services/infra-fleet-service/cmd/fleetctl/main.go` (new CLI binary)
- `backend-go/services/infra-fleet-service/cmd/fleetctl/fleetyaml/parse.go` (new — YAML parse + validate)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`infra-fleet-service.md` §5 already models `ssh_targets` as this service's own
table (carried forward from ADR-021, `infra-fleet-service.md:204-216`), and
§4's `SshTarget` domain object description doesn't mention `project`/`tags`
grouping — this is a genuine, flagged extension to the domain, the same
posture SOL-009 took for `FileIO` on `git-gateway-service`'s proto (SOL-009
"Where this belongs" section, `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md:56-65`).
It belongs on `SshTarget` rather than a new `fleet_projects`/`fleet_tags`
service because `infra-fleet-service` already owns the exact row this
grouping annotates, and per design principle 4 in
`02-microservices-decomposition.md:33-36`, a metadata-only concept with no
independent business rules folds into the service that owns the data it
decorates rather than becoming its own deployable.

**Where the CLI talks to**: `infra-fleet-service.md` §8 frames this service
as reachable only by the declared dependency graph
(`02-microservices-decomposition.md:110-166`, and the deployment doc's
"`NetworkPolicy` default-deny, explicit allow per the dependency graph",
`10-deployment-infrastructure.md:28-30`) — an operator's laptop is not a
listed caller of `infra-fleet-service`'s internal gRPC port. `api-gateway`
is explicitly "the edge: terminates HTTPS/WSS ... JWT validation, request
routing to internal gRPC services"
(`02-microservices-decomposition.md:67`), and a REST->gRPC proxy for this
exact service already exists at
`backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go:20-31`
(`mountInfraRoutes`, covering `RegisterDevServer`/`ListDevServers`/
`CreateSshTarget`/etc.). `fleetctl` is therefore designed as a thin HTTPS
client of `api-gateway`'s existing authenticated REST surface — extending
`infra_routes.go` with `fleet.*` endpoints — rather than a new direct gRPC
caller of `infra-fleet-service`, avoiding a NetworkPolicy exception and
reusing `api-gateway`'s existing JWT auth wholesale.

**The `identityFile` incompatibility (flagged, not silently dropped)**:
BL-FLEET-01's YAML schema has each server carry `identityFile` — a path to a
raw SSH private key file on the admin's machine
(`docs/logic/fleet/BL-FLEET-01-fleet-inventory.md:31,37,42`). This directly
conflicts with `infra-fleet-service.md` §9's hard security invariant: "No
long-lived SSH private keys on this service's filesystem or database, ever"
(`infra-fleet-service.md:487-488`), enforced today by
`domain.ErrEmptyVaultSSHRole`'s doc comment ("this service never stores raw
key material", `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go:12-16`)
and `NewSshTarget`'s constructor requiring `vaultSSHRole` unconditionally
(`ssh_target.go:44-46`). This solution does **not** honor `identityFile`
literally — it is a genuine backend-go-vs-TS-spec deviation, not an
oversight: the YAML schema's per-server (or `defaults`) field is renamed to
`vaultSshRole` (a pointer into Vault's SSH secrets engine role, matching
`CreateSshTargetRequest.vault_ssh_role`), and `fleetctl` rejects a YAML file
that still carries `identityFile` with an actionable error ("`identityFile`
is not supported against backend-go — provision a Vault SSH role for this
target and set `vaultSshRole` instead, see `infra-fleet-service.md` §9").
This mirrors `06-secrets-vault-architecture.md`'s policy that raw key
material never enters application-tier storage. `port` from the YAML schema
is similarly not representable — `domain.SshTarget`/`CreateSshTargetRequest`
have no port field (`ssh_target.go:25-31`, `infrafleet.proto:211-216`);
non-default ports are out of scope for this pass and `fleetctl` warns (not
errors) on a non-22 `port` value being dropped.

---

## Design — domain / migration

```go
// internal/domain/ssh_target.go (extended)
type SshTarget struct {
    ID           string
    TenantID     string
    Host         string
    UserName     string
    VaultSSHRole string
    Project      string   // "" = ungrouped; matches YAML's servers[].project
    Tags         []string // matches YAML's servers[].tags
}
```

`NewSshTarget` gains `project string, tags []string` params — both optional
(no invariant added), since single-server registration via `ssh.*`/
`CreateSshTarget` predates grouping and must keep working with empty values.

```sql
-- migrations/0006_ssh_target_project_tags.up.sql
ALTER TABLE infra.ssh_targets
  ADD COLUMN project TEXT NOT NULL DEFAULT '',
  ADD COLUMN tags     TEXT[] NOT NULL DEFAULT '{}';

-- Upsert-by-hostname+user (BL-FLEET-01's "INSERT OR UPDATE by hostname+user")
-- needs this uniqueness constraint to exist at all — it does not today.
CREATE UNIQUE INDEX idx_infra_ssh_targets_tenant_host_user
  ON infra.ssh_targets (tenant_id, host, user_name);
```

## Design — usecase / port

```go
// internal/usecase/ports.go (extended)
type SshTargetRepository interface {
    Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
    Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
    List(ctx context.Context, tenantID string) ([]domain.SshTarget, error)
    // Upsert inserts or updates by (tenant_id, host, user_name) — the
    // conflict target the new migration's unique index establishes.
    // updated=true means an existing row's vault_ssh_role/project/tags were
    // overwritten; updated=false means a new row was inserted.
    Upsert(ctx context.Context, target domain.SshTarget) (saved domain.SshTarget, updated bool, err error)
}
```

Postgres implementation (`ssh_target_store.go`) is one
`INSERT ... ON CONFLICT (tenant_id, host, user_name) DO UPDATE SET
vault_ssh_role = EXCLUDED.vault_ssh_role, project = EXCLUDED.project, tags =
EXCLUDED.tags RETURNING id, (xmax != 0) AS updated` — the `xmax != 0` trick
is the standard Postgres idiom for "was this an insert or an update" in one
round trip, avoiding a separate `SELECT ... FOR UPDATE` per row (important
for a bulk-import fan-in against Postgres, per
`08-inter-service-communication.md`'s general per-call efficiency posture).

```go
// internal/usecase/import_fleet_inventory.go
type FleetServerInput struct {
    Host, UserName, VaultSSHRole, Project string
    Tags                                  []string
}

type ImportFleetInventoryInput struct {
    Servers []FleetServerInput
    DryRun  bool
}

type ImportFleetInventoryResult struct {
    Imported, Updated, Skipped int
    Errors []ImportFleetInventoryError // {Host, UserName string; Reason string}
}

type ImportFleetInventory struct {
    repo SshTargetRepository
}

func (uc *ImportFleetInventory) Execute(ctx context.Context, in ImportFleetInventoryInput) (ImportFleetInventoryResult, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return ImportFleetInventoryResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
    }
    var result ImportFleetInventoryResult
    for _, s := range in.Servers {
        target, err := domain.NewSshTarget(uuid.NewString(), tenantID, s.Host, s.UserName, s.VaultSSHRole, s.Project, s.Tags)
        if err != nil {
            result.Skipped++
            result.Errors = append(result.Errors, ImportFleetInventoryError{Host: s.Host, UserName: s.UserName, Reason: err.Error()})
            continue
        }
        if in.DryRun {
            // Dry-run still classifies imported-vs-updated by probing
            // existence, without persisting — GetByHostUser is a narrow
            // additional SshTargetRepository method for this preview path
            // only; it does not commit anything.
            _, found, _ := uc.repo.GetByHostUser(ctx, tenantID, s.Host, s.UserName)
            if found {
                result.Updated++
            } else {
                result.Imported++
            }
            continue
        }
        saved, updated, err := uc.repo.Upsert(ctx, target)
        _ = saved
        if err != nil {
            result.Skipped++
            result.Errors = append(result.Errors, ImportFleetInventoryError{Host: s.Host, UserName: s.UserName, Reason: err.Error()})
            continue
        }
        if updated {
            result.Updated++
        } else {
            result.Imported++
        }
    }
    return result, nil
}
```

Per-record error handling (skip-and-continue rather than fail-fast) matches
BL-FLEET-01's `{ imported, updated, skipped, errors }` shape
(`BL-FLEET-01-fleet-inventory.md:56`) — one malformed row must not abort an
otherwise-valid batch, the same posture BL-FLEET-02 later requires for
per-server provisioning errors (see SOL-FLEET-02).

## Design — proto additions

```protobuf
message SshTarget {
  string id = 1;
  string tenant_id = 2;
  string host = 3;
  string user = 4;
  string vault_ssh_role = 5;
  string project = 6;  // new
  repeated string tags = 7;  // new
}

message FleetServerInput {
  string host = 1;
  string user = 2;
  string vault_ssh_role = 3;
  string project = 4;
  repeated string tags = 5;
}

message ImportFleetInventoryRequest {
  repeated FleetServerInput servers = 1;
  bool dry_run = 2;
}
message ImportFleetInventoryError { string host = 1; string user = 2; string reason = 3; }
message ImportFleetInventoryResponse {
  int32 imported = 1;
  int32 updated = 2;
  int32 skipped = 3;
  repeated ImportFleetInventoryError errors = 4;
}

service InfraFleetService {
  // ... existing RPCs unchanged ...
  rpc ImportFleetInventory(ImportFleetInventoryRequest) returns (ImportFleetInventoryResponse);
}
```

`ListSshTargetsResponse` needs no shape change — `project`/`tags` ride along
on the now-extended `SshTarget` message; `orca fleet list --project X`
filters client-side in `fleetctl`, following the exact precedent
`fleet.health.checkAll` already set for "the RPC returns everything, the
caller filters" (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:523-526`,
"GetFleetHealth returns health for ALL of the tenant's dev servers, not
filtered ... filter client-side here").

## Design — wiring (api-gateway REST + fleetctl CLI)

`infra_routes.go`'s `mountInfraRoutes` gains one route, following
`handleCreateSshTarget`'s exact shape (`infra_routes.go:154-177`):

```go
sub.Post("/fleet/import", handleImportFleetInventory(client))
// GET /v1/infra/ssh-targets already exists via ListSshTargets — no new
// route needed for `orca fleet list`; fleetctl calls it and filters by
// ?project= client-side, same posture as fleet.health.checkAll above.
// `orca fleet status` reuses the existing GET /v1/infra/health route
// verbatim (see SOL-FLEET-03 for making that endpoint return real data).
```

```go
func handleImportFleetInventory(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        var body importFleetInventoryRequestBody // Servers []FleetServerInput; DryRun bool
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
            return
        }
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
        resp, err := client.ImportFleetInventory(ctx, &infrafleetv1.ImportFleetInventoryRequest{
            Servers: toProtoFleetServers(body.Servers), DryRun: body.DryRun,
        })
        if err != nil {
            writeGRPCError(w, err)
            return
        }
        writeJSON(w, http.StatusOK, resp)
    }
}
```

`fleetctl` (new binary, `backend-go/services/infra-fleet-service/cmd/fleetctl/main.go`)
uses stdlib `flag` for subcommand dispatch (`fleetctl import|list|status`,
mirroring Go's own `go` tool convention) rather than adding a `cobra`
dependency — `infra-fleet-service`'s `go.mod` already carries
`gopkg.in/yaml.v3` as an indirect dependency (promotable to direct, no new
third-party parser needed) and no backend-go service currently depends on a
CLI framework (confirmed via a graph search across `orca` for
`cobra`/CLI-subcommand precedent turning up only unrelated TS files), so
`flag`-based dispatch keeps this the first CLI's footprint minimal:

- `fleetctl import --file orca-fleet.yaml [--dry-run] --api-base <url> --token <bearer>`:
  reads the YAML locally (the operator's laptop is the only place the file
  can live), parses+validates client-side (hostname format, `port` 1-65535,
  `project` must be declared in `projects[]` if set, rejects lingering
  `identityFile` per the rationale above — `fleetyaml/parse.go`), then POSTs
  the validated `FleetServerInput` list to `/v1/infra/fleet/import`. Prints
  the returned `{imported, updated, skipped, errors}` summary verbatim.
- `fleetctl list [--project X]`: `GET /v1/infra/ssh-targets`, filters by
  `project` client-side, prints a table.
- `fleetctl status`: `GET /v1/infra/health`, prints per-server
  reachable/CPU/RAM/disk (depends on SOL-FLEET-03's writer for non-empty
  output).

Auth: `fleetctl` takes `--token` (a bearer JWT obtained via `auth-service`'s
existing login flow, out of scope for this solution) and forwards it as
`Authorization: Bearer <token>` — identical to any other HTTPS client of
`api-gateway`, no new auth mechanism.

---

## Test plan

- `domain/ssh_target_test.go` — `NewSshTarget` accepts empty
  `project`/`tags` (backward compat) and a populated pair; existing
  `ErrEmptyVaultSSHRole` invariant unchanged.
- `usecase/import_fleet_inventory_test.go` — fake `SshTargetRepository`:
  all-new batch → `imported=N, updated=0`; a batch with one pre-existing
  `(tenant,host,user)` → that row counted `updated`; one row with an empty
  `VaultSSHRole` → `skipped` + populated `Errors`, and the valid rows in the
  same batch still commit (partial-success, not fail-fast); `DryRun=true`
  → `repo.Upsert` never called (assert on the fake), only `GetByHostUser`.
- `adapter/postgres/ssh_target_store_test.go` (integration, testcontainers) —
  `Upsert` twice with the same `(host,user)` and a changed `vault_ssh_role`:
  second call returns `updated=true`, row count stays 1, unique index
  enforces the conflict target.
- `httpgateway/infra_routes_test.go` — `POST /v1/infra/fleet/import` with a
  fake gRPC client asserts request/response marshaling and identity
  attachment, mirroring `TestHandleCreateSshTarget`-shaped tests already in
  `infra_routes_test.go`.
- `cmd/fleetctl/fleetyaml/parse_test.go` — the full YAML sample from
  `BL-FLEET-01-fleet-inventory.md:16-44` parses into the expected
  `FleetServerInput` list; a server referencing an undeclared `project`
  fails validation; a server carrying `identityFile` fails validation with
  the actionable Vault-role message; a `port` outside 1-65535 fails.
- Contract test: round-trip `orca-fleet.yaml` → `fleetctl import --dry-run`
  → `{imported: 3, updated: 0, skipped: 0, errors: []}` against the sample
  YAML, then a real (non-dry-run) import, then a second import of the same
  file → `{imported: 0, updated: 3, skipped: 0}` (idempotency check, shared
  concern with SOL-FLEET-02's "idempotent re-runs" requirement).

## References

- `docs/logic/fleet/BL-FLEET-01-fleet-inventory.md` — YAML schema, import
  flow, CLI commands
- `specs/backend-go/bugs/logic-v1/BUG-FLEET-01-fleet-inventory-not-implemented.md`
- `specs/backend-go/tdd/services/infra-fleet-service.md:174-292` (§5 data
  model, `ssh_targets` DDL), `:487-488` (§9 no-raw-key invariant)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:33-36`
  (design principle 4, fold-in rationale), `:67` (`api-gateway`'s edge role),
  `:110-166` (dependency graph — no direct external caller of
  `infra-fleet-service`)
- `specs/backend-go/tdd/architecture/10-deployment-infrastructure.md:28-30`
  (`NetworkPolicy` default-deny per dependency graph)
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go:12-16,25-31,44-46`
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:42-51`
  (`SshTargetRepository`, extended here)
- `backend-go/services/infra-fleet-service/migrations/0001_init.up.sql:37-50`
  (`infra.ssh_targets` DDL this solution alters)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:260-275`
  (`SshTarget`/`ListSshTargets*` messages extended here)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go:1-31,145-177`
  (`mountInfraRoutes`, `handleCreateSshTarget` pattern this solution follows)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:523-526`
  (client-side filtering precedent for `orca fleet list --project`)
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md:56-65`
  (fold-in-vs-new-service reasoning this solution mirrors)

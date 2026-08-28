# TASK-FLEET-01-08: `fleetctl` CLI binary (`import`/`list`/`status`)

**From Solution:** SOL-FLEET-01
**Priority:** P2
**Service:** `infra-fleet-service` (new CLI binary)
**File:** `backend-go/services/infra-fleet-service/cmd/fleetctl/main.go` (new), `backend-go/services/infra-fleet-service/cmd/fleetctl/fleetyaml/parse.go` (new)
**Depends on:** TASK-FLEET-01-07
**Status:** [x] DONE — implemented fleetyaml.Parse (hostname/IP validation, port drop-with-warning, project-declared check, identityFile rejection, defaults.vaultSshRole fallback) + fleetctl main.go (import/list/status against api-gateway REST, flag-based dispatch). Note: TASK-FLEET-01-07 (api-gateway REST routes this CLI targets) was out of scope for this batch, so the CLI compiles/builds and is fully unit-tested for parsing, but end-to-end HTTP calls are untested against a live api-gateway. `go build ./cmd/fleetctl/...` and `go test ./cmd/fleetctl/fleetyaml/...` both pass; promoted yaml.v3 to direct via `go mod tidy`.

---

## Context

`fleetctl` is a thin HTTPS client of `api-gateway`'s authenticated REST
surface — not a direct gRPC caller of `infra-fleet-service` — reusing
`api-gateway`'s existing JWT auth wholesale and avoiding a `NetworkPolicy`
exception. Uses stdlib `flag` for subcommand dispatch (no new CLI-framework
dependency).

## Changes to make

`cmd/fleetctl/fleetyaml/parse.go`: parse+validate the YAML sample schema
from `docs/logic/fleet/BL-FLEET-01-fleet-inventory.md:16-44` into
`[]usecase.FleetServerInput`-shaped structs. Validation rules:
- hostname format required
- `port` must be 1-65535 if present, but is **dropped with a warning**
  (not an error) since `domain.SshTarget`/`CreateSshTargetRequest` have no
  port field
- `project` must be declared in the YAML's top-level `projects[]` if a
  server references one
- a server (or `defaults`) block carrying `identityFile` fails validation
  with: `"identityFile is not supported against backend-go — provision a
  Vault SSH role for this target and set vaultSshRole instead, see
  infra-fleet-service.md §9"`
- the schema's per-server/`defaults` key is `vaultSshRole` (not
  `identityFile`), mapping directly to `FleetServerInput.VaultSSHRole`

`cmd/fleetctl/main.go`: `flag`-based subcommand dispatch, mirroring the `go`
tool's own convention:
- `fleetctl import --file orca-fleet.yaml [--dry-run] --api-base <url> --token <bearer>`:
  parses the YAML locally, POSTs the validated server list to
  `POST /v1/infra/fleet/import` with `Authorization: Bearer <token>`, prints
  the returned `{imported, updated, skipped, errors}` summary verbatim.
- `fleetctl list [--project X] --api-base <url> --token <bearer>`:
  `GET /v1/infra/ssh-targets`, filters by `project` client-side, prints a
  table.
- `fleetctl status --api-base <url> --token <bearer>`: `GET /v1/infra/health`,
  prints per-server reachable/CPU/RAM/disk (non-empty once SOL-FLEET-03's
  writer lands).

Promote `gopkg.in/yaml.v3` from indirect to direct in
`backend-go/services/infra-fleet-service/go.mod` (already present
transitively — no new third-party dependency).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/cmd/fleetctl/...
go test ./services/infra-fleet-service/cmd/fleetctl/fleetyaml/... -v
```

Expected: the full YAML sample from `BL-FLEET-01-fleet-inventory.md:16-44`
parses into the expected `FleetServerInput` list; a server referencing an
undeclared `project` fails validation; a server carrying `identityFile`
fails with the actionable Vault-role message; a `port` outside 1-65535
fails.

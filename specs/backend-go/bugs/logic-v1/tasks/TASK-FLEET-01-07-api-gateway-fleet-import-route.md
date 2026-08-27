# TASK-FLEET-01-07: `POST /v1/infra/fleet/import` REST route on api-gateway

**From Solution:** SOL-FLEET-01
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go`
**Depends on:** TASK-FLEET-01-01, TASK-FLEET-01-06
**Status:** `[ ]` TODO

---

## Context

`fleetctl import` (TASK-FLEET-01-08) needs an authenticated HTTPS entry
point into `infra-fleet-service`'s new RPC — `api-gateway` is the only
listed caller of that internal gRPC service per the dependency graph, so
this follows `handleCreateSshTarget`'s exact REST-proxy shape already in
this file.

## Changes to make

In `mountInfraRoutes`:

```go
sub.Post("/fleet/import", handleImportFleetInventory(client))
```

New handler:

```go
type importFleetInventoryRequestBody struct {
    Servers []fleetServerInputBody `json:"servers"`
    DryRun  bool                   `json:"dryRun"`
}
type fleetServerInputBody struct {
    Host, User, VaultSSHRole, Project string
    Tags                              []string
}

func handleImportFleetInventory(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        var body importFleetInventoryRequestBody
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
            return
        }
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
        servers := make([]*infrafleetv1.FleetServerInput, 0, len(body.Servers))
        for _, s := range body.Servers {
            servers = append(servers, &infrafleetv1.FleetServerInput{
                Host: s.Host, User: s.User, VaultSshRole: s.VaultSSHRole, Project: s.Project, Tags: s.Tags,
            })
        }
        resp, err := client.ImportFleetInventory(ctx, &infrafleetv1.ImportFleetInventoryRequest{
            Servers: servers, DryRun: body.DryRun,
        })
        if err != nil {
            writeGRPCError(w, err)
            return
        }
        writeJSON(w, http.StatusOK, resp)
    }
}
```

Note: `GET /v1/infra/ssh-targets` already exists via `ListSshTargets` — no
new route is needed for `orca fleet list`; `fleetctl` filters by
`?project=` client-side. `orca fleet status` reuses the existing
`GET /v1/infra/health` route verbatim (made non-empty by SOL-FLEET-03).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestHandleImportFleetInventory -v
```

Expected: `POST /v1/infra/fleet/import` request/response marshaling and
identity attachment assert correctly against a fake gRPC client, mirroring
`TestHandleCreateSshTarget`-shaped tests already in `infra_routes_test.go`.

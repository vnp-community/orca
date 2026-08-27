# TASK-FLEET-02-07: `POST /v1/infra/fleet/provision` route + `fleetctl provision`

**From Solution:** SOL-FLEET-02
**Priority:** P2
**Service:** `api-gateway` + `infra-fleet-service` (`fleetctl`)
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go`, `backend-go/services/infra-fleet-service/cmd/fleetctl/main.go`
**Depends on:** TASK-FLEET-02-06, TASK-FLEET-01-08 (fleetctl binary must already exist)
**Status:** `[ ]` TODO

---

## Context

Follows `handleImportFleetInventory`'s exact shape (SOL-FLEET-01) — a thin
REST proxy plus a `fleetctl` subcommand that POSTs to it.

## Changes to make

`infra_routes.go`, in `mountInfraRoutes`:

```go
sub.Post("/fleet/provision", handleBulkProvisionFleet(client))
```

```go
type bulkProvisionFleetRequestBody struct {
    Project     string `json:"project"`
    Concurrency int32  `json:"concurrency"`
}

func handleBulkProvisionFleet(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        var body bulkProvisionFleetRequestBody
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
            return
        }
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
        resp, err := client.BulkProvisionFleet(ctx, &infrafleetv1.BulkProvisionFleetRequest{
            Project: body.Project, Concurrency: body.Concurrency,
        })
        if err != nil {
            writeGRPCError(w, err)
            return
        }
        writeJSON(w, http.StatusOK, resp)
    }
}
```

`fleetctl` gains `fleetctl provision [--project X] [--concurrency 5]
--api-base <url> --token <bearer>`, POSTing to `/v1/infra/fleet/provision`
and printing the returned `{success, failed, skipped, outcomes}` summary.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... ./services/infra-fleet-service/cmd/fleetctl/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestHandleBulkProvisionFleet -v
```

Expected: `POST /v1/infra/fleet/provision` request/response marshaling
asserts correctly against a fake gRPC client.

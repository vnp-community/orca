# TASK-FLEET-02-06: `BulkProvisionFleet` proto RPC + gRPC wiring

**From Solution:** SOL-FLEET-02
**Priority:** P1
**Service:** `infra-fleet-service` (proto + grpc adapter)
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-FLEET-02-05
**Status:** [x] DONE — added BulkProvisionFleetRequest/Response, ProvisionOutcome messages, BulkProvisionFleet RPC, DevServer.status=6 (next free slot, not 8 as the task's stale draft assumed). Wired Server.BulkProvisionFleet handler + main.go bootstrap (with unavailableBulkProvisioner graceful-degrade when Vault/relay-ssh isn't configured, matching the existing ErrConnectionModeNotImplemented convention). `buf breaking` couldn't run against local `main` (this worktree's local main ref predates backend-go entirely — see the merge in the group commit); additive-only verified by construction (only new fields at next-free numbers, new messages, new RPC). server_test.go covers marshaling + error->gRPC-status. Full suite + `-race` pass.

---

## Context

Unary (not streaming) — bulk provisioning is N-servers-in-parallel, not one
server's steps in sequence, and BL-FLEET-02's own contract is a single
terminal summary object.

## Changes to make

`infrafleet.proto`:

```protobuf
message BulkProvisionFleetRequest {
  string project = 1; // "" = all
  int32 concurrency = 2; // 0 = default 5
}
message ProvisionOutcome { string dev_server_id = 1; string host = 2; string status = 3; string error = 4; }
message BulkProvisionFleetResponse {
  int32 success = 1; int32 failed = 2; int32 skipped = 3;
  repeated ProvisionOutcome outcomes = 4;
}
```

Add to `InfraFleetService`:

```protobuf
rpc BulkProvisionFleet(BulkProvisionFleetRequest) returns (BulkProvisionFleetResponse);
```

Also add `status` to the existing `DevServer` message (field number = next
free slot):

```protobuf
string status = 8; // pending|healthy|degraded|unhealthy
```

`server.go`:

```go
func (s *Server) BulkProvisionFleet(ctx context.Context, req *infrafleetv1.BulkProvisionFleetRequest) (*infrafleetv1.BulkProvisionFleetResponse, error) {
    result, err := s.bulkProvisionFleet.Execute(ctx, usecase.BulkProvisionFleetInput{
        Project: req.GetProject(), Concurrency: int(req.GetConcurrency()),
    })
    if err != nil {
        return nil, mapAppError(err)
    }
    resp := &infrafleetv1.BulkProvisionFleetResponse{
        Success: int32(result.Success), Failed: int32(result.Failed), Skipped: int32(result.Skipped),
    }
    for _, o := range result.Outcomes {
        resp.Outcomes = append(resp.Outcomes, &infrafleetv1.ProvisionOutcome{
            DevServerId: o.DevServerID, Host: o.Host, Status: o.Status, Error: o.Error,
        })
    }
    return resp, nil
}
```

Wire `s.bulkProvisionFleet *usecase.BulkProvisionFleet` into `Server`'s
constructor and construct it at bootstrap in `cmd/server/main.go`.

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/... ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/grpc/... -run TestBulkProvisionFleet -v
```

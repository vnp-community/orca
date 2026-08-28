# TASK-SSH-01-01: Extend `SshTarget`/`CreateSshTargetRequest` proto with port, known-hosts, jump-host

**From Solution:** SOL-SSH-01
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[x] DONE — added port/known_hosts_fingerprint/jump_host_target_id to CreateSshTargetRequest+SshTarget, buf generate + buf breaking (vs base commit with backend-go) clean`

---

## Context

`infra-fleet-service.md` §4/§5 already specifies `SshTarget` with port,
known-hosts fingerprint, and jump-host chaining, but the current proto only
carries `id, tenant_id, host, user, vault_ssh_role`. This task adds the
missing fields — additive only, no breaking change, so `buf breaking` stays
clean.

## Changes to make

In `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, extend
`CreateSshTargetRequest` (currently at line 211) and `SshTarget` (currently
at line 260):

```protobuf
message CreateSshTargetRequest {
  string tenant_id = 1;
  string host = 2;
  string user = 3;
  string vault_ssh_role = 4; // Vault SSH secrets engine role for cert issuance
  int32 port = 5;                     // 0 = default to 22, mirrors domain.NewSshTarget
  string known_hosts_fingerprint = 6; // optional; "" = unverified (documented gap)
  string jump_host_target_id = 7;     // optional; "" = no jump host
}

message CreateSshTargetResponse {
  string ssh_target_id = 1;
}
```

```protobuf
message SshTarget {
  string id = 1;
  string tenant_id = 2;
  string host = 3;
  string user = 4;
  string vault_ssh_role = 5; // a Vault role pointer, never key material — safe to return (infra-fleet-service.md §9)
  int32 port = 6;
  string known_hosts_fingerprint = 7;
  string jump_host_target_id = 8;
}
```

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/...
```

Expected: clean build, `buf breaking` reports no breaking changes (only additions).

# TASK-AIP-02-01: Add `dev_server_id`/`model_hint`/`account_id`/`scoped_ref` to `ResolveProviderRequest`

**From Solution:** SOL-AIP-02
**Priority:** P0 — correctness bug, `ResolveProvider` can currently hand back the wrong provider's account
**Service:** `ai-provider-service` proto
**File:** `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
**Depends on:** none
**Status:** `[x] DONE — dev_server_id/model_hint/account_id/scoped_ref added to ResolveProviderRequest; go build ./proto/... clean.`

---

## Context

`ResolveProviderRequest` today only carries `tenant_id`/`user_id`/
`project_id` (`aiprovider.proto:73-77`). `ai-provider-service.md` §3
already sketches `dev_server_id` and `model_hint` field-for-field
(`ai-provider-service.md:84-90`) — without them, `ResolveProvider` cannot
filter by provider type or scope to a specific dev server, which is the
root cause of BUG-AIP-02's "hands back a different provider's account"
finding. `account_id`/`scoped_ref` are extensions beyond §3's literal
sketch (flagged in SOL-AIP-02's rationale) needed for BL-AIP-02's Case 1/2
acceptance criteria — lower severity than the filtering fix itself, but
added here since they're the same message.

## Changes to make

In `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`, replace:

```protobuf
message ResolveProviderRequest {
  string tenant_id = 1;
  string user_id = 2;
  string project_id = 3;
  string dev_server_id = 4;  // NEW — matches ai-provider-service.md §3's ResolveRequest field-for-field
  optional string model_hint = 5;  // NEW — same

  // NEW — extensions beyond §3's sketch (SOL-AIP-02's rationale). Zero
  // value on both means "run the normal cascade"; setting account_id
  // short-circuits it entirely (Case 1); setting scoped_ref parses and
  // resolves directly (Case 2). Not a oneof, to keep wire compatibility
  // trivial for existing callers that only ever set tenant_id/user_id/
  // project_id today.
  string account_id = 6;
  string scoped_ref = 7;
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

Expected: clean build, `buf breaking` reports only additions.

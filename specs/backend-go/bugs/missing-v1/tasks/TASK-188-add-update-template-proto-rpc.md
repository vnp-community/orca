# TASK-188: Add `UpdateTemplate` RPC to `workflow.proto` (+ `version` field on `WorkflowTemplate`)

**From Solution:** SOL-030
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`workflow.execute`/`workflow.cancel`/`workflow.template.create` are already
backed by real RPCs (`Execute`/`CancelExecution`/`CreateTemplate`) — no
proto change needed for those three. Only `workflow.template.update` has no
backing RPC: `workflow.proto`'s `WorkflowService` has no `UpdateTemplate`.
This is a deliberate scope addition beyond `workflow-service.md` §3's RPC
sketch (that doc doesn't list `UpdateTemplate` either) — flagged as such,
same class of addition SOL-001 made for `auth-service`'s `GetAdminStats`.

`templates.version INT NOT NULL DEFAULT 1` is already in the schema
(`workflow-service.md:154`), unused by any RPC today. `UpdateTemplate` must
bump `version` on every write (never mutate in place) and use it for
optimistic concurrency — same pattern SOL-001 used for `auth-service`'s
`AccessPolicy`.

## Changes to make

**File:** `backend-go/proto/orca/workflow/v1/workflow.proto`

Add to the `WorkflowService` block, after `CreateTemplate`:

```protobuf
  rpc CreateTemplate(CreateTemplateRequest) returns (CreateTemplateResponse);

  // UpdateTemplate bumps templates.version on every write (never an
  // in-place mutation) — deliberate scope addition beyond
  // workflow-service.md §3's RPC sketch, flagged in SOL-030. Editing a
  // template never retroactively changes a running execution:
  // WorkflowExecution.DefinitionSnapshot freezes the resolved DAG at
  // Execute time (§4), so no active-execution guard is needed here.
  rpc UpdateTemplate(UpdateTemplateRequest) returns (UpdateTemplateResponse);
```

Add `int32 version = 7;` to the existing `WorkflowTemplate` message:

```protobuf
message WorkflowTemplate {
  string id = 1;
  string tenant_id = 2;
  string name = 3;
  string dag_json = 4;
  string scope = 5;
  string parent_template_id = 6;
  int32 version = 7; // bumped by UpdateTemplate on every write; 1 at creation
}
```

Add the new messages at the bottom of the file:

```protobuf
message UpdateTemplateRequest {
  string id = 1;
  // Field-mask-free: every field is always sent (matches
  // CreateTemplateRequest's shape — no PATCH-style partial update in this
  // reduced surface, same simplification project-service.md's
  // UpdateProject accepts for its own non-dev_server_id fields).
  string name = 2;
  string dag_json = 3;
  string scope = 4;
  string parent_template_id = 5; // empty = detach from any parent
  int32 expected_version = 6;    // optimistic concurrency: reject if templates.version has moved
}

message UpdateTemplateResponse {
  WorkflowTemplate template = 1; // includes the bumped version
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

Expected: clean build, `buf breaking` reports only additions (1 new RPC, 2
new messages, 1 new field on an existing message).

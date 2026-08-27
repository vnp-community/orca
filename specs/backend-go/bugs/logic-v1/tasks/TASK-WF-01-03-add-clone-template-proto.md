# TASK-WF-01-03: Add authoring fields + `CloneTemplate` RPC to `workflow.proto`

**From Solution:** SOL-WF-01
**Priority:** P0 — usecase/REST tasks depend on generated stubs from this
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

The proto `WorkflowTemplate`/`CreateTemplateRequest`/`UpdateTemplateRequest`
messages have no fields for owner/description/tags/inherit-merge, and
there is no `CloneTemplate` RPC at all. This task adds both — additive
only, `buf breaking` stays clean.

## Changes to make

In `backend-go/proto/orca/workflow/v1/workflow.proto`, extend
`WorkflowTemplate`:

```protobuf
message WorkflowTemplate {
  string id = 1;
  string tenant_id = 2;
  string name = 3;
  string dag_json = 4;
  string scope = 5;
  string parent_template_id = 6;
  int32 version = 7;

  string owner_id = 8;
  string description = 9;
  repeated string tags = 10;
  string overrides_json = 11;
  string inject_steps_json = 12;
  string remove_steps_json = 13;
  int32 usage_count = 14;
  string cloned_from_template_id = 15;
}
```

Extend `CreateTemplateRequest` and `UpdateTemplateRequest` (existing
fields unchanged, append):

```protobuf
  string description = 6;
  repeated string tags = 7;
  string overrides_json = 8;
  string inject_steps_json = 9;
  string remove_steps_json = 10;
```

Add the new RPC and its messages to the `WorkflowService` service block:

```protobuf
// CloneTemplate snapshots a RESOLVED template (server-computed) into a
// brand-new, disconnected root template — distinct from CreateTemplate,
// which always takes caller-supplied dag_json.
rpc CloneTemplate(CloneTemplateRequest) returns (CloneTemplateResponse);

message CloneTemplateRequest {
  string source_template_id = 1;
  string name = 2;
  string description = 3;
  repeated string tags = 4;
}
message CloneTemplateResponse {
  WorkflowTemplate template = 1;
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

Expected: clean build, `buf breaking` reports no breaking changes (only
additions).

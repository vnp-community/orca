# TASK-WF-03-03: Add sharing/approval/rating/search RPCs to `workflow.proto`

**From Solution:** SOL-WF-03
**Priority:** P0 — usecase/REST tasks depend on generated stubs from this
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto`
**Depends on:** TASK-WF-01-03 (appends to the same `WorkflowTemplate` message SOL-WF-01 extends — land that proto change first to avoid a merge conflict on the message body)
**Status:** `[ ]` TODO

---

## Context

None of BUG-WF-03's publish/approval/share-link/preview/import/rate/
search surface exists in the proto today. This adds it — additive only,
`buf breaking` stays clean.

## Changes to make

Extend `WorkflowTemplate` (append after SOL-WF-01's fields, next free
field numbers):

```protobuf
  string visibility = 16;
  string share_token = 17;
  int32 rating_sum = 18;
  int32 rating_count = 19;
```

Add RPCs and messages to the `WorkflowService` service block:

```protobuf
rpc PublishTemplate(PublishTemplateRequest) returns (WorkflowTemplate);
message PublishTemplateRequest {
  string template_id = 1;
  string new_visibility = 2;
}

rpc ListPendingApprovals(ListPendingApprovalsRequest) returns (ListPendingApprovalsResponse);
message ListPendingApprovalsRequest { string page_token = 1; int32 page_size = 2; }
message ListPendingApprovalsResponse { repeated Approval approvals = 1; string next_page_token = 2; }
message Approval {
  string id = 1;
  string template_id = 2;
  string requested_by = 3;
  string status = 4;
  string resolved_by = 5;
  google.protobuf.Timestamp resolved_at = 6;
}

rpc ResolveApproval(ResolveApprovalRequest) returns (Approval);
message ResolveApprovalRequest {
  string approval_id = 1;
  string decision = 2; // "approved" | "rejected"
}

rpc GenerateShareLink(GenerateShareLinkRequest) returns (GenerateShareLinkResponse);
message GenerateShareLinkRequest { string template_id = 1; }
message GenerateShareLinkResponse { string share_token = 1; }

rpc PreviewSharedTemplate(PreviewSharedTemplateRequest) returns (SharedTemplatePreview);
message PreviewSharedTemplateRequest { string share_token = 1; }
message SharedTemplatePreview {
  string name = 1;
  string description = 2;
  repeated string tags = 3;
  string dag_json = 4;
  int32 rating_sum = 5;
  int32 rating_count = 6;
}

rpc ImportSharedTemplate(ImportSharedTemplateRequest) returns (WorkflowTemplate);
message ImportSharedTemplateRequest { string share_token = 1; }

rpc RateTemplate(RateTemplateRequest) returns (RateTemplateResponse);
message RateTemplateRequest { string template_id = 1; int32 stars = 2; }
message RateTemplateResponse { int32 rating_sum = 1; int32 rating_count = 2; }
```

Extend `ListTemplatesRequest`:

```protobuf
  string query = 4;           // full-text against name/description
  repeated string tags = 5;   // AND-filter — every listed tag must be present
  string sort = 6;            // "trending" | "recent" | "" (default: id order)
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

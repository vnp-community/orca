# TASK-PI-03-01: Add `WorktreeLifecycleEvent`/`PullRequestLifecycleEvent` payload messages + `ReceiveWebhook` RPC

**From Solution:** SOL-PI-03
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `project-service`, `scm-integration-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`, `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
**Depends on:** TASK-PI-02-02 (uses `Worktree.linked_issue_provider`/`linked_issue_ref` added there)
**Status:** `[ ]` TODO

---

## Context

BUG-PI-03 finds no event/webhook plumbing exists at all for issue-status
sync. This task adds only the proto surface: the two event payload shapes
(published via NATS JetStream, not gRPC — the schema still lives in each
publisher's own proto file per this codebase's convention) and
`scm-integration-service`'s new webhook-receiver RPC, which is what gives
`webhook_delivery_log` (an existing but currently-unused table,
`scm-integration-service.md:155`) its first writer.

## Changes to make

### `project.proto` — append near `Worktree`/`RecordWorktreeCreatedRequest`

```protobuf
// Subject: orca.project.worktree.created / orca.project.worktree.deleted
// Naming per 08-inter-service-communication.md: orca.<service>.<entity>.<event>
message WorktreeLifecycleEvent {
  string event_id = 1;          // consumer-side dedup key
  string tenant_id = 2;
  string occurred_at = 3;       // RFC3339
  int32 schema_version = 4;     // starts at 1
  string worktree_id = 5;
  string project_id = 6;
  string linked_issue_provider = 7;  // empty = no linked issue, consumer no-ops
  string linked_issue_ref = 8;
  bool had_open_pr = 9;              // worktree.deleted only; publisher always sends false — see record_worktree_removed.go's doc comment (TASK-PI-03-03)
}
```

### `scmintegration.proto` — append near the RPC service block and `PullRequest`

Add to the `ScmIntegrationService` service block:

```protobuf
  rpc ReceiveWebhook(ReceiveWebhookRequest) returns (ReceiveWebhookResponse);
```

Add messages:

```protobuf
// Subject: orca.scm.pull_request.created / orca.scm.pull_request.merged
message PullRequestLifecycleEvent {
  string event_id = 1;
  string tenant_id = 2;
  string occurred_at = 3;
  int32 schema_version = 4;
  string provider = 5;
  string repo = 6;
  int32 pr_number = 7;
  string linked_issue_provider = 8;
  string linked_issue_ref = 9;
}

// Webhook ingestion — plain HTTP via api-gateway at
// /v1/scm/webhooks/{provider}, forwarded to this RPC. Deliberate exception
// to gRPC-for-sync: the caller is GitHub/GitLab's own servers, which cannot
// be given an Orca JWT.
message ReceiveWebhookRequest {
  string provider = 1;
  bytes raw_body = 2;             // signature verified against this exact byte sequence
  string signature_header = 3;
  string delivery_id_header = 4;  // GitHub X-GitHub-Delivery / GitLab X-Gitlab-Event-UUID
}
message ReceiveWebhookResponse {
  bool accepted = 1;
  bool duplicate = 2;
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

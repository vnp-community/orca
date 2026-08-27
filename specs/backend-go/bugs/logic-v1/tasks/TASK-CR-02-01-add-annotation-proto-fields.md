# TASK-CR-02-01: Add side/range/sent-state fields and `MarkAnnotationsSent` RPC to `annotation.proto`

**From Solution:** SOL-CR-02
**Priority:** P0 — every other task in this set depends on generated stubs from this
**Service:** `annotation-service`
**File:** `backend-go/proto/orca/annotation/v1/annotation.proto`
**Depends on:** none
**Status:** `[x]` DONE — annotation.proto updated with Side enum, Anchor/Annotation/CreateAnnotationRequest/ListAnnotationsRequest/DeleteAnnotationRequest fields and MarkAnnotationsSent RPC; buf generate + buf breaking (against origin/main) clean, go build ./proto/... passes

---

## Context

`BUG-CR-02` needs a diff `side`, a multi-line range, an original-code
snapshot, a `worktree_id` scope, and a `sent_to_agent` state that none of
`Anchor`/`Annotation` carry today. This task adds them as pure-additive
proto fields plus one new RPC (`MarkAnnotationsSent`), so `buf breaking`
stays clean.

## Changes to make

In the `AnnotationService` service block, add:

```protobuf
rpc MarkAnnotationsSent(MarkAnnotationsSentRequest) returns (MarkAnnotationsSentResponse);
```

Replace the `Anchor` message:

```protobuf
enum Side {
  SIDE_UNSPECIFIED = 0; // non-diff comment (plain file/line note) — BR-CR-05 only applies to diff review
  SIDE_OLD = 1;
  SIDE_NEW = 2;
}

message Anchor {
  string repo_id = 1;
  string file_path = 2;
  int32 line = 3;
  string ref = 4; // commit sha or branch, best-effort
  string worktree_id = 5;  // NEW — optional; scope addition, see SOL-CR-02 rationale
  int32 end_line = 6;      // NEW — 0 or == line means single-line; must be >= line (BR-CR-06)
  Side side = 7;           // NEW — BR-CR-05
}
```

Add fields to `Annotation`:

```protobuf
message Annotation {
  string id = 1;
  string tenant_id = 2;
  string author_id = 3;
  Anchor anchor = 4;
  string content = 5;
  bool resolved = 6;
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
  string original_code = 9;               // NEW — BL-CR-02 DiffComment.originalCode
  bool sent_to_agent = 10;                // NEW — distinct from `resolved` (BR-CR-08)
  google.protobuf.Timestamp sent_at = 11; // NEW — nil until MarkAnnotationsSent
}
```

Add a field to `CreateAnnotationRequest`:

```protobuf
message CreateAnnotationRequest {
  Anchor anchor = 1;
  string content = 2;
  string request_id = 3;
  string original_code = 4; // NEW
}
```

Add fields to `ListAnnotationsRequest`:

```protobuf
message ListAnnotationsRequest {
  string repo_id = 1;
  string file_path = 2; // optional filter
  string page_token = 3;
  int32 page_size = 4;
  string worktree_id = 5;          // NEW — optional filter, alternative to repo_id+file_path
  optional bool sent_to_agent = 6; // NEW — lets a caller ask for only-unsent (SOL-CR-03's send flow)
}
```

Add a field to `DeleteAnnotationRequest`:

```protobuf
message DeleteAnnotationRequest {
  string id = 1;
  bool confirmed = 2; // NEW — BR-CR-08; see TASK-CR-02-05
}
```

Append new RPC messages at the bottom of the file:

```protobuf
message MarkAnnotationsSentRequest {
  repeated string ids = 1;
}

message MarkAnnotationsSentResponse {
  repeated Annotation annotations = 1;
}
```

`worktree_id` is deliberately optional on both `Anchor` and
`ListAnnotationsRequest` — existing callers that only know `repo_id`
continue to work unchanged.

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

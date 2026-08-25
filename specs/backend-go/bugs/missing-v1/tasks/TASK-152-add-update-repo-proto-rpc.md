# TASK-152: Add `UpdateRepo` RPC to `project.proto` (Bucket 2)

**From Solution:** SOL-023 (Bucket 2)
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[x]` DONE (verified) — `UpdateRepo` RPC + `UpdateRepoRequest`/`UpdateRepoResponse` added to `project.proto`, stubs regenerated via `buf generate` in `backend-go/proto/`, `go build ./proto/...` clean. `buf breaking` not run (no usable git remote for `.git#branch=main` inside this worktree) — additive-only change, `go build` confirms no breakage.

---

## Context

`repo.update` has no backing RPC. `Repo{url, display_name, position}`
(`project.proto:136-141`) is plain Postgres metadata with no
working-tree/host field — an edit to `display_name`/`url` is exactly the
shape of `UpdateProject`'s field-mask pattern (`project.proto:22`, "empty
string means no change"), not a git operation, so it belongs next to
`AddRepo`/`RemoveRepo` on `project-service`, not on `git-gateway-service`.

## Changes to make

**File:** `backend-go/proto/orca/project/v1/project.proto`

Add the RPC to the `ProjectService` service block, next to the other
`Repo` RPCs:

```protobuf
  rpc AddRepo(AddRepoRequest) returns (AddRepoResponse);
  rpc ListRepos(ListReposRequest) returns (ListReposResponse);
  rpc ReorderRepos(ReorderReposRequest) returns (ReorderReposResponse);
  rpc RemoveRepo(RemoveRepoRequest) returns (RemoveRepoResponse);
  rpc UpdateRepo(UpdateRepoRequest) returns (UpdateRepoResponse); // NEW
```

Add the messages, next to `RemoveRepoRequest`/`RemoveRepoResponse`:

```protobuf
message UpdateRepoRequest {
  string repo_id = 1;
  string url = 2;          // empty = no change
  string display_name = 3; // empty = no change
}

message UpdateRepoResponse {
  Repo repo = 1;
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

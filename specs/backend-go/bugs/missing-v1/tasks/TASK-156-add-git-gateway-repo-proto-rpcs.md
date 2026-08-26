# TASK-156: Add 8 new repo-shaped RPCs to `gitgateway.proto` (Bucket 3)

**From Solution:** SOL-023 (Bucket 3)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`repo.clone`, `repo.baseRefDefault`, `repo.searchRefs`, `repo.create`,
`repo.hooksCheck`, `repo.issueCommandRead`, `repo.issueCommandWrite`,
`repo.setupScriptImports` all read or write files inside a repo's working
tree (git hooks, `.orca`/setup-script imports, an issue-command config
file) or run `git init` against a host path — none of them touch the
`repos` Postgres table's `url`/`display_name`/`position` columns, so they
belong on `git-gateway-service`, following its resolve→dispatch→translate
model (`git-gateway-service.md` §2) exactly like the existing
`GetStatus`/`GetDiff`.

**Scope-addition flag** (same posture as SOL-001's `GetAdminStats`
addition): none of these 8 RPCs are in `git-gateway-service.md`'s own §3
API-surface sketch — this is a genuine scope addition beyond the TDD, not
a gap in an RPC the TDD already specified.

## Changes to make

**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`

Add to the `GitGatewayService` service block (current RPCs:
`GetStatus`, `GetDiff`, `Commit`, `Push`, `Pull`,
`GenerateCommitMessage`):

```protobuf
  rpc Clone(CloneRequest) returns (CloneResponse);
  rpc BaseRefDefault(BaseRefDefaultRequest) returns (BaseRefDefaultResponse);
  rpc SearchRefs(SearchRefsRequest) returns (SearchRefsResponse);
  rpc InitRepo(InitRepoRequest) returns (InitRepoResponse);          // repo.create
  rpc CheckHooks(CheckHooksRequest) returns (CheckHooksResponse);     // repo.hooksCheck
  rpc ReadIssueCommand(ReadIssueCommandRequest) returns (ReadIssueCommandResponse);
  rpc WriteIssueCommand(WriteIssueCommandRequest) returns (google.protobuf.Empty);
  rpc ScanSetupScriptImports(ScanSetupScriptImportsRequest) returns (ScanSetupScriptImportsResponse);
```

If `google.protobuf.Empty` is not already imported in this file, add:

```protobuf
import "google/protobuf/empty.proto";
```

Append the messages to the bottom of the file:

```protobuf
message CloneRequest {
  string dev_server_id = 1; // which host to clone onto — resolved via infra-fleet-service by the caller (project-service context) before this call
  string url = 2;
  string dest_path = 3;
}
message CloneResponse {
  string worktree_path = 1;
  string default_branch = 2;
}

message BaseRefDefaultRequest { string worktree_id = 1; }
message BaseRefDefaultResponse { string ref = 1; }

message SearchRefsRequest { string worktree_id = 1; string query = 2; }
message SearchRefsResponse { repeated string refs = 1; }

// InitRepo runs `git init` at dest_path on the resolved host and returns
// enough for the caller to then call ProjectService.AddRepo — mirrors
// project-service.md §2's "git-gateway-service does the git op, then
// writes back metadata" saga already established for worktrees
// (RecordWorktreeCreated), applied here to repo creation instead.
message InitRepoRequest {
  string dev_server_id = 1;
  string dest_path = 2;
  string default_branch = 3; // empty = git's own default
}
message InitRepoResponse {
  string path = 1;
  string default_branch = 2;
}

message CheckHooksRequest { string worktree_id = 1; }
message CheckHooksResponse { repeated string installed_hooks = 1; bool orca_hooks_current = 2; }

message ReadIssueCommandRequest { string worktree_id = 1; }
message ReadIssueCommandResponse { string content = 1; bool exists = 2; }

message WriteIssueCommandRequest { string worktree_id = 1; string content = 2; }

message ScanSetupScriptImportsRequest { string worktree_id = 1; }
message ScanSetupScriptImportsResponse { repeated string imported_paths = 1; }
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

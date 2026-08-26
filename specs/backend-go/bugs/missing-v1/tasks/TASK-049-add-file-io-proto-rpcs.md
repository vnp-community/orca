# TASK-049: Add File I/O RPCs to `gitgateway.proto`

**From Solution:** SOL-009 (Design — Proto additions)
**Priority:** P0 — every other `files.*` task depends on generated stubs from this
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[x]` DONE — all 14 RPCs and their messages added to `gitgateway.proto` (`ReadFile`/`ReadFileChunk`/`ReadFilePreview`/`ReadDir`/`WriteFile`/`WriteFileChunk`/`CreateDir`/`DeleteFile`/`StatFile`/`SearchFiles`/`ListAllFiles`/`ListMarkdownDocuments`/`RenameFile`/`CopyFile`), `buf generate proto` clean, `go build ./proto/...` clean.

---

## Context

`git-gateway-service` currently exposes only git RPCs (`GetStatus`,
`GetDiff`, `Commit`, `Push`, `Pull`, `GenerateCommitMessage`). BUG-009
found no backend-go service exposes file I/O; SOL-009 extends this
service's existing proto package (`orca.git.v1`) rather than standing up
an 18th service, since `files.*` and `git.*` share every dependency
(`project-service`, `infra-fleet-service`) and the same
resolve-host-then-dispatch shape this service already implements via
`ConnectionResolver`/`GitExecutor`.

18 `files.*` frontend methods map to 16 new RPCs — `files.writeBase64`
folds onto `WriteFile` via an `encoding` field and
`files.createDirNoClobber` folds onto `CreateDir` via a `no_clobber`
field, so no separate RPCs for those two. `files.commitUpload` and
`files.unwatch` are always-local renderer-side bookkeeping (no RPC at
all) — handled directly in wscompat, see TASK-058.

`ReadFileChunk` is unsupported for any remote target (SSH/Dev Server
relay) — the usecase layer rejects before calling the executor, see
TASK-052. `RenameFile`/`CopyFile` work locally but are `NOT_SUPPORTED`
over any relay target — the Dev Server Agent's `fs.*` surface has no
`rename`/`copy` — see TASK-055.

---

## Changes to make

**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`

### Step 1: Add RPCs to the `GitGatewayService` service block

```protobuf
service GitGatewayService {
  // ... existing git RPCs unchanged ...

  // ── File I/O — resolve worktree_id -> host -> local exec or relay,
  // identical dispatch shape to every git.* RPC above. ─────────────────
  rpc ReadFile(ReadFileRequest) returns (ReadFileResponse);
  rpc ReadFileChunk(ReadFileChunkRequest) returns (ReadFileChunkResponse);
  rpc ReadFilePreview(ReadFilePreviewRequest) returns (ReadFilePreviewResponse);
  rpc ReadDir(ReadDirRequest) returns (ReadDirResponse);
  rpc WriteFile(WriteFileRequest) returns (WriteFileResponse);
  rpc WriteFileChunk(WriteFileChunkRequest) returns (WriteFileChunkResponse);
  rpc CreateDir(CreateDirRequest) returns (CreateDirResponse);
  rpc DeleteFile(DeleteFileRequest) returns (google.protobuf.Empty);
  rpc StatFile(StatFileRequest) returns (StatFileResponse);
  rpc SearchFiles(SearchFilesRequest) returns (SearchFilesResponse);
  rpc ListAllFiles(ListAllFilesRequest) returns (ListAllFilesResponse);
  rpc ListMarkdownDocuments(ListMarkdownDocumentsRequest) returns (ListMarkdownDocumentsResponse);
  // Known gaps carried forward from the old backend — both RPCs exist so
  // the contract is honest, but return FAILED_PRECONDITION whenever
  // dispatch resolves to a relay target, never a silent no-op.
  rpc RenameFile(RenameFileRequest) returns (RenameFileResponse);
  rpc CopyFile(CopyFileRequest) returns (CopyFileResponse);
}
```

### Step 2: Add messages (append to the bottom of the file)

```protobuf
message ReadFileRequest {
  string worktree_id = 1;
  string path = 2;       // worktree-relative
}
message ReadFileResponse {
  bytes content = 1;
  string encoding = 2;   // "utf8" | "base64" — binary files come back base64
}

message ReadFileChunkRequest {
  string worktree_id = 1;
  string path = 2;
  int64 offset_bytes = 3;
  int64 length_bytes = 4;
}
message ReadFileChunkResponse {
  bytes content = 1;
  bool eof = 2;
}

message ReadFilePreviewRequest {
  string worktree_id = 1;
  string path = 2;
  int64 max_bytes = 3;
}
message ReadFilePreviewResponse {
  bytes content = 1;
  bool truncated = 2;
}

message ReadDirRequest {
  string worktree_id = 1;
  string path = 2;
}
message DirEntry {
  string name = 1;
  bool is_directory = 2;
}
message ReadDirResponse {
  repeated DirEntry entries = 1;
}

message WriteFileRequest {
  string worktree_id = 1;
  string path = 2;
  bytes content = 3;
  string encoding = 4;   // mirrors ReadFileResponse.encoding
  bool create_parents = 5;
}
message WriteFileResponse {
  int64 bytes_written = 1;
}

message WriteFileChunkRequest {
  string worktree_id = 1;
  string path = 2;
  int64 offset_bytes = 3;
  bytes content = 4;      // base64-decoded before this RPC is called
  bool is_final = 5;
}
message WriteFileChunkResponse {
  int64 bytes_written = 1;
}

message CreateDirRequest {
  string worktree_id = 1;
  string path = 2;
  bool recursive = 3;
  bool no_clobber = 4;   // files.createDirNoClobber collapses onto this field
}
message CreateDirResponse {}

message DeleteFileRequest {
  string worktree_id = 1;
  string path = 2;
  bool recursive = 3;
}

message StatFileRequest {
  string worktree_id = 1;
  string path = 2;
}
message StatFileResponse {
  bool exists = 1;
  bool is_directory = 2;
  int64 size_bytes = 3;
  int64 modified_at_unix_ms = 4;
}

message SearchFilesRequest {
  string worktree_id = 1;
  string pattern = 2;
  bool is_regex = 3;
  string path_glob = 4;
  int32 max_results = 5;
}
message SearchMatch {
  string path = 1;
  int32 line = 2;
  string line_text = 3;
}
message SearchFilesResponse {
  repeated SearchMatch matches = 1;
}

message ListAllFilesRequest {
  string worktree_id = 1;
  string path_glob = 2;
  int32 max_results = 3;
}
message ListAllFilesResponse {
  repeated string paths = 1;
}

message ListMarkdownDocumentsRequest {
  string worktree_id = 1;
  int32 max_results = 2;
}
message ListMarkdownDocumentsResponse {
  repeated string paths = 1;
}

message RenameFileRequest {
  string worktree_id = 1;
  string from_path = 2;
  string to_path = 3;
}
message RenameFileResponse {}

message CopyFileRequest {
  string worktree_id = 1;
  string from_path = 2;
  string to_path = 3;
}
message CopyFileResponse {}
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

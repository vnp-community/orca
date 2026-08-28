# TASK-PW-02-01: Add `size_bytes` to `DirEntry` in `gitgateway.proto`

**From Solution:** SOL-PW-02
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[x]` DONE — added DirEntry.size_bytes field 3, buf generate clean, no breaking changes vs origin/main

---

## Context

`ReadDirResponse.DirEntry` (`gitgateway.proto:462-465`) has no size field,
forcing the frontend's `expandDirectory()` into a client-side N+1
`files.stat`-per-entry workaround. `StatFileResponse` already has
`size_bytes` (`gitgateway.proto:510-515`), proving the value is already
computable on both the local and relay dispatch paths — this task is a
pure additive wire-shape extension.

## Changes to make

In `gitgateway.proto`, update `DirEntry`:

```protobuf
message DirEntry {
  string name = 1;
  bool is_directory = 2;
  // Added (SOL-PW-02): 0 for directories — a directory's "size" isn't a
  // meaningful git-gateway-service concept and the frontend doesn't render
  // one for folder rows (BL-PW-02's expandDirectory() contract). Populated
  // from the same os.Stat/fs.stat source StatFile already uses.
  int64 size_bytes = 3;
}
```

`ReadDirRequest`/`ReadDirResponse` are otherwise untouched — no new RPC, no
request-shape change.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only an addition (new field
3 on an existing message, no removed/renumbered fields).

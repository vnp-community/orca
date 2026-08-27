# SOL-009: Add a file-I/O RPC surface to `git-gateway-service` and wire `files.*`

**Resolves:** [BUG-009](../BUG-009-files-channels-not-implemented.md)
**Service:** `git-gateway-service` (extended — see "Where this belongs"
below) + `api-gateway`
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
- `backend-go/services/git-gateway-service/internal/usecase/read_file.go`, `write_file.go`, `stat_file.go`, `read_dir.go`, `list_all_files.go`, `search_files.go`, `create_dir.go`, `delete_file.go`, `rename_file.go`, `copy_file.go`, … (new use cases; see signature table)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` (extend with `FilesystemExecutor`)
- `backend-go/services/git-gateway-service/internal/adapter/local/` (extend the existing local executor)
- `backend-go/services/git-gateway-service/internal/adapter/devserveragent/` (extend the existing relay client's method set)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerFilesChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Where this belongs: extend `git-gateway-service`, don't create an 18th service

BUG-009 already ruled out "just wire an existing method" — no backend-go
service exposes file I/O today — and asked whoever picks this up to decide
between extending `git-gateway-service` or standing up a new service.
`git-gateway-service.md` settles this by describing infrastructure that is
*already the exact shape `files.*` needs*, method-for-method:

- **Same worktree-scoped identity.** Every `files.*` call from
  `runtime-file-client.ts` operates against a worktree path, exactly like
  every `git.*` call (`git-gateway-service.md` §3: "Every request carries a
  `worktree_id`... rather than a raw filesystem path").
- **Same dispatch shape.** §2's "resolve host → dispatch → translate" is
  BUG-009's dispatch model verbatim: resolve `worktree_id` → `project-service`
  → resolve `connectionId` → `infra-fleet-service` → local exec or relay to
  the Dev Server Agent.
- **Same ports, already defined.** §6's `usecase/ports.go` already lists
  `WorktreeResolver`, `HostResolver`, `GitExecutor` (local), `AgentRelayClient`,
  `ConnectionCache` — everything `files.*` needs except a `FilesystemExecutor`
  port parallel to `GitExecutor`.
- **Same "owns no data" shape** (§5) — file I/O is exactly as stateless as
  git plumbing; no new database, no new migrations.

Standing up a separate `file-io-service` would duplicate all five ports and
the entire resolve-dispatch-translate machinery for no isolation benefit —
`files.*` and `git.*` share every dependency (`project-service`,
`infra-fleet-service`) and every failure mode (`ErrHostUnreachable`). This
solution extends `git-gateway-service`'s existing proto package
(`orca.git.v1`) with a `FileIO` RPC group rather than adding an 18th
service to the 17-service catalog — consistent with
`02-microservices-decomposition.md`'s design principle 4 ("no service is a
thin CRUD wrapper... folded into the closest service that owns related
workflow logic"), which applies here in reverse: file I/O has *no* workflow
logic of its own, so it folds into the service that already owns the
resolve/dispatch machinery it needs, same reasoning `workspacePorts.*`
followed folding into `infra-fleet-service` per
`02-microservices-decomposition.md`'s "What's deliberately not a separate
service" list.

`git-gateway-service.md` never explicitly rules file I/O in or out of its
proto (§3's RPC list is git-only) — this is this solution's proposed
extension, not something already specified. Flag as a scope addition to
that service's TDD, the same way SOL-001 flagged `GetAdminStats` as a scope
addition beyond `auth-service.md`.

`02-microservices-decomposition.md`'s dependency graph already has
`git --> proj` and `git --> infra` (the exact two calls file I/O needs) —
no new edges required.

---

## Design — Proto additions (`gitgateway.proto`)

18 methods map to 16 new RPCs (2 — `commitUpload`, `unwatch` — are
always-local bookkeeping per BUG-009's own finding, no RPC needed; see
"Local-only channels" below). Representative sketch for the three shapes
that cover most of the namespace (simple read, simple write, and the two
known-gap ops), then a signature table for the rest.

```protobuf
service GitGatewayService {
  // ... existing git RPCs unchanged ...

  // ── File I/O — resolve worktree_id -> host -> local exec or relay,
  // identical dispatch shape to every git.* RPC above (see §2 of
  // git-gateway-service.md). ─────────────────────────────────────────
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
  // Known gaps carried forward from the old backend (see §"Known gaps"
  // below) — both RPCs exist so the contract is honest, but return
  // FAILED_PRECONDITION/NOT_SUPPORTED whenever dispatch resolves to a
  // relay target, never a silent no-op.
  rpc RenameFile(RenameFileRequest) returns (RenameFileResponse);
  rpc CopyFile(CopyFileRequest) returns (CopyFileResponse);
}

message ReadFileRequest {
  string worktree_id = 1;
  string path = 2;       // worktree-relative
}
message ReadFileResponse {
  bytes content = 1;
  string encoding = 2;   // "utf8" | "base64" — binary files come back base64
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
```

### Signature table — remaining RPCs

| `files.*` method | RPC | Request fields (beyond `worktree_id`) | Notes |
|---|---|---|---|
| `files.readChunk` | `ReadFileChunk` | `path, offset_bytes, length_bytes` | **NOT_SUPPORTED for any relay target** — preserve the old backend's intentional scope limit (see "Known gaps") |
| `files.readPreview` | `ReadFilePreview` | `path, max_bytes` | Truncated read, same local/relay dispatch as `ReadFile` |
| `files.readDir` | `ReadDir` | `path` | Non-recursive listing; entries carry `name, is_directory` |
| `files.writeBase64` | maps to `WriteFile` with `encoding="base64"` | — | No separate RPC — base64 vs. utf8 is a wire-encoding choice, not a distinct operation; the usecase layer decodes before calling the executor port |
| `files.writeBase64Chunk` | `WriteFileChunk` | `path, offset_bytes, content (base64), is_final` | Append-at-offset semantics for large-file upload |
| `files.createDir` | `CreateDir` | `path, recursive` | |
| `files.createDirNoClobber` | maps to `CreateDir` with `no_clobber=true` | — | One RPC, one bool field — not a second RPC (mirrors `WriteFile`'s encoding-field precedent above) |
| `files.delete` | `DeleteFile` | `path, recursive` | |
| `files.rename` | `RenameFile` | `from_path, to_path` | **NOT_SUPPORTED for any relay target** (known gap, see below) |
| `files.copy` | `CopyFile` | `from_path, to_path` | **NOT_SUPPORTED for any relay target** (known gap, see below) |
| `files.search` | `SearchFiles` | `pattern, is_regex, path_glob, max_results` | Content grep, relays to the agent's `grep` fs case per BUG-009's reference |
| `files.listAll` | `ListAllFiles` | `path_glob, max_results` | Relays to the agent's `glob` fs case |
| `files.listMarkdownDocuments` | `ListMarkdownDocuments` | `max_results` | Same underlying `glob`/walk as `ListAllFiles`, filtered server-side to `*.md`/`*.mdx` — one usecase, thin wrapper, not a duplicate walk implementation |
| `files.stat` | `StatFile` | `path` | |

`CreateDirNoClobber` and `writeBase64` folding into one RPC each (rather
than 1:1 method mapping) keeps the proto surface at 16 RPCs instead of 18 —
flagged explicitly since it's a deliberate collapse, not an omission.

### Known gaps carried forward (per BUG-009's explicit instruction to preserve or document divergence)

- **`ReadFileChunk`** — unsupported for *any* remote target (SSH or Dev
  Server), by design, matching the old backend. The usecase layer checks
  dispatch target before even attempting the relay call and returns
  `FAILED_PRECONDITION` with a clear message, rather than attempting a
  relay call that the agent's `fs.*` surface doesn't implement anyway.
- **`RenameFile`/`CopyFile`** — the Dev Server Agent's `fs.*` surface only
  implements `stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep`
  (BUG-009's finding, citing `agent/src/relay/agent-rpc-dispatch.ts`'s
  `fs.*` cases) — no `rename`/`copy`. Both RPCs work for the host-local
  dispatch branch (implemented as `os.Rename`/a read-then-write copy against
  the local filesystem) but return `NOT_SUPPORTED` when dispatch resolves
  to a relay target, exactly matching old-backend behavior. This is **not**
  a new `agent/` dependency this solution needs — it's an explicit,
  documented continuation of an existing limitation, not a blocker like
  SOL-008's ADB/`simctl` gap.

---

## Design — `usecase/` layer

Extends `git-gateway-service`'s existing ports (`ports.go`) with one new
port, `FilesystemExecutor`, parallel to the existing `GitExecutor`:

```go
// internal/usecase/ports.go (extended)
type FilesystemExecutor interface {
    ReadFile(ctx context.Context, worktreePath, relPath string) ([]byte, error)
    WriteFile(ctx context.Context, worktreePath, relPath string, content []byte, createParents bool) (int64, error)
    Stat(ctx context.Context, worktreePath, relPath string) (domain.FileStat, error)
    ReadDir(ctx context.Context, worktreePath, relPath string) ([]domain.DirEntry, error)
    CreateDir(ctx context.Context, worktreePath, relPath string, recursive, noClobber bool) error
    Delete(ctx context.Context, worktreePath, relPath string, recursive bool) error
    Search(ctx context.Context, worktreePath string, opts domain.SearchOptions) ([]domain.SearchMatch, error)
    Glob(ctx context.Context, worktreePath, pattern string, maxResults int) ([]string, error)
    // Rename/Copy deliberately absent from this interface's relay-backed
    // implementation — see AgentRelayClient below. The local implementation
    // (adapter/local) still provides them via a *separate* narrower
    // interface, LocalOnlyFilesystemExecutor, so the type system reflects
    // the known-gap asymmetry instead of a runtime-only check.
}

type LocalOnlyFilesystemExecutor interface {
    Rename(ctx context.Context, worktreePath, fromRel, toRel string) error
    Copy(ctx context.Context, worktreePath, fromRel, toRel string) error
}
```

Representative usecase (the shape every simple RPC — `ReadFile`, `StatFile`,
`ReadDir`, `CreateDir`, `DeleteFile`, `SearchFiles`, `ListAllFiles` — follows;
the rest are one file each, same three-step body):

```go
// internal/usecase/read_file.go
type ReadFileUseCase struct {
    worktrees WorktreeResolver
    hosts     HostResolver
    local     FilesystemExecutor // adapter/local
    relay     AgentRelayClient   // adapter/devserveragent
}

func (uc *ReadFileUseCase) Execute(ctx context.Context, worktreeID, path string) ([]byte, error) {
    wt, err := uc.worktrees.Resolve(ctx, worktreeID)
    if err != nil {
        return nil, err
    }
    host, err := uc.hosts.Resolve(ctx, wt.DevServerID)
    if err != nil {
        return nil, err
    }
    if host.ConnectionID == "" {
        return uc.local.ReadFile(ctx, wt.Path, path)
    }
    raw, err := uc.relay.Call(ctx, host.ConnectionID, "fs.readFile", map[string]any{
        "path": filepath.Join(wt.Path, path),
    })
    if err != nil {
        return nil, translateRelayError(err) // ErrHostUnreachable etc., per §8 of git-gateway-service.md
    }
    return decodeFileContent(raw)
}
```

`RenameFile`/`CopyFile` usecases add the explicit dispatch-target check
`git-gateway-service.md` doesn't need for pure git ops:

```go
// internal/usecase/rename_file.go
func (uc *RenameFileUseCase) Execute(ctx context.Context, worktreeID, from, to string) error {
    wt, err := uc.worktrees.Resolve(ctx, worktreeID)
    if err != nil {
        return err
    }
    host, err := uc.hosts.Resolve(ctx, wt.DevServerID)
    if err != nil {
        return err
    }
    if host.ConnectionID != "" {
        // Known gap, preserved deliberately — see this file's "Known
        // gaps" section. NOT falling back to a relay call that the
        // agent's fs.* surface doesn't implement.
        return usecase.ErrFileOpNotSupportedOverRelay
    }
    return uc.local.Rename(ctx, wt.Path, from, to)
}
```

`ReadFileChunk`'s usecase follows the identical shape — check dispatch
target first, return `ErrChunkedReadNotSupportedRemote` before attempting
any relay call.

### Path safety

Every usecase resolves `path`/`relPath` against the worktree's own root and
rejects any resolved path that escapes it (`filepath.Clean` + prefix check)
before calling either executor — the same "never trust a client-supplied
host path" posture `git-gateway-service.md` §3 already states for
`worktree_id` vs. a raw path, applied one level deeper since file I/O takes
an additional relative path the git RPCs don't.

---

## Design — `wscompat` wiring: one generic dispatcher, not 16 hand-written handlers

Most of the 16 RPC-backed channels share an identical shape: decode one
JSON arg, call one RPC, return the response (or a mapped subset of it).
Wrap that repetition in a small generic helper instead of repeating
`decodeArg`/timeout/error-check boilerplate 16 times — the pattern
`registerGitChannels` and `registerAnnotationChannels` already repeat by
hand at 2 and 4 methods respectively doesn't scale cleanly to 16.

```go
// ── files.* ──────────────────────────────────────────────────────────
//
// files.commitUpload and files.unwatch are always-local renderer-side
// bookkeeping in the old backend (no fs I/O) — registered as local no-op
// acks below, not wired to git-gateway-service. Every other channel
// dispatches through GitGatewayServiceClient's new FileIO RPC group
// (SOL-009), which itself resolves local-vs-relay per worktree — wscompat
// never makes that decision; it only forwards worktreeId + params.

// simpleFileOp wires a channel that decodes one JSON arg, issues one
// gRPC call with an rpcTimeout deadline, and returns the response
// verbatim — the shape 12 of files.*'s 16 RPC-backed channels share.
func simpleFileOp[Req any](
	r *Registry,
	channel string,
	call func(ctx context.Context, req Req) (any, error),
) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[Req](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		return call(rpcCtx, in)
	})
}

func registerFilesChannels(r *Registry, client gitgatewayv1.GitGatewayServiceClient) {
	type readArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
	}
	simpleFileOp(r, "files.read", func(ctx context.Context, in readArgs) (any, error) {
		resp, err := client.ReadFile(ctx, &gitgatewayv1.ReadFileRequest{WorktreeId: in.WorktreeID, Path: in.Path})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
	simpleFileOp(r, "files.stat", func(ctx context.Context, in readArgs) (any, error) {
		resp, err := client.StatFile(ctx, &gitgatewayv1.StatFileRequest{WorktreeId: in.WorktreeID, Path: in.Path})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
	simpleFileOp(r, "files.readDir", func(ctx context.Context, in readArgs) (any, error) {
		resp, err := client.ReadDir(ctx, &gitgatewayv1.ReadDirRequest{WorktreeId: in.WorktreeID, Path: in.Path})
		if err != nil {
			return nil, err
		}
		return resp.GetEntries(), nil
	})
	// readChunk / readPreview / createDir / createDirNoClobber / delete /
	// search / listAll / listMarkdownDocuments / writeBase64Chunk follow
	// the same simpleFileOp(...) call, one args struct + one client method
	// each — omitted here for brevity, see the signature table above for
	// the exact request-field mapping each one needs.

	// files.write / files.writeBase64 collapse onto WriteFile with an
	// encoding switch, per the proto section's note that this is one RPC
	// with a wire-encoding field, not two RPCs.
	type writeArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
		Content    string `json:"content"`
		Base64     bool   `json:"base64"` // true for files.writeBase64
	}
	writeHandler := func(ctx context.Context, in writeArgs) (any, error) {
		content := []byte(in.Content)
		encoding := "utf8"
		if in.Base64 {
			decoded, err := base64.StdEncoding.DecodeString(in.Content)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 content: %w", err)
			}
			content, encoding = decoded, "base64"
		}
		resp, err := client.WriteFile(ctx, &gitgatewayv1.WriteFileRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Content: content, Encoding: encoding,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	simpleFileOp(r, "files.write", writeHandler)
	simpleFileOp(r, "files.writeBase64", writeHandler)

	// files.rename / files.copy — known-gap RPCs; forwarded as-is, the
	// NOT_SUPPORTED-over-relay decision happens usecase-side (see Design
	// section above), wscompat just relays whatever status comes back.
	type renameArgs struct {
		WorktreeID string `json:"worktreeId"`
		From       string `json:"fromPath"`
		To         string `json:"toPath"`
	}
	simpleFileOp(r, "files.rename", func(ctx context.Context, in renameArgs) (any, error) {
		resp, err := client.RenameFile(ctx, &gitgatewayv1.RenameFileRequest{WorktreeId: in.WorktreeID, FromPath: in.From, ToPath: in.To})
		if err != nil {
			return nil, err // FAILED_PRECONDITION over relay surfaces as-is
		}
		return resp, nil
	})
	simpleFileOp(r, "files.copy", func(ctx context.Context, in renameArgs) (any, error) {
		resp, err := client.CopyFile(ctx, &gitgatewayv1.CopyFileRequest{WorktreeId: in.WorktreeID, FromPath: in.From, ToPath: in.To})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// ── Always-local bookkeeping — no gRPC call, matches
	// crashReports.getLatestPending's in-process pattern. ────────────
	r.Register("files.commitUpload", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	r.Register("files.unwatch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
}
```

`RegisterRealChannels` gains `registerFilesChannels(r, gitClient)` next to
the existing `registerGitChannels(r, gitClient)` call — same client, no new
dial needed since `gitClient` is already constructed in `main.go`.

---

## Test plan

- `git-gateway-service/internal/usecase/read_file_test.go` (and one per
  other usecase file) — fake `FilesystemExecutor`/`AgentRelayClient`: local
  dispatch calls the local executor, relay dispatch calls
  `AgentRelayClient.Call` with the right `fs.*` method name, host-unreachable
  maps to `ErrHostUnreachable`.
- `rename_file_test.go` / `copy_file_test.go` — relay dispatch returns
  `ErrFileOpNotSupportedOverRelay` **without** ever calling
  `AgentRelayClient.Call` (assert the fake records zero calls) — regression
  guard against silently attempting an op the agent doesn't support.
- `read_file_chunk_test.go` — same not-called assertion for any
  non-empty `connectionId`.
- `adapter/local/` — integration tests against a real temp-dir worktree:
  write/read/stat/delete/rename/copy/search/glob round-trip correctly,
  path-escape attempts (`../../etc/passwd`) rejected before hitting the
  filesystem.
- `adapter/grpc` — contract tests against the new proto RPCs.
- `wscompat/channels_test.go` — one test per channel using a fake
  `GitGatewayServiceClient`, plus a dedicated `files.writeBase64` test
  asserting invalid base64 returns a decode error before any RPC call.
- Contract test: `files.write` and `files.writeBase64` both resolve to
  `WriteFile` with the correct `encoding` field — regression guard against
  the two-channels-one-RPC collapse drifting.

## References

- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — problem statement, the 18-method list, and the three known-gap findings this solution preserves
- `specs/backend-go/tdd/services/git-gateway-service.md:1-34` (overview, "reference implementation" framing), `:75-119` (§3 API surface, `worktree_id` posture), `:142-160` (§4 domain model shape), `:179-216` (§6 package layout, existing ports this solution extends), `:294-298` (`ErrHostUnreachable` failure mode reused as-is)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:33-36` (design principle 4, folding rationale), `:106-108` (`workspacePorts.*` fold-in precedent this solution's fold-in decision mirrors)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/port layering, one-usecase-per-RPC convention
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:11-16` — existing RPC list this solution extends
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-20,221-252` — package doc conventions and `registerGitChannels`'s existing pattern this solution's `registerFilesChannels` follows
- `backend-go/services/api-gateway/cmd/server/main.go:137` — `gitClient` already constructed, reused for `registerFilesChannels`

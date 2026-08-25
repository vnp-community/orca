# TASK-057: Add `simpleFileOp` wscompat helper and wire `files.read`/`files.write`/`files.stat`/`files.readDir`/... channels

**From Solution:** SOL-009 (Design — `wscompat` wiring: "one generic dispatcher, not 16 hand-written handlers")
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-056
**Status:** `[x]` DONE — `simpleFileOp` + `registerFilesChannels` (read/write/search/list channels) implemented, but in the new `channels_git.go` file, not `channels.go` directly (off-limits for this pass, see that file's package doc comment). `go build`/`go vet` clean.

---

## Context

12 of `files.*`'s 16 RPC-backed channels share an identical shape: decode
one JSON arg, call one RPC with an `rpcTimeout` deadline, return the
response. `simpleFileOp` wraps that repetition — the pattern
`registerGitChannels`/`registerAnnotationChannels` already repeat by hand
at 2–4 methods doesn't scale cleanly to 16. This task covers the read,
write, and search/list channels; TASK-058 covers `files.rename`/
`files.copy` (the known-gap RPCs) and the two always-local bookkeeping
channels, plus the `RegisterRealChannels` wiring both tasks' handlers need.

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

Add at the end of the file (after `registerRateLimitChannels`, or after
TASK-046/047's `registerEmulatorChannels` if that task landed first — order
doesn't matter, this is a new top-level section):

```go
// ── files.* ──────────────────────────────────────────────────────────
//
// files.commitUpload and files.unwatch are always-local renderer-side
// bookkeeping in the old backend (no fs I/O) — registered as local no-op
// acks in TASK-058, not wired to git-gateway-service. Every other channel
// dispatches through GitGatewayServiceClient's FileIO RPC group
// (SOL-009), which itself resolves local-vs-relay per worktree — wscompat
// never makes that decision; it only forwards worktreeId + params.

// simpleFileOp wires a channel that decodes one JSON arg, issues one
// gRPC call with an rpcTimeout deadline, and returns the response
// verbatim — the shape most of files.*'s RPC-backed channels share.
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

	type readChunkArgs struct {
		WorktreeID  string `json:"worktreeId"`
		Path        string `json:"path"`
		OffsetBytes int64  `json:"offsetBytes"`
		LengthBytes int64  `json:"lengthBytes"`
	}
	simpleFileOp(r, "files.readChunk", func(ctx context.Context, in readChunkArgs) (any, error) {
		resp, err := client.ReadFileChunk(ctx, &gitgatewayv1.ReadFileChunkRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, OffsetBytes: in.OffsetBytes, LengthBytes: in.LengthBytes,
		})
		if err != nil {
			return nil, err // FAILED_PRECONDITION over relay surfaces as-is
		}
		return resp, nil
	})

	type readPreviewArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
		MaxBytes   int64  `json:"maxBytes"`
	}
	simpleFileOp(r, "files.readPreview", func(ctx context.Context, in readPreviewArgs) (any, error) {
		resp, err := client.ReadFilePreview(ctx, &gitgatewayv1.ReadFilePreviewRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, MaxBytes: in.MaxBytes,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// files.write / files.writeBase64 collapse onto WriteFile with an
	// encoding switch, per TASK-049's proto note that this is one RPC
	// with a wire-encoding field, not two RPCs.
	type writeArgs struct {
		WorktreeID    string `json:"worktreeId"`
		Path          string `json:"path"`
		Content       string `json:"content"`
		Base64        bool   `json:"base64"` // true for files.writeBase64
		CreateParents bool   `json:"createParents"`
	}
	writeHandler := func(ctx context.Context, in writeArgs) (any, error) {
		content := []byte(in.Content)
		if in.Base64 {
			decoded, err := base64.StdEncoding.DecodeString(in.Content)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 content: %w", err)
			}
			content = decoded
		}
		resp, err := client.WriteFile(ctx, &gitgatewayv1.WriteFileRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Content: content, CreateParents: in.CreateParents,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	simpleFileOp(r, "files.write", writeHandler)
	simpleFileOp(r, "files.writeBase64", writeHandler)

	type writeChunkArgs struct {
		WorktreeID  string `json:"worktreeId"`
		Path        string `json:"path"`
		OffsetBytes int64  `json:"offsetBytes"`
		Content     string `json:"content"` // always base64 per files.writeBase64Chunk's contract
		IsFinal     bool   `json:"isFinal"`
	}
	simpleFileOp(r, "files.writeBase64Chunk", func(ctx context.Context, in writeChunkArgs) (any, error) {
		content, err := base64.StdEncoding.DecodeString(in.Content)
		if err != nil {
			return nil, fmt.Errorf("decoding base64 content: %w", err)
		}
		resp, err := client.WriteFileChunk(ctx, &gitgatewayv1.WriteFileChunkRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, OffsetBytes: in.OffsetBytes, Content: content, IsFinal: in.IsFinal,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	type createDirArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
		NoClobber  bool   `json:"noClobber"` // true for files.createDirNoClobber
	}
	createDirHandler := func(ctx context.Context, in createDirArgs) (any, error) {
		resp, err := client.CreateDir(ctx, &gitgatewayv1.CreateDirRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Recursive: in.Recursive, NoClobber: in.NoClobber,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	simpleFileOp(r, "files.createDir", createDirHandler)
	simpleFileOp(r, "files.createDirNoClobber", createDirHandler)

	type deleteArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
	}
	simpleFileOp(r, "files.delete", func(ctx context.Context, in deleteArgs) (any, error) {
		if _, err := client.DeleteFile(ctx, &gitgatewayv1.DeleteFileRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Recursive: in.Recursive,
		}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	type searchArgs struct {
		WorktreeID string `json:"worktreeId"`
		Pattern    string `json:"pattern"`
		IsRegex    bool   `json:"isRegex"`
		PathGlob   string `json:"pathGlob"`
		MaxResults int32  `json:"maxResults"`
	}
	simpleFileOp(r, "files.search", func(ctx context.Context, in searchArgs) (any, error) {
		resp, err := client.SearchFiles(ctx, &gitgatewayv1.SearchFilesRequest{
			WorktreeId: in.WorktreeID, Pattern: in.Pattern, IsRegex: in.IsRegex, PathGlob: in.PathGlob, MaxResults: in.MaxResults,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetMatches(), nil
	})

	type listAllArgs struct {
		WorktreeID string `json:"worktreeId"`
		PathGlob   string `json:"pathGlob"`
		MaxResults int32  `json:"maxResults"`
	}
	simpleFileOp(r, "files.listAll", func(ctx context.Context, in listAllArgs) (any, error) {
		resp, err := client.ListAllFiles(ctx, &gitgatewayv1.ListAllFilesRequest{
			WorktreeId: in.WorktreeID, PathGlob: in.PathGlob, MaxResults: in.MaxResults,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetPaths(), nil
	})

	type listMarkdownArgs struct {
		WorktreeID string `json:"worktreeId"`
		MaxResults int32  `json:"maxResults"`
	}
	simpleFileOp(r, "files.listMarkdownDocuments", func(ctx context.Context, in listMarkdownArgs) (any, error) {
		resp, err := client.ListMarkdownDocuments(ctx, &gitgatewayv1.ListMarkdownDocumentsRequest{
			WorktreeId: in.WorktreeID, MaxResults: in.MaxResults,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetPaths(), nil
	})

	// files.rename / files.copy / files.commitUpload / files.unwatch are
	// registered in TASK-058 — kept in a separate task since they don't
	// use simpleFileOp (known-gap error passthrough / no RPC at all).
}
```

Do **not** wire `registerFilesChannels` into `RegisterRealChannels` yet —
that happens in TASK-058, once the rename/copy/bookkeeping handlers this
same function needs are also in place, so the function is registered
complete rather than in two partial edits to the same call site.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
```

Expected: clean build. `registerFilesChannels` is a package-level
function, so Go does not flag it as unused even before TASK-058 calls it
from `RegisterRealChannels`.

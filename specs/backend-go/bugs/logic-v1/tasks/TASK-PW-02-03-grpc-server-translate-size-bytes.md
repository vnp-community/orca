# TASK-PW-02-03: `grpc.Server.ReadDir` translates `SizeBytes` into the wire response

**From Solution:** SOL-PW-02
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-PW-02-02
**Status:** `[ ]` TODO

---

## Context

`Server.ReadDir` (`server.go:529-539`) builds `*gitgatewayv1.DirEntry` from
`domain.DirEntry` but only copies `Name`/`IsDirectory`. This is the
translate step that finally surfaces TASK-PW-02-02's new field on the
wire.

## Changes to make

In `internal/adapter/grpc/server.go`:

```go
func (s *Server) ReadDir(ctx context.Context, req *gitgatewayv1.ReadDirRequest) (*gitgatewayv1.ReadDirResponse, error) {
	entries, err := s.readDir.Execute(ctx, req.GetWorktreeId(), req.GetPath())
	if err != nil {
		return nil, toFileGRPCStatus(err)
	}
	out := make([]*gitgatewayv1.DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, &gitgatewayv1.DirEntry{Name: e.Name, IsDirectory: e.IsDirectory, SizeBytes: e.SizeBytes})
	}
	return &gitgatewayv1.ReadDirResponse{Entries: out}, nil
}
```

Also update `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git_test.go`'s
existing `files.readDir` test to assert `SizeBytes`/`sizeBytes` appears on
the returned entry — this channel is a pure passthrough
(`registerFilesChannels`, `channels_git.go:702`), so no production code
change is needed there, only the regression-guard assertion.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/adapter/grpc/... -run TestReadDir -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestFilesReadDir -v
```

Expected: clean build; a `ReadDir` call against a fixture with a known
file size returns that exact `size_bytes` on the wire response, and the
wscompat passthrough test confirms it survives the gateway hop unchanged.

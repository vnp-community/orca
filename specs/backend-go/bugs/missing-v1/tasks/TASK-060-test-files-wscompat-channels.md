# TASK-060: Test `files.*` wscompat channels

**From Solution:** SOL-009 (Test plan — `wscompat/channels_test.go` items)
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-058
**Status:** `[partial]` 4 of 18 `files.*` channels covered in the new `channels_git_test.go` (`files.read`, `files.writeBase64` incl. base64-decode-then-forward, `files.rename` incl. known-gap error passthrough, `files.commitUpload` local no-op ack). NOT covered: the remaining 14 channels (`files.stat`/`readDir`/`readChunk`/`readPreview`/`write`/`writeBase64Chunk`/`createDir`/`createDirNoClobber`/`delete`/`search`/`listAll`/`listMarkdownDocuments`/`copy`/`unwatch`) and the specific "invalid base64 rejected before any RPC call" regression guard this task calls out. `go test` passes for what's written.

---

## Context

One test per `files.*` channel using a fake `GitGatewayServiceClient`
(mirroring the existing `registerGitChannels`/`registerAnnotationChannels`
test pattern in this file), plus two contract-specific regression guards:
`files.writeBase64` rejecting invalid base64 before any RPC call, and
`files.write`/`files.writeBase64` both resolving to `WriteFile` — guarding
against the two-channels-one-RPC collapse (TASK-057) drifting apart.

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`

Extend the existing fake gRPC client used by this file's git-channel
tests (or add a new `fakeGitGatewayServiceClient` covering the file RPCs
if the existing fake doesn't already implement the full interface) with
recording fields for the new methods, e.g.:

```go
type fakeGitGatewayServiceClient struct {
	gitgatewayv1.GitGatewayServiceClient // embed to satisfy the interface; override what's tested

	readFileCalls  []*gitgatewayv1.ReadFileRequest
	readFileResp   *gitgatewayv1.ReadFileResponse
	readFileErr    error

	writeFileCalls []*gitgatewayv1.WriteFileRequest
	writeFileResp  *gitgatewayv1.WriteFileResponse
	writeFileErr   error

	renameFileCalls []*gitgatewayv1.RenameFileRequest
	renameFileErr   error
	// ... one calls-slice + resp/err pair per method under test
}

func (f *fakeGitGatewayServiceClient) ReadFile(ctx context.Context, in *gitgatewayv1.ReadFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadFileResponse, error) {
	f.readFileCalls = append(f.readFileCalls, in)
	return f.readFileResp, f.readFileErr
}

func (f *fakeGitGatewayServiceClient) WriteFile(ctx context.Context, in *gitgatewayv1.WriteFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.WriteFileResponse, error) {
	f.writeFileCalls = append(f.writeFileCalls, in)
	return f.writeFileResp, f.writeFileErr
}

func (f *fakeGitGatewayServiceClient) RenameFile(ctx context.Context, in *gitgatewayv1.RenameFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.RenameFileResponse, error) {
	f.renameFileCalls = append(f.renameFileCalls, in)
	return &gitgatewayv1.RenameFileResponse{}, f.renameFileErr
}
// ... one override per RPC exercised below (StatFile, ReadDir, ReadFileChunk,
// ReadFilePreview, WriteFileChunk, CreateDir, DeleteFile, SearchFiles,
// ListAllFiles, ListMarkdownDocuments, CopyFile) — same shape as above.
```

### Representative dispatch test

```go
func TestFilesReadChannel_Success(t *testing.T) {
	client := &fakeGitGatewayServiceClient{readFileResp: &gitgatewayv1.ReadFileResponse{Content: []byte("hi"), Encoding: "utf8"}}
	r := NewRegistry()
	registerFilesChannels(r, client)

	args := []json.RawMessage{[]byte(`{"worktreeId":"wt1","path":"a.txt"}`)}
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.read", args)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.readFileCalls) != 1 || client.readFileCalls[0].GetWorktreeId() != "wt1" {
		t.Errorf("expected 1 ReadFile call with worktreeId=wt1, got %+v", client.readFileCalls)
	}
	_ = result
}
```

Repeat this shape (dispatch, assert exactly one recorded call with the
right request fields, assert the projected/passthrough response) for:
`files.stat`, `files.readDir`, `files.readChunk`, `files.readPreview`,
`files.write`, `files.writeBase64`, `files.writeBase64Chunk`,
`files.createDir`, `files.createDirNoClobber`, `files.delete`,
`files.search`, `files.listAll`, `files.listMarkdownDocuments`,
`files.rename`, `files.copy`, `files.commitUpload`, `files.unwatch`
(the last two dispatch with **no** fake client calls recorded at all,
since they never reach `client`).

### `files.writeBase64` invalid-base64 regression guard

```go
func TestFilesWriteBase64Channel_InvalidBase64_NoRPCCall(t *testing.T) {
	client := &fakeGitGatewayServiceClient{}
	r := NewRegistry()
	registerFilesChannels(r, client)

	args := []json.RawMessage{[]byte(`{"worktreeId":"wt1","path":"a.bin","content":"not-valid-base64!!","base64":true}`)}
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.writeBase64", args)
	if err == nil {
		t.Fatal("expected a decode error")
	}
	if len(client.writeFileCalls) != 0 {
		t.Errorf("expected zero WriteFile calls on decode failure, got %v", client.writeFileCalls)
	}
}
```

### `files.write`/`files.writeBase64` both resolve to `WriteFile` — collapse regression guard

```go
func TestFilesWriteChannels_BothResolveToWriteFileRPC(t *testing.T) {
	client := &fakeGitGatewayServiceClient{writeFileResp: &gitgatewayv1.WriteFileResponse{BytesWritten: 2}}
	r := NewRegistry()
	registerFilesChannels(r, client)

	plainArgs := []json.RawMessage{[]byte(`{"worktreeId":"wt1","path":"a.txt","content":"hi","base64":false}`)}
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.write", plainArgs); err != nil {
		t.Fatal(err)
	}

	b64Args := []json.RawMessage{[]byte(`{"worktreeId":"wt1","path":"b.bin","content":"aGk=","base64":true}`)}
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.writeBase64", b64Args); err != nil {
		t.Fatal(err)
	}

	if len(client.writeFileCalls) != 2 {
		t.Fatalf("expected both channels to call WriteFile, got %d calls", len(client.writeFileCalls))
	}
	if string(client.writeFileCalls[0].GetContent()) != "hi" {
		t.Errorf("files.write: expected raw content passthrough, got %q", client.writeFileCalls[0].GetContent())
	}
	if string(client.writeFileCalls[1].GetContent()) != "hi" {
		t.Errorf("files.writeBase64: expected decoded content %q, got %q", "hi", client.writeFileCalls[1].GetContent())
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run TestFiles -v -count=1
go vet ./internal/adapter/wscompat/...
```

Expected: all `files.*` channel tests pass, including both regression
guards.

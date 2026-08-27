# TASK-056: Wire File I/O RPCs into `adapter/grpc.Server` and `cmd/server/main.go`'s composition root

**From Solution:** SOL-009 (implied by the proto/usecase split — this service's existing `Server` translates every RPC to a usecase call, same shape file I/O needs)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go`, `backend-go/services/git-gateway-service/cmd/server/main.go`
**Depends on:** TASK-049, TASK-050, TASK-051, TASK-052, TASK-053, TASK-054, TASK-055
**Status:** `[x]` DONE — `Server` extended with all 14 files.* usecase fields/params and translation methods; `main.go` composition root wires `localfs.New()` + the existing `RelayExecutor` (now also satisfying `FilesystemExecutor`) into every new usecase constructor. `go build`/`go vet`/`go test` clean for the whole service.

---

## Context

`Server` (in `internal/adapter/grpc/server.go`) implements
`gitgatewayv1.GitGatewayServiceServer` by holding one usecase pointer per
RPC and translating request/response messages — no business logic here,
per the file's own package doc comment. `git.proto`'s new RPCs
(TASK-049) need the same treatment, or they will silently return
`Unimplemented` (the embedded `UnimplementedGitGatewayServiceServer`'s
default), which would make TASK-049 through TASK-055's work unreachable
from wscompat.

`cmd/server/main.go`'s composition root constructs `localgit.New()`,
`grpcclient.NewRelayExecutor(infraFleetClient)`, and one usecase per RPC,
then passes them all into `grpc.New(...)`. This task extends that same
wiring with `localfs.New()`, the existing `RelayExecutor` (now also
implementing `FilesystemExecutor`, TASK-051), and one new usecase
constructor call per RPC from TASK-052 through TASK-055.

---

## Changes to make

### Step 1: Extend `Server` struct and `New`

**File:** `internal/adapter/grpc/server.go`

Add fields and constructor parameters (append after `generateCommitMessage`):

```go
type Server struct {
	gitgatewayv1.UnimplementedGitGatewayServiceServer

	getStatus             *usecase.GetStatus
	getDiff               *usecase.GetDiff
	commit                *usecase.Commit
	push                  *usecase.Push
	pull                  *usecase.Pull
	generateCommitMessage *usecase.GenerateCommitMessage

	readFile               *usecase.ReadFileUseCase
	readFileChunk          *usecase.ReadFileChunkUseCase
	readFilePreview        *usecase.ReadFilePreviewUseCase
	readDir                *usecase.ReadDirUseCase
	writeFile              *usecase.WriteFileUseCase
	writeFileChunk         *usecase.WriteFileChunkUseCase
	createDir              *usecase.CreateDirUseCase
	deleteFile             *usecase.DeleteFileUseCase
	statFile               *usecase.StatFileUseCase
	searchFiles            *usecase.SearchFilesUseCase
	listAllFiles           *usecase.ListAllFilesUseCase
	listMarkdownDocuments  *usecase.ListMarkdownDocumentsUseCase
	renameFile             *usecase.RenameFileUseCase
	copyFile               *usecase.CopyFileUseCase
}

func New(
	getStatus *usecase.GetStatus,
	getDiff *usecase.GetDiff,
	commit *usecase.Commit,
	push *usecase.Push,
	pull *usecase.Pull,
	generateCommitMessage *usecase.GenerateCommitMessage,
	readFile *usecase.ReadFileUseCase,
	readFileChunk *usecase.ReadFileChunkUseCase,
	readFilePreview *usecase.ReadFilePreviewUseCase,
	readDir *usecase.ReadDirUseCase,
	writeFile *usecase.WriteFileUseCase,
	writeFileChunk *usecase.WriteFileChunkUseCase,
	createDir *usecase.CreateDirUseCase,
	deleteFile *usecase.DeleteFileUseCase,
	statFile *usecase.StatFileUseCase,
	searchFiles *usecase.SearchFilesUseCase,
	listAllFiles *usecase.ListAllFilesUseCase,
	listMarkdownDocuments *usecase.ListMarkdownDocumentsUseCase,
	renameFile *usecase.RenameFileUseCase,
	copyFile *usecase.CopyFileUseCase,
) *Server {
	return &Server{
		getStatus:             getStatus,
		getDiff:               getDiff,
		commit:                commit,
		push:                  push,
		pull:                  pull,
		generateCommitMessage: generateCommitMessage,
		readFile:              readFile,
		readFileChunk:         readFileChunk,
		readFilePreview:       readFilePreview,
		readDir:               readDir,
		writeFile:             writeFile,
		writeFileChunk:        writeFileChunk,
		createDir:             createDir,
		deleteFile:            deleteFile,
		statFile:              statFile,
		searchFiles:           searchFiles,
		listAllFiles:          listAllFiles,
		listMarkdownDocuments: listMarkdownDocuments,
		renameFile:            renameFile,
		copyFile:              copyFile,
	}
}
```

### Step 2: Add translation methods

Append to `server.go`:

```go
func (s *Server) ReadFile(ctx context.Context, req *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error) {
	content, err := s.readFile.Execute(ctx, req.GetWorktreeId(), req.GetPath())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ReadFileResponse{Content: content, Encoding: "utf8"}, nil
}

func (s *Server) ReadFileChunk(ctx context.Context, req *gitgatewayv1.ReadFileChunkRequest) (*gitgatewayv1.ReadFileChunkResponse, error) {
	content, err := s.readFileChunk.Execute(ctx, req.GetWorktreeId(), req.GetPath(), req.GetOffsetBytes(), req.GetLengthBytes())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ReadFileChunkResponse{Content: content, Eof: int64(len(content)) < req.GetLengthBytes()}, nil
}

func (s *Server) ReadFilePreview(ctx context.Context, req *gitgatewayv1.ReadFilePreviewRequest) (*gitgatewayv1.ReadFilePreviewResponse, error) {
	content, truncated, err := s.readFilePreview.Execute(ctx, req.GetWorktreeId(), req.GetPath(), req.GetMaxBytes())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ReadFilePreviewResponse{Content: content, Truncated: truncated}, nil
}

func (s *Server) ReadDir(ctx context.Context, req *gitgatewayv1.ReadDirRequest) (*gitgatewayv1.ReadDirResponse, error) {
	entries, err := s.readDir.Execute(ctx, req.GetWorktreeId(), req.GetPath())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*gitgatewayv1.DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, &gitgatewayv1.DirEntry{Name: e.Name, IsDirectory: e.IsDirectory})
	}
	return &gitgatewayv1.ReadDirResponse{Entries: out}, nil
}

func (s *Server) WriteFile(ctx context.Context, req *gitgatewayv1.WriteFileRequest) (*gitgatewayv1.WriteFileResponse, error) {
	n, err := s.writeFile.Execute(ctx, req.GetWorktreeId(), req.GetPath(), req.GetContent(), req.GetCreateParents())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.WriteFileResponse{BytesWritten: n}, nil
}

func (s *Server) WriteFileChunk(ctx context.Context, req *gitgatewayv1.WriteFileChunkRequest) (*gitgatewayv1.WriteFileChunkResponse, error) {
	n, err := s.writeFileChunk.Execute(ctx, req.GetWorktreeId(), req.GetPath(), req.GetOffsetBytes(), req.GetContent(), req.GetIsFinal())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.WriteFileChunkResponse{BytesWritten: n}, nil
}

func (s *Server) CreateDir(ctx context.Context, req *gitgatewayv1.CreateDirRequest) (*gitgatewayv1.CreateDirResponse, error) {
	if err := s.createDir.Execute(ctx, req.GetWorktreeId(), req.GetPath(), req.GetRecursive(), req.GetNoClobber()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CreateDirResponse{}, nil
}

func (s *Server) DeleteFile(ctx context.Context, req *gitgatewayv1.DeleteFileRequest) (*emptypb.Empty, error) {
	if err := s.deleteFile.Execute(ctx, req.GetWorktreeId(), req.GetPath(), req.GetRecursive()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) StatFile(ctx context.Context, req *gitgatewayv1.StatFileRequest) (*gitgatewayv1.StatFileResponse, error) {
	result, err := s.statFile.Execute(ctx, req.GetWorktreeId(), req.GetPath())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.StatFileResponse{
		Exists:           result.Exists,
		IsDirectory:      result.IsDirectory,
		SizeBytes:        result.SizeBytes,
		ModifiedAtUnixMs: result.ModifiedAtUnixMs,
	}, nil
}

func (s *Server) SearchFiles(ctx context.Context, req *gitgatewayv1.SearchFilesRequest) (*gitgatewayv1.SearchFilesResponse, error) {
	matches, err := s.searchFiles.Execute(ctx, req.GetWorktreeId(), domain.SearchOptions{
		Pattern: req.GetPattern(), IsRegex: req.GetIsRegex(), PathGlob: req.GetPathGlob(), MaxResults: int(req.GetMaxResults()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*gitgatewayv1.SearchMatch, 0, len(matches))
	for _, m := range matches {
		out = append(out, &gitgatewayv1.SearchMatch{Path: m.Path, Line: int32(m.Line), LineText: m.LineText})
	}
	return &gitgatewayv1.SearchFilesResponse{Matches: out}, nil
}

func (s *Server) ListAllFiles(ctx context.Context, req *gitgatewayv1.ListAllFilesRequest) (*gitgatewayv1.ListAllFilesResponse, error) {
	paths, err := s.listAllFiles.Execute(ctx, req.GetWorktreeId(), req.GetPathGlob(), int(req.GetMaxResults()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ListAllFilesResponse{Paths: paths}, nil
}

func (s *Server) ListMarkdownDocuments(ctx context.Context, req *gitgatewayv1.ListMarkdownDocumentsRequest) (*gitgatewayv1.ListMarkdownDocumentsResponse, error) {
	paths, err := s.listMarkdownDocuments.Execute(ctx, req.GetWorktreeId(), int(req.GetMaxResults()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ListMarkdownDocumentsResponse{Paths: paths}, nil
}

func (s *Server) RenameFile(ctx context.Context, req *gitgatewayv1.RenameFileRequest) (*gitgatewayv1.RenameFileResponse, error) {
	if err := s.renameFile.Execute(ctx, req.GetWorktreeId(), req.GetFromPath(), req.GetToPath()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.RenameFileResponse{}, nil
}

func (s *Server) CopyFile(ctx context.Context, req *gitgatewayv1.CopyFileRequest) (*gitgatewayv1.CopyFileResponse, error) {
	if err := s.copyFile.Execute(ctx, req.GetWorktreeId(), req.GetFromPath(), req.GetToPath()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CopyFileResponse{}, nil
}
```

Add `"google.golang.org/protobuf/types/known/emptypb"` to the import block
(`DeleteFile` returns `google.protobuf.Empty` per TASK-049's proto).

### Step 3: Extend `cmd/server/main.go`'s composition root

Find where `localgit.New()`, `grpcclient.NewRelayExecutor(...)`, and each
git usecase (`usecase.NewGetStatus(...)` etc.) are constructed and passed
into `grpc.New(...)`. Add, in the same place:

```go
localFS := localfs.New()
relayFS := relayExecutor // the same *grpcclient.RelayExecutor already constructed for git.*, now also satisfying usecase.FilesystemExecutor

readFile := usecase.NewReadFileUseCase(connResolver, localFS, relayFS)
readFileChunk := usecase.NewReadFileChunkUseCase(connResolver, localFS)
readFilePreview := usecase.NewReadFilePreviewUseCase(connResolver, localFS, relayFS)
readDir := usecase.NewReadDirUseCase(connResolver, localFS, relayFS)
writeFile := usecase.NewWriteFileUseCase(connResolver, localFS, relayFS)
writeFileChunk := usecase.NewWriteFileChunkUseCase(connResolver, localFS, relayFS)
createDir := usecase.NewCreateDirUseCase(connResolver, localFS, relayFS)
deleteFile := usecase.NewDeleteFileUseCase(connResolver, localFS, relayFS)
statFile := usecase.NewStatFileUseCase(connResolver, localFS, relayFS)
searchFiles := usecase.NewSearchFilesUseCase(connResolver, localFS, relayFS)
listAllFiles := usecase.NewListAllFilesUseCase(connResolver, localFS, relayFS)
listMarkdownDocuments := usecase.NewListMarkdownDocumentsUseCase(connResolver, localFS, relayFS)
renameFile := usecase.NewRenameFileUseCase(connResolver, localFS)
copyFile := usecase.NewCopyFileUseCase(connResolver, localFS)

server := grpc.New(
	getStatus, getDiff, commit, push, pull, generateCommitMessage,
	readFile, readFileChunk, readFilePreview, readDir, writeFile, writeFileChunk,
	createDir, deleteFile, statFile, searchFiles, listAllFiles, listMarkdownDocuments,
	renameFile, copyFile,
)
```

Adjust variable names (`connResolver`, `relayExecutor`) to whatever the
existing composition root actually calls them — this task only adds
lines, it does not rename existing wiring.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./...
go vet ./...
```

Expected: clean build across the whole service — this is the point where
every earlier `files.*` task (TASK-049–TASK-055) becomes reachable end to
end via gRPC, not just compiling in isolation.

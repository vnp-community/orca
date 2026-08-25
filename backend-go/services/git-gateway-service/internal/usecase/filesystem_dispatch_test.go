package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// fakeFilesystemExecutor is an in-memory FilesystemExecutor +
// LocalOnlyFilesystemExecutor — records which method was called so tests
// can assert dispatch routing, mirroring fakeGitExecutor's pattern above.
type fakeFilesystemExecutor struct {
	calledReadFile   bool
	calledWriteFile  bool
	calledRename     bool
	calledCopy       bool
	calledSearch     bool
	calledGlob       bool
	gotRepoPath      string
	readFileContent  []byte
	readFileErr      error
}

func (f *fakeFilesystemExecutor) ReadFile(ctx context.Context, repoPath, relPath string) ([]byte, error) {
	f.calledReadFile = true
	f.gotRepoPath = repoPath
	if f.readFileErr != nil {
		return nil, f.readFileErr
	}
	if f.readFileContent != nil {
		return f.readFileContent, nil
	}
	return []byte("content"), nil
}

func (f *fakeFilesystemExecutor) ReadFilePreview(ctx context.Context, repoPath, relPath string, maxBytes int64) ([]byte, bool, error) {
	return []byte("preview"), false, nil
}

func (f *fakeFilesystemExecutor) ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error) {
	return []domain.DirEntry{{Name: "a.txt"}}, nil
}

func (f *fakeFilesystemExecutor) WriteFile(ctx context.Context, repoPath, relPath string, content []byte, createParents bool) (int64, error) {
	f.calledWriteFile = true
	f.gotRepoPath = repoPath
	return int64(len(content)), nil
}

func (f *fakeFilesystemExecutor) WriteFileChunk(ctx context.Context, repoPath, relPath string, offsetBytes int64, content []byte, isFinal bool) (int64, error) {
	return int64(len(content)), nil
}

func (f *fakeFilesystemExecutor) CreateDir(ctx context.Context, repoPath, relPath string, recursive, noClobber bool) error {
	return nil
}

func (f *fakeFilesystemExecutor) Delete(ctx context.Context, repoPath, relPath string, recursive bool) error {
	return nil
}

func (f *fakeFilesystemExecutor) Stat(ctx context.Context, repoPath, relPath string) (domain.FileStat, error) {
	return domain.FileStat{Exists: true}, nil
}

func (f *fakeFilesystemExecutor) Search(ctx context.Context, repoPath string, opts domain.SearchOptions) ([]domain.SearchMatch, error) {
	f.calledSearch = true
	return []domain.SearchMatch{{Path: "a.txt", Line: 1}}, nil
}

func (f *fakeFilesystemExecutor) Glob(ctx context.Context, repoPath, pattern string, maxResults int) ([]string, error) {
	f.calledGlob = true
	return []string{"a.txt", "b.md"}, nil
}

func (f *fakeFilesystemExecutor) Rename(ctx context.Context, repoPath, fromRel, toRel string) error {
	f.calledRename = true
	return nil
}

func (f *fakeFilesystemExecutor) Copy(ctx context.Context, repoPath, fromRel, toRel string) error {
	f.calledCopy = true
	return nil
}

func TestReadFileUseCase_RoutesByConnectionState(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	relay := &fakeFilesystemExecutor{}
	uc := NewReadFileUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/repo"}}, local, relay)

	got, err := uc.Execute(context.Background(), "wt1", "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relay.calledReadFile || local.calledReadFile {
		t.Error("expected ReadFile to route to relay when Connected=true")
	}
	if string(got) != "content" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestReadFileChunkUseCase_Connected_ReturnsNotSupported(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	uc := NewReadFileChunkUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: true}}, local)

	_, err := uc.Execute(context.Background(), "wt1", "a.txt", 0, 10)
	if !errors.Is(err, ErrChunkedReadNotSupportedRemote) {
		t.Fatalf("expected ErrChunkedReadNotSupportedRemote, got %v", err)
	}
	if local.calledReadFile {
		t.Error("expected local executor NOT to be called for a relay-connected worktree")
	}
}

func TestReadFileChunkUseCase_NotConnected_SlicesContent(t *testing.T) {
	local := &fakeFilesystemExecutor{readFileContent: []byte("0123456789")}
	uc := NewReadFileChunkUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}, local)

	got, err := uc.Execute(context.Background(), "wt1", "a.txt", 2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "234" {
		t.Errorf("expected chunk %q, got %q", "234", got)
	}
}

func TestWriteFileUseCase_RoutesByConnectionState(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	relay := &fakeFilesystemExecutor{}
	uc := NewWriteFileUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}, local, relay)

	n, err := uc.Execute(context.Background(), "wt1", "a.txt", []byte("hello"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledWriteFile || relay.calledWriteFile {
		t.Error("expected WriteFile to route to local when Connected=false")
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
}

func TestSearchFilesUseCase_DelegatesToExecutor(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	uc := NewSearchFilesUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}, local, local)

	_, err := uc.Execute(context.Background(), "wt1", domain.SearchOptions{Pattern: "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledSearch {
		t.Error("expected Search to be called")
	}
}

func TestListMarkdownDocumentsUseCase_FiltersToMarkdown(t *testing.T) {
	local := &fakeFilesystemExecutor{} // Glob returns {"a.txt", "b.md"}
	uc := NewListMarkdownDocumentsUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}, local, local)

	got, err := uc.Execute(context.Background(), "wt1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "b.md" {
		t.Errorf("expected only b.md, got %+v", got)
	}
}

func TestRenameFileUseCase_Connected_ReturnsNotSupported(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	uc := NewRenameFileUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: true}}, local)

	err := uc.Execute(context.Background(), "wt1", "a.txt", "b.txt")
	if !errors.Is(err, ErrFileOpNotSupportedOverRelay) {
		t.Fatalf("expected ErrFileOpNotSupportedOverRelay, got %v", err)
	}
	if local.calledRename {
		t.Error("expected local executor NOT to be called for a relay-connected worktree")
	}
}

func TestRenameFileUseCase_NotConnected_CallsLocal(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	uc := NewRenameFileUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}, local)

	if err := uc.Execute(context.Background(), "wt1", "a.txt", "b.txt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !local.calledRename {
		t.Error("expected local Rename to be called")
	}
}

func TestCopyFileUseCase_Connected_ReturnsNotSupported(t *testing.T) {
	local := &fakeFilesystemExecutor{}
	uc := NewCopyFileUseCase(&fakeConnectionResolver{conn: ResolvedConnection{Connected: true}}, local)

	err := uc.Execute(context.Background(), "wt1", "a.txt", "b.txt")
	if !errors.Is(err, ErrFileOpNotSupportedOverRelay) {
		t.Fatalf("expected ErrFileOpNotSupportedOverRelay, got %v", err)
	}
}

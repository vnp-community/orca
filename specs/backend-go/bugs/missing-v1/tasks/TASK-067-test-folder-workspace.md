# TASK-067: Test `FolderWorkspace` usecases, repository, and wscompat channels

**From Solution:** SOL-010 (Test plan)
**Priority:** P1
**Service:** `project-service`, `api-gateway`
**File:** `backend-go/services/project-service/internal/usecase/create_folder_workspace_test.go`, `get_folder_workspace_path_status_test.go`, `update_folder_workspace_test.go`, `delete_folder_workspace_test.go`, `backend-go/services/project-service/internal/adapter/postgres/folder_workspace_repository_test.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-064, TASK-065, TASK-066
**Status:** `[ ]` TODO

---

## Changes to make

### `project-service` usecase tests

**File:** `internal/usecase/create_folder_workspace_test.go`

```go
func TestFolderWorkspaceUseCase_CreateThenList(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	id := Identity{TenantID: "t1", UserID: "u1"}

	created, err := uc.Create(context.Background(), id, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: ""})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "x" { // defaults to filepath.Base(path)
		t.Errorf("expected default name %q, got %q", "x", created.Name)
	}

	list, err := uc.List(context.Background(), id)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v %+v", err, list)
	}
}

func TestFolderWorkspaceUseCase_Create_DuplicatePath(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	id := Identity{TenantID: "t1", UserID: "u1"}
	in := CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"}

	if _, err := uc.Create(context.Background(), id, in); err != nil {
		t.Fatal(err)
	}
	_, err := uc.Create(context.Background(), id, in)
	if !errors.Is(err, domain.ErrPathAlreadyRegistered) {
		t.Errorf("expected ErrPathAlreadyRegistered, got %v", err)
	}
}

func TestFolderWorkspaceUseCase_Create_RelativePathRejected(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	id := Identity{TenantID: "t1", UserID: "u1"}

	_, err := uc.Create(context.Background(), id, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "relative/path"})
	if !errors.Is(err, domain.ErrPathNotAbsolute) {
		t.Errorf("expected ErrPathNotAbsolute, got %v", err)
	}
}
```

**File:** `internal/usecase/get_folder_workspace_path_status_test.go`

```go
func TestGetPathStatus_Available(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	status, err := uc.GetPathStatus(context.Background(), Identity{TenantID: "t1"}, "d1", "/home/new")
	if err != nil || status.Status != domain.PathStatusAvailable {
		t.Fatalf("expected AVAILABLE, got %+v %v", status, err)
	}
}

func TestGetPathStatus_AlreadyFolderWorkspace(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	id := Identity{TenantID: "t1", UserID: "u1"}
	created, _ := uc.Create(context.Background(), id, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"})

	status, err := uc.GetPathStatus(context.Background(), id, "d1", "/home/x")
	if err != nil || status.Status != domain.PathStatusAlreadyFolderWorkspace || status.ExistingID != created.ID {
		t.Fatalf("expected ALREADY_FOLDER_WORKSPACE with id %q, got %+v %v", created.ID, status, err)
	}
}

func TestGetPathStatus_AlreadyRepo(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	repo.repoPathExists = true
	uc := NewFolderWorkspaceUseCase(repo)

	status, err := uc.GetPathStatus(context.Background(), Identity{TenantID: "t1"}, "d1", "/home/repo")
	if err != nil || status.Status != domain.PathStatusAlreadyRepo {
		t.Fatalf("expected ALREADY_REPO, got %+v %v", status, err)
	}
}

func TestGetPathStatus_InvalidPath_NoRepositoryCallMade(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)

	status, err := uc.GetPathStatus(context.Background(), Identity{TenantID: "t1"}, "d1", "relative")
	if err != nil || status.Status != domain.PathStatusInvalid {
		t.Fatalf("expected INVALID, got %+v %v", status, err)
	}
	// Regression guard: no live fs probe attempted, no repo lookup either.
	if repo.findByPathCalls != 0 || repo.repoPathExistsCalls != 0 {
		t.Errorf("expected zero repository calls for an invalid path, got findByPath=%d repoPathExists=%d",
			repo.findByPathCalls, repo.repoPathExistsCalls)
	}
}
```

**File:** `internal/usecase/update_folder_workspace_test.go` / `delete_folder_workspace_test.go`

```go
func TestFolderWorkspaceUseCase_Update_NonOwnerNonAdminRejected(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	owner := Identity{TenantID: "t1", UserID: "owner"}
	created, _ := uc.Create(context.Background(), owner, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"})

	stranger := Identity{TenantID: "t1", UserID: "stranger"}
	_, err := uc.Update(context.Background(), stranger, created.ID, "new-name")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}
```

(Mirror for `Delete`; add a companion "admin can update/delete anyone's"
positive case using an `Identity{IsGlobalAdmin: true}` caller.)

Implement `newFakeFolderWorkspaceRepository()` as a fake in-memory
`usecase.FolderWorkspaceRepository` in this package's existing
`fakes_test.go`, following the `fakeProjectRepository` (or equivalent)
naming convention already in that file. It needs a `repoPathExists bool`
field and `findByPathCalls`/`repoPathExistsCalls` counters for the
"no call" regression guard above.

### `project-service` postgres integration test

**File:** `internal/adapter/postgres/folder_workspace_repository_test.go`

Use this package's existing `testcontainers-go` Postgres test harness
(same one `repo_repository_test.go`/`worktree_repository_test.go` use).
Cover: create-then-get round-trip; a second `Create` with the same
`(tenant_id, dev_server_id, path)` returns `domain.ErrPathAlreadyRegistered`
(asserting the `pgconn.PgError` unique-violation mapping in TASK-063
actually fires against a real Postgres, not just a fake).

### `api-gateway` wscompat channel tests

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`

One test per channel, fake `ProjectServiceClient`, mirroring
`annotation_channels_test.go`'s existing shape (dispatch, assert exactly
one recorded call with the right request fields, assert the response is
passed through). Cover `folderWorkspace.create`, `.update`, `.delete`,
`.list`, `.getPathStatus`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go test ./internal/usecase/... -count=1 -v
go test ./internal/adapter/postgres/... -count=1 -v   # requires Docker for testcontainers-go

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run TestFolderWorkspace -count=1 -v
```

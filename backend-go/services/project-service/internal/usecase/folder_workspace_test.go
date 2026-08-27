package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestFolderWorkspaceUseCase_CreateThenList(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenantAndUser(context.Background(), "t1", "u1")

	created, err := uc.Create(ctx, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: ""})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "x" { // defaults to filepath.Base(path)
		t.Errorf("expected default name %q, got %q", "x", created.Name)
	}
	if created.AddedBy != "u1" {
		t.Errorf("expected added_by %q, got %q", "u1", created.AddedBy)
	}

	list, err := uc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v %+v", err, list)
	}
}

func TestFolderWorkspaceUseCase_Create_DuplicatePath(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenantAndUser(context.Background(), "t1", "u1")
	in := CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"}

	if _, err := uc.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	_, err := uc.Create(ctx, in)
	if !errors.Is(err, domain.ErrPathAlreadyRegistered) {
		t.Errorf("expected ErrPathAlreadyRegistered, got %v", err)
	}
	assertAppError(t, err, apperrors.KindAlreadyExists, "PROJECT_FOLDER_WORKSPACE_PATH_TAKEN")
}

func TestFolderWorkspaceUseCase_Create_RelativePathRejected(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenantAndUser(context.Background(), "t1", "u1")

	_, err := uc.Create(ctx, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "relative/path"})
	if !errors.Is(err, domain.ErrPathNotAbsolute) {
		t.Errorf("expected ErrPathNotAbsolute, got %v", err)
	}
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_FOLDER_WORKSPACE_PATH_NOT_ABSOLUTE")
}

func TestFolderWorkspaceUseCase_Create_NoTenant(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)

	_, err := uc.Create(context.Background(), CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x"})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_TENANT")
}

func TestGetPathStatus_Available(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenant(context.Background(), "t1")

	status, err := uc.GetPathStatus(ctx, "d1", "/home/new")
	if err != nil || status.Status != domain.PathStatusAvailable {
		t.Fatalf("expected AVAILABLE, got %+v %v", status, err)
	}
}

func TestGetPathStatus_AlreadyFolderWorkspace(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenantAndUser(context.Background(), "t1", "u1")
	created, err := uc.Create(ctx, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}

	status, err := uc.GetPathStatus(ctx, "d1", "/home/x")
	if err != nil || status.Status != domain.PathStatusAlreadyFolderWorkspace || status.ExistingID != created.ID {
		t.Fatalf("expected ALREADY_FOLDER_WORKSPACE with id %q, got %+v %v", created.ID, status, err)
	}
}

func TestGetPathStatus_AlreadyRepo(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	repo.repoPathExists = true
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenant(context.Background(), "t1")

	status, err := uc.GetPathStatus(ctx, "d1", "/home/repo")
	if err != nil || status.Status != domain.PathStatusAlreadyRepo {
		t.Fatalf("expected ALREADY_REPO, got %+v %v", status, err)
	}
}

func TestGetPathStatus_InvalidPath_NoRepositoryCallMade(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenant(context.Background(), "t1")

	status, err := uc.GetPathStatus(ctx, "d1", "relative")
	if err != nil || status.Status != domain.PathStatusInvalid {
		t.Fatalf("expected INVALID, got %+v %v", status, err)
	}
	// Regression guard: no live fs probe attempted, no repo lookup either.
	if repo.findByPathCalls != 0 || repo.repoPathExistsCalls != 0 {
		t.Errorf("expected zero repository calls for an invalid path, got findByPath=%d repoPathExists=%d",
			repo.findByPathCalls, repo.repoPathExistsCalls)
	}
}

func TestFolderWorkspaceUseCase_Update_NonOwnerRejected(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ownerCtx := withTenantAndUser(context.Background(), "t1", "owner")
	created, err := uc.Create(ownerCtx, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}

	strangerCtx := withTenantAndUser(context.Background(), "t1", "stranger")
	_, err = uc.Update(strangerCtx, created.ID, "new-name")
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_FOLDER_WORKSPACE_NOT_OWNER")
}

func TestFolderWorkspaceUseCase_Update_OwnerAllowed(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ownerCtx := withTenantAndUser(context.Background(), "t1", "owner")
	created, err := uc.Create(ownerCtx, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := uc.Update(ownerCtx, created.ID, "renamed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "renamed" {
		t.Errorf("expected name %q, got %q", "renamed", updated.Name)
	}
}

func TestFolderWorkspaceUseCase_Update_NotFound(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ctx := withTenantAndUser(context.Background(), "t1", "u1")

	_, err := uc.Update(ctx, "missing-id", "new-name")
	assertAppError(t, err, apperrors.KindNotFound, "PROJECT_FOLDER_WORKSPACE_NOT_FOUND")
}

func TestFolderWorkspaceUseCase_Delete_NonOwnerRejected(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ownerCtx := withTenantAndUser(context.Background(), "t1", "owner")
	created, err := uc.Create(ownerCtx, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}

	strangerCtx := withTenantAndUser(context.Background(), "t1", "stranger")
	err = uc.Delete(strangerCtx, created.ID)
	assertAppError(t, err, apperrors.KindPermissionDenied, "PROJECT_FOLDER_WORKSPACE_NOT_OWNER")
}

func TestFolderWorkspaceUseCase_Delete_OwnerAllowed(t *testing.T) {
	repo := newFakeFolderWorkspaceRepository()
	uc := NewFolderWorkspaceUseCase(repo)
	ownerCtx := withTenantAndUser(context.Background(), "t1", "owner")
	created, err := uc.Create(ownerCtx, CreateFolderWorkspaceInput{DevServerID: "d1", Path: "/home/x", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}

	if err := uc.Delete(ownerCtx, created.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list, err := uc.List(ownerCtx)
	if err != nil || len(list) != 0 {
		t.Fatalf("expected empty list after delete, got %v %+v", err, list)
	}
}

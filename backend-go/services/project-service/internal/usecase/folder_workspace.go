// Package usecase — FolderWorkspaceUseCase implements create/update/
// delete/list/getPathStatus for standalone, non-git folder workspaces.
//
// Standard CRUD shape, no relay/dispatch branching — per BUG-010's
// dispatch-model finding, this namespace is Postgres-only. Authorization:
// per project-service.md §9's posture ("CreateProject requires only
// authentication"), folder_workspaces rows have no membership model of
// their own — any authenticated tenant member can create/list. Update/
// Delete additionally require the caller to be the row's original
// added_by. A global-admin override is deliberately NOT implemented here:
// callerGlobalRole (authorization.go) is a documented, always-"" known gap
// until claim propagation from api-gateway lands — adding a second,
// unreachable admin-override path for this namespace would just be more
// surface pretending to be wired.
package usecase

import (
	"cmp"
	"context"
	"errors"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// FolderWorkspaceUseCase groups every folder-workspace operation behind
// one struct — mirrors this package's single-usecase-per-file convention
// for its other simple CRUD entities being split across sibling files
// (create_folder_workspace.go-equivalent methods below) while sharing one
// repository dependency.
type FolderWorkspaceUseCase struct {
	repo FolderWorkspaceRepository
}

func NewFolderWorkspaceUseCase(repo FolderWorkspaceRepository) *FolderWorkspaceUseCase {
	return &FolderWorkspaceUseCase{repo: repo}
}

type CreateFolderWorkspaceInput struct {
	DevServerID string
	Path        string
	Name        string
	// ProjectGroupID is "" for no group — the sidebar's own optional
	// organizational concept, layered onto this dev-server-scoped entity.
	ProjectGroupID string
}

// Create validates Path is absolute, then delegates to the repository. The
// repository's UNIQUE(tenant_id, dev_server_id, path) constraint is the
// real conflict guard; GetPathStatus (below) is a pre-flight convenience
// the frontend calls separately — this usecase still surfaces the same
// domain.ErrPathAlreadyRegistered on a constraint violation, not a generic
// error.
func (uc *FolderWorkspaceUseCase) Create(ctx context.Context, in CreateFolderWorkspaceInput) (domain.FolderWorkspace, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}
	if !filepath.IsAbs(in.Path) {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_FOLDER_WORKSPACE_PATH_NOT_ABSOLUTE", domain.ErrPathNotAbsolute.Error(), domain.ErrPathNotAbsolute)
	}
	cleanPath := filepath.Clean(in.Path)

	fw, err := domain.NewFolderWorkspace(uuid.NewString(), tenantID, in.DevServerID, cleanPath, cmp.Or(in.Name, filepath.Base(cleanPath)), userID, in.ProjectGroupID)
	if err != nil {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_FOLDER_WORKSPACE_INVALID", err.Error(), err)
	}

	created, err := uc.repo.Create(ctx, fw)
	if err != nil {
		if errors.Is(err, domain.ErrPathAlreadyRegistered) {
			return domain.FolderWorkspace{}, apperrors.New(apperrors.KindAlreadyExists, "PROJECT_FOLDER_WORKSPACE_PATH_TAKEN", err.Error(), err)
		}
		// Why ErrProjectGroupNotFound here: the repository maps a
		// project_group_id foreign-key violation to this sentinel — no
		// app-level pre-check of group existence, same trust-the-FK
		// pattern AddRepo already uses for project_id.
		if errors.Is(err, domain.ErrProjectGroupNotFound) {
			return domain.FolderWorkspace{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_GROUP_NOT_FOUND", err.Error(), err)
		}
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindInternal, "PROJECT_FOLDER_WORKSPACE_CREATE_FAILED", "failed to persist folder workspace", err)
	}
	return created, nil
}

// Update renames a folder workspace — the only mutable field, per
// project.proto's UpdateFolderWorkspaceRequest doc comment. Rejects callers
// who aren't the folder workspace's original added_by.
func (uc *FolderWorkspaceUseCase) Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error) {
	if _, err := uc.requireOwnedFolderWorkspace(ctx, id); err != nil {
		return domain.FolderWorkspace{}, err
	}

	updated, err := uc.repo.Update(ctx, id, name)
	if err != nil {
		if errors.Is(err, domain.ErrFolderWorkspaceNotFound) {
			return domain.FolderWorkspace{}, apperrors.New(apperrors.KindNotFound, "PROJECT_FOLDER_WORKSPACE_NOT_FOUND", err.Error(), err)
		}
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindInternal, "PROJECT_FOLDER_WORKSPACE_UPDATE_FAILED", "failed to update folder workspace", err)
	}
	return updated, nil
}

// Delete rejects callers who aren't the folder workspace's original
// added_by, same as Update.
func (uc *FolderWorkspaceUseCase) Delete(ctx context.Context, id string) error {
	if _, err := uc.requireOwnedFolderWorkspace(ctx, id); err != nil {
		return err
	}

	if err := uc.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrFolderWorkspaceNotFound) {
			return apperrors.New(apperrors.KindNotFound, "PROJECT_FOLDER_WORKSPACE_NOT_FOUND", err.Error(), err)
		}
		return apperrors.New(apperrors.KindInternal, "PROJECT_FOLDER_WORKSPACE_DELETE_FAILED", "failed to delete folder workspace", err)
	}
	return nil
}

// requireOwnedFolderWorkspace loads id and verifies the acting user is its
// added_by — shared by Update/Delete, the two mutations this namespace
// gates beyond plain authentication.
func (uc *FolderWorkspaceUseCase) requireOwnedFolderWorkspace(ctx context.Context, id string) (domain.FolderWorkspace, error) {
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	existing, err := uc.repo.Get(ctx, id)
	if err != nil {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindInternal, "PROJECT_FOLDER_WORKSPACE_GET_FAILED", "failed to load folder workspace", err)
	}
	if existing == nil {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindNotFound, "PROJECT_FOLDER_WORKSPACE_NOT_FOUND", domain.ErrFolderWorkspaceNotFound.Error(), domain.ErrFolderWorkspaceNotFound)
	}
	if existing.AddedBy != userID {
		return domain.FolderWorkspace{}, apperrors.New(apperrors.KindPermissionDenied, "PROJECT_FOLDER_WORKSPACE_NOT_OWNER", "only the folder workspace's creator may modify it", nil)
	}
	return *existing, nil
}

// List returns every folder workspace registered for the caller's tenant.
func (uc *FolderWorkspaceUseCase) List(ctx context.Context) ([]domain.FolderWorkspace, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	list, err := uc.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_FOLDER_WORKSPACE_LIST_FAILED", "failed to list folder workspaces", err)
	}
	return list, nil
}

// GetPathStatus answers purely from this service's own tables — NOT a live
// filesystem probe. See domain.PathStatus*'s doc comment and SOL-010's
// design note before changing that assumption. An invalid (non-absolute)
// path short-circuits before any repository call — see
// TestGetPathStatus_InvalidPath_NoRepositoryCallMade.
func (uc *FolderWorkspaceUseCase) GetPathStatus(ctx context.Context, devServerID, path string) (domain.PathStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.PathStatus{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if !filepath.IsAbs(path) {
		return domain.PathStatus{Status: domain.PathStatusInvalid}, nil
	}
	clean := filepath.Clean(path)

	existing, err := uc.repo.FindByPath(ctx, tenantID, devServerID, clean)
	if err != nil {
		return domain.PathStatus{}, apperrors.New(apperrors.KindInternal, "PROJECT_FOLDER_WORKSPACE_PATH_STATUS_FAILED", "failed to check folder workspace path", err)
	}
	if existing != nil {
		return domain.PathStatus{Status: domain.PathStatusAlreadyFolderWorkspace, ExistingID: existing.ID}, nil
	}

	isRepo, err := uc.repo.RepoPathExists(ctx, tenantID, devServerID, clean)
	if err != nil {
		return domain.PathStatus{}, apperrors.New(apperrors.KindInternal, "PROJECT_FOLDER_WORKSPACE_PATH_STATUS_FAILED", "failed to check repo path", err)
	}
	if isRepo {
		return domain.PathStatus{Status: domain.PathStatusAlreadyRepo}, nil
	}

	return domain.PathStatus{Status: domain.PathStatusAvailable}, nil
}

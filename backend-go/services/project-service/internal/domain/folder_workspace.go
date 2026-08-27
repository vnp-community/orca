package domain

import (
	"errors"
	"time"
)

var (
	// ErrEmptyPath is returned by NewFolderWorkspace when Path is empty.
	ErrEmptyPath = errors.New("domain: path is required")
	// ErrPathNotAbsolute is returned by usecase.FolderWorkspaceUseCase.Create
	// when Path isn't absolute — a folder workspace names a real location on
	// its bound dev server, not a shell-relative fragment.
	ErrPathNotAbsolute = errors.New("domain: path must be absolute")
	// ErrPathAlreadyRegistered is what the postgres adapter maps a
	// UNIQUE(tenant_id, dev_server_id, path) violation to — the usecase
	// layer surfaces this as apperrors.KindAlreadyExists, not a generic 500.
	ErrPathAlreadyRegistered = errors.New("domain: this path is already registered as a folder workspace")
	// ErrFolderWorkspaceNotFound is the sentinel adapter/postgres returns
	// (wrapped) when a lookup/mutation targets a folder workspace that
	// doesn't exist — usecase/ maps this to apperrors.KindNotFound.
	ErrFolderWorkspaceNotFound = errors.New("domain: folder workspace not found")
)

// PathStatus values for GetFolderWorkspacePathStatus — a DB-conflict check,
// not a live filesystem probe. See project.proto's
// GetFolderWorkspacePathStatusResponse doc comment before changing that
// assumption.
const (
	PathStatusAvailable              = "PATH_STATUS_AVAILABLE"
	PathStatusAlreadyFolderWorkspace = "PATH_STATUS_ALREADY_FOLDER_WORKSPACE"
	PathStatusAlreadyRepo            = "PATH_STATUS_ALREADY_REPO"
	PathStatusInvalid                = "PATH_STATUS_INVALID"
)

// FolderWorkspace is a standalone, non-git filesystem path added directly
// to the workspace — see project.proto's FolderWorkspace message doc
// comment for how this differs from ProjectGroup/Repo.
type FolderWorkspace struct {
	ID          string
	TenantID    string
	DevServerID string
	Path        string
	Name        string
	AddedBy     string
	CreatedAt   time.Time
}

// PathStatus is GetFolderWorkspacePathStatus's result.
type PathStatus struct {
	Status string
	// ExistingID is set when Status == PathStatusAlreadyFolderWorkspace.
	ExistingID string
}

// NewFolderWorkspace constructs a FolderWorkspace, enforcing the invariants
// a record must satisfy to be meaningful. Position/CreatedAt aren't
// constructor parameters — CreatedAt is assigned by the repository on
// insert, matching this package's other entities' convention.
func NewFolderWorkspace(id, tenantID, devServerID, path, name, addedBy string) (FolderWorkspace, error) {
	if tenantID == "" {
		return FolderWorkspace{}, ErrEmptyTenantID
	}
	if devServerID == "" {
		return FolderWorkspace{}, ErrEmptyDevServerID
	}
	if path == "" {
		return FolderWorkspace{}, ErrEmptyPath
	}
	return FolderWorkspace{
		ID:          id,
		TenantID:    tenantID,
		DevServerID: devServerID,
		Path:        path,
		Name:        name,
		AddedBy:     addedBy,
	}, nil
}

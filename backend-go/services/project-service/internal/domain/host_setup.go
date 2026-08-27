package domain

import "errors"

// HostSetupStatus is projectHostSetup's wizard-record lifecycle state.
type HostSetupStatus string

const (
	HostSetupPending   HostSetupStatus = "pending"
	HostSetupValidated HostSetupStatus = "validated"
	HostSetupCompleted HostSetupStatus = "completed"
	HostSetupFailed    HostSetupStatus = "failed"
)

// Valid reports whether s is one of the known status values.
func (s HostSetupStatus) Valid() bool {
	switch s {
	case HostSetupPending, HostSetupValidated, HostSetupCompleted, HostSetupFailed:
		return true
	default:
		return false
	}
}

var (
	// ErrHostSetupNotFound is the sentinel adapter/postgres returns
	// (wrapped) when a lookup/mutation targets a setup that doesn't exist.
	ErrHostSetupNotFound = errors.New("domain: project host setup not found")
	// ErrFolderNotFoundOnHost is returned by usecase.SetupExistingFolder
	// when the Dev Server Agent reports folder_path doesn't exist or isn't
	// a directory.
	ErrFolderNotFoundOnHost = errors.New("domain: folder not found on dev server host")
	// ErrHostSetupAlreadyCompleted guards SetupExistingFolder against
	// re-finalizing a setup that already produced a project.
	ErrHostSetupAlreadyCompleted = errors.New("domain: host setup already completed")
)

// HostSetup is the pre-project wizard record projectHostSetup.* manages.
// ProjectID is empty until SetupExistingFolder finalizes it into a real
// Project. DevServerID is a logical FK -> infra-fleet-service, ID-only,
// validated via gRPC (05-data-architecture.md), never joined in SQL.
type HostSetup struct {
	ID          string
	TenantID    string
	DevServerID string
	FolderPath  string
	DisplayName string
	Status      HostSetupStatus
	ProjectID   string
	CreatedBy   string
}

// NewHostSetup constructs a HostSetup, enforcing the invariants a wizard
// record must satisfy — always starts Pending; Status is advanced only by
// usecase.SetupExistingFolder (via repository SetStatus/Complete calls),
// never chosen at construction time.
func NewHostSetup(id, tenantID, devServerID, folderPath, displayName, createdBy string) (HostSetup, error) {
	if tenantID == "" {
		return HostSetup{}, ErrEmptyTenantID
	}
	if devServerID == "" {
		return HostSetup{}, errors.New("domain: dev_server_id is required")
	}
	if folderPath == "" {
		return HostSetup{}, errors.New("domain: folder_path is required")
	}
	if createdBy == "" {
		return HostSetup{}, errors.New("domain: created_by is required")
	}
	return HostSetup{
		ID: id, TenantID: tenantID, DevServerID: devServerID, FolderPath: folderPath,
		DisplayName: displayName, Status: HostSetupPending, CreatedBy: createdBy,
	}, nil
}

// HostSetupPatch carries UpdateHostSetup's field-mask semantics: an empty
// string means "leave unchanged" — same convention as
// domain.ProjectUpdatePatch/CompanySettingsPatch.
type HostSetupPatch struct {
	FolderPath  string
	DisplayName string
}

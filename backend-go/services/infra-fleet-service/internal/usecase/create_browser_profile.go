package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CreateBrowserProfileInput mirrors CreateBrowserProfileRequest 1:1, see
// register_dev_server.go's comment for the rationale.
type CreateBrowserProfileInput struct {
	DevServerID   string
	Name          string
	SourceBrowser string
	IsDefault     bool
}

// CreateBrowserProfile registers new browser profile metadata for a dev
// server — see SOL-006 Group C. Never stores cookie/session data itself.
type CreateBrowserProfile struct {
	repo  BrowserProfileRepository
	newID func() string
}

func NewCreateBrowserProfile(repo BrowserProfileRepository, newID func() string) *CreateBrowserProfile {
	return &CreateBrowserProfile{repo: repo, newID: newID}
}

func (uc *CreateBrowserProfile) Execute(ctx context.Context, in CreateBrowserProfileInput) (domain.BrowserProfile, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.BrowserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.DevServerID == "" || in.Name == "" {
		return domain.BrowserProfile{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_BROWSER_PROFILE_INVALID", "dev_server_id and name are required", nil)
	}
	profile := domain.BrowserProfile{
		ID: uc.newID(), TenantID: tenantID, DevServerID: in.DevServerID,
		Name: in.Name, SourceBrowser: in.SourceBrowser, IsDefault: in.IsDefault,
	}
	return uc.repo.Create(ctx, profile)
}

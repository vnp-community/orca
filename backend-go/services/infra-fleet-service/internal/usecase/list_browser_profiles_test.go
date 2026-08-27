package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeBrowserProfileRepository is an in-memory BrowserProfileRepository.
type fakeBrowserProfileRepository struct {
	byDevServer map[string][]domain.BrowserProfile
	listErr     error

	created   []domain.BrowserProfile
	createErr error

	deletedTenantID string
	deletedID       string
	deleteErr       error
}

func (f *fakeBrowserProfileRepository) List(ctx context.Context, tenantID, devServerID string) ([]domain.BrowserProfile, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byDevServer[devServerID], nil
}

func (f *fakeBrowserProfileRepository) Create(ctx context.Context, profile domain.BrowserProfile) (domain.BrowserProfile, error) {
	if f.createErr != nil {
		return domain.BrowserProfile{}, f.createErr
	}
	f.created = append(f.created, profile)
	return profile, nil
}

func (f *fakeBrowserProfileRepository) Delete(ctx context.Context, tenantID, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedTenantID = tenantID
	f.deletedID = id
	return nil
}

func TestListBrowserProfiles_RequiresTenantContext(t *testing.T) {
	uc := NewListBrowserProfiles(&fakeBrowserProfileRepository{})
	_, err := uc.Execute(context.Background(), "ds-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListBrowserProfiles_EmptyDevServerID_FailsWithoutCallingRepo(t *testing.T) {
	repo := &fakeBrowserProfileRepository{}
	uc := NewListBrowserProfiles(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected INFRA_NO_DEV_SERVER error for empty dev_server_id")
	}
}

func TestListBrowserProfiles_ReturnsRepoRowsUnmodified(t *testing.T) {
	want := []domain.BrowserProfile{
		{ID: "bp-1", TenantID: "tenant-1", DevServerID: "ds-1", Name: "Work"},
		{ID: "bp-2", TenantID: "tenant-1", DevServerID: "ds-1", Name: "Personal", SourceBrowser: "chrome"},
	}
	repo := &fakeBrowserProfileRepository{byDevServer: map[string][]domain.BrowserProfile{"ds-1": want}}
	uc := NewListBrowserProfiles(repo)

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("profile[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

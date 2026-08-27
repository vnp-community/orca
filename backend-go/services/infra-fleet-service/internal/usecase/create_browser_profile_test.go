package usecase

import (
	"context"
	"testing"
)

func TestCreateBrowserProfile_RequiresTenantContext(t *testing.T) {
	uc := NewCreateBrowserProfile(&fakeBrowserProfileRepository{}, func() string { return "bp-1" })
	_, err := uc.Execute(context.Background(), CreateBrowserProfileInput{DevServerID: "ds-1", Name: "Work"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateBrowserProfile_EmptyName_FailsValidation(t *testing.T) {
	repo := &fakeBrowserProfileRepository{}
	uc := NewCreateBrowserProfile(repo, func() string { return "bp-1" })

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateBrowserProfileInput{DevServerID: "ds-1", Name: ""})
	if err == nil {
		t.Fatal("expected an error for a missing name")
	}
	if len(repo.created) != 0 {
		t.Error("expected no creation to occur for invalid input")
	}
}

func TestCreateBrowserProfile_EmptyDevServerID_FailsValidation(t *testing.T) {
	repo := &fakeBrowserProfileRepository{}
	uc := NewCreateBrowserProfile(repo, func() string { return "bp-1" })

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, CreateBrowserProfileInput{DevServerID: "", Name: "Work"})
	if err == nil {
		t.Fatal("expected an error for a missing dev_server_id")
	}
	if len(repo.created) != 0 {
		t.Error("expected no creation to occur for invalid input")
	}
}

func TestCreateBrowserProfile_CreatesWithGeneratedIDAndFieldsVerbatim(t *testing.T) {
	repo := &fakeBrowserProfileRepository{}
	uc := NewCreateBrowserProfile(repo, func() string { return "bp-generated" })

	ctx := withTenant(context.Background(), "tenant-1")
	got, err := uc.Execute(ctx, CreateBrowserProfileInput{
		DevServerID: "ds-1", Name: "Work", SourceBrowser: "chrome", IsDefault: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "bp-generated" {
		t.Errorf("ID = %q, want bp-generated", got.ID)
	}
	if got.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want tenant-1", got.TenantID)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 created browser profile, got %d", len(repo.created))
	}
	created := repo.created[0]
	if created.ID != "bp-generated" || created.DevServerID != "ds-1" || created.Name != "Work" ||
		created.SourceBrowser != "chrome" || !created.IsDefault {
		t.Errorf("repo received unexpected profile: %+v", created)
	}
}

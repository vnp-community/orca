package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

const tenantA = "aaaaaaaa-0000-4000-8000-000000000001"

type fakeRepo struct {
	rows      map[string]domain.TenantSettings
	getErr    error
	creations int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[string]domain.TenantSettings{}} }

func (f *fakeRepo) GetOrCreateTenantSettings(_ context.Context, d domain.TenantSettings) (domain.TenantSettings, error) {
	if f.getErr != nil {
		return domain.TenantSettings{}, f.getErr
	}
	if s, ok := f.rows[d.TenantID]; ok {
		return s, nil
	}
	f.creations++
	f.rows[d.TenantID] = d
	return d, nil
}

func (f *fakeRepo) UpdateTenantSettings(_ context.Context, s domain.TenantSettings) (domain.TenantSettings, error) {
	f.rows[s.TenantID] = s
	return s, nil
}

func kind(t *testing.T, err error) apperrors.Kind {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("not an AppError: %v", err)
	}
	return ae.Kind
}

func TestGetServerInfo_NoTenantIsUnauthenticated(t *testing.T) {
	uc := NewGetServerInfo(newFakeRepo(), Defaults{TenantEnabled: true, MaxTokenDays: 90})
	_, err := uc.Execute(context.Background())
	if kind(t, err) != apperrors.KindUnauthenticated {
		t.Fatalf("got %v", err)
	}
}

func TestGetServerInfo_LazyDefaultFollowsFlagOnlyOnce(t *testing.T) {
	repo := newFakeRepo()
	ctx := tenant.WithTenantID(context.Background(), tenantA)

	info, err := NewGetServerInfo(repo, Defaults{TenantEnabled: true, MaxTokenDays: 90}).Execute(ctx)
	if err != nil || !info.Settings.Enabled || len(info.Scopes) == 0 {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	// Flag flips later: existing tenant keeps its stored value.
	info, err = NewGetServerInfo(repo, Defaults{TenantEnabled: false, MaxTokenDays: 90}).Execute(ctx)
	if err != nil || !info.Settings.Enabled || repo.creations != 1 {
		t.Fatalf("info=%+v err=%v creations=%d", info, err, repo.creations)
	}
}

func TestGetServerInfo_DefaultDisabledWhenFlagFalse(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), tenantA)
	info, err := NewGetServerInfo(newFakeRepo(), Defaults{TenantEnabled: false, MaxTokenDays: 30}).Execute(ctx)
	if err != nil || info.Settings.Enabled || info.Settings.MaxTokenDays != 30 {
		t.Fatalf("info=%+v err=%v", info, err)
	}
}

func TestGetServerInfo_RepoErrorIsInternal(t *testing.T) {
	repo := newFakeRepo()
	repo.getErr = errors.New("db down")
	ctx := tenant.WithTenantID(context.Background(), tenantA)
	_, err := NewGetServerInfo(repo, Defaults{MaxTokenDays: 90}).Execute(ctx)
	if kind(t, err) != apperrors.KindInternal {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateTenantSettings_PartialUpdateAndValidation(t *testing.T) {
	repo := newFakeRepo()
	uc := NewUpdateTenantSettings(repo, Defaults{TenantEnabled: true, MaxTokenDays: 90})
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), tenantA), "u1")

	off, days := false, 30
	got, err := uc.Execute(ctx, UpdateTenantSettingsInput{Enabled: &off, MaxTokenDays: &days})
	if err != nil || got.Enabled || got.MaxTokenDays != 30 || got.ApprovalTTLSeconds != domain.DefaultApprovalTTL || got.UpdatedBy != "u1" {
		t.Fatalf("got %+v err=%v", got, err)
	}

	bad := 91
	_, err = uc.Execute(ctx, UpdateTenantSettingsInput{MaxTokenDays: &bad})
	if kind(t, err) != apperrors.KindInvalidArgument {
		t.Fatalf("got %v", err)
	}
	if repo.rows[tenantA].MaxTokenDays != 30 {
		t.Fatal("invalid update must not be persisted")
	}

	_, err = uc.Execute(context.Background(), UpdateTenantSettingsInput{})
	if kind(t, err) != apperrors.KindUnauthenticated {
		t.Fatalf("got %v", err)
	}
}

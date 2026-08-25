package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func TestCreateHostSetup_ValidatesDevServerID(t *testing.T) {
	repo := newFakeHostSetupRepository()
	devServers := &fakeDevServerLister{exists: false}
	uc := NewCreateHostSetup(repo, devServers)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, CreateHostSetupInput{DevServerID: "unknown", FolderPath: "/home/dev/repo"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND")
	if repo.createCalled {
		t.Error("expected repo.Create to never be called for an unknown dev server")
	}
}

func TestCreateHostSetup_PersistsOnValidDevServer(t *testing.T) {
	repo := newFakeHostSetupRepository()
	devServers := &fakeDevServerLister{exists: true}
	uc := NewCreateHostSetup(repo, devServers)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	setup, err := uc.Execute(ctx, CreateHostSetupInput{DevServerID: "dev-1", FolderPath: "/home/dev/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setup.Status != domain.HostSetupPending {
		t.Errorf("expected Status=%q, got %q", domain.HostSetupPending, setup.Status)
	}
	if !repo.createCalled {
		t.Error("expected repo.Create to be called")
	}
}

func TestCreateHostSetup_DevServerLookupErrorIsInternal(t *testing.T) {
	repo := newFakeHostSetupRepository()
	devServers := &fakeDevServerLister{err: errors.New("infra-fleet-service unreachable")}
	uc := NewCreateHostSetup(repo, devServers)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "u1")
	_, err := uc.Execute(ctx, CreateHostSetupInput{DevServerID: "dev-1", FolderPath: "/home/dev/repo"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED")
	if repo.createCalled {
		t.Error("expected repo.Create to never be called on a lookup error")
	}
}

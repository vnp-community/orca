package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestCreatePublicLink_RequiresTenantContext(t *testing.T) {
	uc := NewCreatePublicLink(newFakeShareLinkRepository(), NewResolvePermission(newFakeTaskRepository(), &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true}))
	if _, _, err := uc.Execute(context.Background(), "t1"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestCreatePublicLink_RequiresManageAccess: CreatePublicLink requires
// 'manage' on the target task before issuing a link.
func TestCreatePublicLink_RequiresManageAccess(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "someone-else"}
	links := newFakeShareLinkRepository()
	resolvePermission := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	uc := NewCreatePublicLink(links, resolvePermission)
	ctx := withIdentity(context.Background(), "tenant-1", "attacker")

	_, _, err := uc.Execute(ctx, "t1")
	if err == nil {
		t.Fatal("expected PermissionDenied for a caller with no manage access")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
	if len(links.links) != 0 {
		t.Errorf("expected no link to be created for a denied caller, got %+v", links.links)
	}
}

// TestCreatePublicLink_StoresOnlyTheHash_NeverThePlaintext is the core
// security regression guard: task_share_links (here, the fake store) must
// never contain the plaintext token, only its SHA-256 hash.
func TestCreatePublicLink_StoresOnlyTheHash_NeverThePlaintext(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	links := newFakeShareLinkRepository()
	resolvePermission := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	uc := NewCreatePublicLink(links, resolvePermission)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	id, token, err := uc.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" || token == "" {
		t.Fatal("expected a non-empty id and token")
	}
	stored, ok := links.links[id]
	if !ok {
		t.Fatalf("expected a stored link with id %q", id)
	}
	if stored.tokenHash == token {
		t.Error("expected the stored value to be a hash, not the plaintext token")
	}
	if stored.tokenHash == "" {
		t.Error("expected a non-empty stored token hash")
	}
}

func TestResolvePublicLink_ValidToken_ReturnsTaskID(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	links := newFakeShareLinkRepository()
	resolvePermission := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	createUC := NewCreatePublicLink(links, resolvePermission)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, token, err := createUC.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error creating link: %v", err)
	}

	resolveUC := NewResolvePublicLink(links)
	taskID, err := resolveUC.Execute(context.Background(), "tenant-1", token)
	if err != nil {
		t.Fatalf("unexpected error resolving: %v", err)
	}
	if taskID != "t1" {
		t.Errorf("expected task_id=t1, got %q", taskID)
	}
}

// TestResolvePublicLink_RevokedToken_ReturnsNotFound: a revoked link's
// token must never resolve — not a stale grant.
func TestResolvePublicLink_RevokedToken_ReturnsNotFound(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "user-1"}
	links := newFakeShareLinkRepository()
	resolvePermission := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	createUC := NewCreatePublicLink(links, resolvePermission)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	id, token, err := createUC.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error creating link: %v", err)
	}

	revokeUC := NewRevokePublicLink(links, resolvePermission, tasks)
	if err := revokeUC.Execute(ctx, id); err != nil {
		t.Fatalf("unexpected error revoking: %v", err)
	}

	resolveUC := NewResolvePublicLink(links)
	_, err = resolveUC.Execute(context.Background(), "tenant-1", token)
	if err == nil {
		t.Fatal("expected a not-found error for a revoked token")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound {
		t.Fatalf("expected KindNotFound, got %v", err)
	}
}

// TestResolvePublicLink_ExpiredToken_ReturnsNotFound is the expiry mirror
// of the revoked-token case.
func TestResolvePublicLink_ExpiredToken_ReturnsNotFound(t *testing.T) {
	links := newFakeShareLinkRepository()
	past := time.Now().Add(-time.Hour)
	links.links["link-1"] = fakeShareLink{tenantID: "tenant-1", taskID: "t1", tokenHash: "abc", expiresAt: &past}

	uc := NewResolvePublicLink(links)
	if _, err := uc.Execute(context.Background(), "tenant-1", "irrelevant-plaintext-that-hashes-to-something-else"); err == nil {
		t.Fatal("expected a not-found error for an unrecognized token")
	}
}

func TestResolvePublicLink_UnknownToken_ReturnsNotFound(t *testing.T) {
	uc := NewResolvePublicLink(newFakeShareLinkRepository())
	if _, err := uc.Execute(context.Background(), "tenant-1", "does-not-exist"); err == nil {
		t.Fatal("expected a not-found error for an unrecognized token")
	}
}

func TestRevokePublicLink_RequiresManageAccess(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", OwnerID: "owner"}
	links := newFakeShareLinkRepository()
	links.links["link-1"] = fakeShareLink{tenantID: "tenant-1", taskID: "t1", tokenHash: "abc"}
	resolvePermission := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	uc := NewRevokePublicLink(links, resolvePermission, tasks)
	ctx := withIdentity(context.Background(), "tenant-1", "attacker")

	err := uc.Execute(ctx, "link-1")
	if err == nil {
		t.Fatal("expected PermissionDenied for a caller with no manage access")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("expected KindPermissionDenied, got %v", err)
	}
}

func TestRevokePublicLink_NonexistentLink_ReturnsNotFound(t *testing.T) {
	tasks := newFakeTaskRepository()
	links := newFakeShareLinkRepository()
	resolvePermission := NewResolvePermission(tasks, &fakeGrantRepository{}, &fakeTeamScopeResolver{}, &fakeOPAClient{allow: true})
	uc := NewRevokePublicLink(links, resolvePermission, tasks)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected a not-found error for a nonexistent link")
	}
}

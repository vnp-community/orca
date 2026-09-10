package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func newTestGenerateShareLink(tasks *fakeTaskRepository, grants *fakeGrantRepository, opa *fakeOPAClient) *GenerateShareLink {
	resolvePermission := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, opa, nil)
	return NewGenerateShareLink(tasks, resolvePermission)
}

func TestGenerateShareLink_RequiresTenantContext(t *testing.T) {
	uc := newTestGenerateShareLink(newFakeTaskRepository(), &fakeGrantRepository{}, &fakeOPAClient{allow: true})
	if _, err := uc.Execute(context.Background(), GenerateShareLinkInput{TaskID: "t1", UserID: "u1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestGenerateShareLink_RequiresAdminPermission is the task's own named
// test: a caller with only user-level access is denied.
func TestGenerateShareLink_RequiresAdminPermission(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "t1")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelUser},
	}}
	// OPA's real level_actions table denies "admin" for user-level grants
	// (task_grant.rego:25-31) — this fake mirrors just that distinction.
	opa := &fakeOPAClient{allow: false}
	uc := newTestGenerateShareLink(tasks, grants, opa)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateShareLinkInput{TaskID: "t1", UserID: "user-1"}); err == nil {
		t.Fatal("expected a sub-admin caller to be denied")
	}
	if tasks.tasks["t1"].ShareToken != "" {
		t.Error("expected no share token to be minted for a denied caller")
	}
}

func TestGenerateShareLink_AdminCaller_MintsAndPersistsToken(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "t1")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelAdmin},
	}}
	uc := newTestGenerateShareLink(tasks, grants, &fakeOPAClient{allow: true})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	token, err := uc.Execute(ctx, GenerateShareLinkInput{TaskID: "t1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty token")
	}
	if tasks.tasks["t1"].ShareToken != token {
		t.Errorf("expected the returned token to be persisted on the task, got %+v", tasks.tasks["t1"].ShareToken)
	}
}

// TestGenerateShareLink_TokenUniqueness_AcrossCalls is the task's own
// named statistical/format assertion: two GenerateShareLink calls on
// different tasks never collide.
func TestGenerateShareLink_TokenUniqueness_AcrossCalls(t *testing.T) {
	tasks := newFakeTaskRepository()
	setupChain(t, tasks, "tenant-1", "t1")
	setupChain(t, tasks, "tenant-1", "t2")
	grants := &fakeGrantRepository{grants: []domain.Grant{
		{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelAdmin},
		{TaskID: "t2", SubjectID: "user-1", Level: domain.GrantLevelAdmin},
	}}
	uc := newTestGenerateShareLink(tasks, grants, &fakeOPAClient{allow: true})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	token1, err := uc.Execute(ctx, GenerateShareLinkInput{TaskID: "t1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	token2, err := uc.Execute(ctx, GenerateShareLinkInput{TaskID: "t2", UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token1 == token2 {
		t.Fatalf("expected distinct tokens, got the same value twice: %q", token1)
	}
	if len(token1) != 64 || len(token2) != 64 { // 32 random bytes, hex-encoded
		t.Errorf("expected 64-char hex tokens, got lengths %d and %d", len(token1), len(token2))
	}
}

package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestUpdateAccessPolicy_CalledTwice_PersistsTwoVersions(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	policies := newFakeAccessPolicyRepository()
	publisher := &fakePolicyPublisher{}
	clock := &fakeClock{now: time.Now()}
	opa := &fakeOPAClient{allow: true}
	ctx := withActor(context.Background(), "t1", "admin1")

	create := NewCreateAccessPolicy(users, policies, clock, opa)
	created, err := create.Execute(ctx, CreateAccessPolicyInput{Name: "rate-tier-default", Kind: "rate-tier", DocumentJSON: `{"rps":10}`})
	if err != nil {
		t.Fatalf("CreateAccessPolicy: unexpected error: %v", err)
	}
	if created.Version != 1 {
		t.Fatalf("created.Version = %d, want 1", created.Version)
	}

	update := NewUpdateAccessPolicy(users, policies, publisher, clock, opa)
	updated, err := update.Execute(ctx, created.ID, `{"rps":20}`, 1)
	if err != nil {
		t.Fatalf("UpdateAccessPolicy (1st): unexpected error: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("updated.Version = %d, want 2", updated.Version)
	}

	updated2, err := update.Execute(ctx, created.ID, `{"rps":30}`, 2)
	if err != nil {
		t.Fatalf("UpdateAccessPolicy (2nd): unexpected error: %v", err)
	}
	if updated2.Version != 3 {
		t.Fatalf("updated2.Version = %d, want 3", updated2.Version)
	}

	if len(policies.versions[created.ID]) != 3 {
		t.Fatalf("expected 3 persisted version rows, got %d", len(policies.versions[created.ID]))
	}
	if len(publisher.published) != 2 {
		t.Fatalf("expected PublishPolicyChange to be called twice, got %d", len(publisher.published))
	}

	get := NewGetAccessPolicy(users, policies, opa)
	latest, err := get.Execute(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetAccessPolicy: unexpected error: %v", err)
	}
	if latest.Version != 3 || latest.DocumentJSON != `{"rps":30}` {
		t.Fatalf("GetAccessPolicy returned stale/wrong version: %+v", latest)
	}

	list := NewListAccessPolicies(users, policies, opa)
	out, err := list.Execute(ctx, ListAccessPoliciesInput{})
	if err != nil {
		t.Fatalf("ListAccessPolicies: unexpected error: %v", err)
	}
	if len(out.Policies) != 1 || out.Policies[0].Version != 3 {
		t.Fatalf("ListAccessPolicies should return exactly one row (latest version) per policy id, got %+v", out.Policies)
	}
}

func TestUpdateAccessPolicy_StaleExpectedVersion_FailsPrecondition(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	policies := newFakeAccessPolicyRepository()
	clock := &fakeClock{now: time.Now()}
	opa := &fakeOPAClient{allow: true}
	ctx := withActor(context.Background(), "t1", "admin1")

	create := NewCreateAccessPolicy(users, policies, clock, opa)
	created, err := create.Execute(ctx, CreateAccessPolicyInput{Name: "role-def", Kind: "role-definition", DocumentJSON: `{}`})
	if err != nil {
		t.Fatalf("CreateAccessPolicy: unexpected error: %v", err)
	}

	update := NewUpdateAccessPolicy(users, policies, &fakePolicyPublisher{}, clock, opa)
	if _, err := update.Execute(ctx, created.ID, `{"changed":true}`, 99); err == nil {
		t.Fatal("expected FailedPrecondition error for a stale expected_version, got nil")
	}

	// The stale attempt must not have silently overwritten anything.
	get := NewGetAccessPolicy(users, policies, opa)
	latest, err := get.Execute(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetAccessPolicy: unexpected error: %v", err)
	}
	if latest.Version != 1 {
		t.Fatalf("expected version to remain 1 after a rejected stale update, got %d", latest.Version)
	}
}

func TestDeleteAccessPolicy_RemovesAllVersions(t *testing.T) {
	users := newFakeUserRepository()
	seedActiveUser(t, users, fakeHasher{}, "admin1", "t1", "admin@example.com", "pw", domain.RoleAdmin)
	policies := newFakeAccessPolicyRepository()
	clock := &fakeClock{now: time.Now()}
	opa := &fakeOPAClient{allow: true}
	ctx := withActor(context.Background(), "t1", "admin1")

	create := NewCreateAccessPolicy(users, policies, clock, opa)
	created, err := create.Execute(ctx, CreateAccessPolicyInput{Name: "to-delete", Kind: "rate-tier", DocumentJSON: `{}`})
	if err != nil {
		t.Fatalf("CreateAccessPolicy: unexpected error: %v", err)
	}

	del := NewDeleteAccessPolicy(users, policies, opa)
	if err := del.Execute(ctx, created.ID); err != nil {
		t.Fatalf("DeleteAccessPolicy: unexpected error: %v", err)
	}

	get := NewGetAccessPolicy(users, policies, opa)
	if _, err := get.Execute(ctx, created.ID); err == nil {
		t.Fatal("expected an error getting a deleted policy")
	}
}

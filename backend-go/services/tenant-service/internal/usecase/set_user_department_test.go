package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestSetUserDepartment_RequiresTenantContext(t *testing.T) {
	uc := NewSetUserDepartment(newFakeDepartmentRepository(), newFakeUserProfileRepository(), newFakeProfileCache(), nil)
	err := uc.Execute(context.Background(), SetUserDepartmentInput{UserID: "user-1", DepartmentID: "dept-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestSetUserDepartment_DepartmentFromAnotherCompanyIsNotFound(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "other-company", "Engineering", nil))

	uc := NewSetUserDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), nil)
	ctx := withTenant(context.Background(), "company-1")

	err := uc.Execute(ctx, SetUserDepartmentInput{UserID: "user-1", DepartmentID: "dept-1"})
	if err == nil {
		t.Fatal("expected a cross-tenant department_id to resolve as not-found, per tenant-service.md §9")
	}
}

func TestSetUserDepartment_UpsertsProfileAndInvalidatesCache(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	profiles := newFakeUserProfileRepository()
	cache := newFakeProfileCache()
	cache.byUserID["user-1"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}

	uc := NewSetUserDepartment(departments, profiles, cache, nil)
	ctx := withTenant(context.Background(), "company-1")

	if err := uc.Execute(ctx, SetUserDepartmentInput{UserID: "user-1", DepartmentID: "dept-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, ok := profiles.byUserID["user-1"]
	if !ok {
		t.Fatal("expected a user_profiles row to be upserted")
	}
	if stored.DepartmentID != "dept-1" || stored.CompanyID != "company-1" {
		t.Errorf("unexpected stored profile: %+v", stored)
	}

	if _, cached := cache.byUserID["user-1"]; cached {
		t.Error("expected SetUserDepartment to invalidate the affected user's cached profile")
	}
}

func TestSetUserDepartment_BroadcastsInvalidationToOtherReplicas(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	invalidation := newFakeCacheInvalidationPublisher()

	uc := NewSetUserDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), invalidation)
	ctx := withTenant(context.Background(), "company-1")

	if err := uc.Execute(ctx, SetUserDepartmentInput{UserID: "user-1", DepartmentID: "dept-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(invalidation.calls) != 1 || invalidation.calls[0] != "user-1" {
		t.Errorf("expected exactly one PublishProfileInvalidated call for user-1, got %+v", invalidation.calls)
	}
}

func TestSetUserDepartment_PublishErrorDoesNotFailTheWrite(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	invalidation := newFakeCacheInvalidationPublisher()
	invalidation.err = errFakeRepository

	uc := NewSetUserDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), invalidation)
	ctx := withTenant(context.Background(), "company-1")

	if err := uc.Execute(ctx, SetUserDepartmentInput{UserID: "user-1", DepartmentID: "dept-1"}); err != nil {
		t.Fatalf("expected a failed best-effort broadcast not to fail the write, got: %v", err)
	}
}

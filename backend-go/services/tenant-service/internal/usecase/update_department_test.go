package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// deptAdminCtx builds a request context for an admin actor with no
// department of their own — admin passes requireDepartmentAccess
// regardless of sameDepartment, so the actor's own DepartmentID is
// irrelevant for these tests unless stated otherwise.
func deptAdminCtx(companyID, actorID string) context.Context {
	return withRole(withActor(withTenant(context.Background(), companyID), actorID), "admin")
}

func TestUpdateDepartment_AppliesPatch(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Old Name", nil))

	uc := NewUpdateDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), nil, newFakeOPAClient(true), nil)
	ctx := deptAdminCtx("company-1", "admin-1")

	got, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "dept-1",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("expected patched name %q, got %q", "New Name", got.Name)
	}
}

func TestUpdateDepartment_InvalidatesEveryDepartmentUser(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["user-1"] = mustUserProfile(t, "user-1", "company-1", "dept-1", nil)
	profiles.byUserID["user-2"] = mustUserProfile(t, "user-2", "company-1", "dept-1", nil)
	profiles.byUserID["user-3"] = mustUserProfile(t, "user-3", "company-1", "dept-1", nil)
	cache := newFakeProfileCache()
	cache.byUserID["user-1"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}
	cache.byUserID["user-2"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}
	cache.byUserID["user-3"] = domain.ResolvedProfile{Settings: domain.Settings{"stale": true}}

	uc := NewUpdateDepartment(departments, profiles, cache, nil, newFakeOPAClient(true), nil)
	ctx := deptAdminCtx("company-1", "admin-1")

	if _, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "dept-1",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, uid := range []string{"user-1", "user-2", "user-3"} {
		if _, cached := cache.byUserID[uid]; cached {
			t.Errorf("expected %s's cached profile to be invalidated", uid)
		}
	}
}

func TestUpdateDepartment_NotFound(t *testing.T) {
	departments := newFakeDepartmentRepository()
	profiles := newFakeUserProfileRepository()
	cache := newFakeProfileCache()

	uc := NewUpdateDepartment(departments, profiles, cache, nil, newFakeOPAClient(true), nil)
	ctx := deptAdminCtx("company-1", "admin-1")

	_, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "missing-dept",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	})
	assertAppError(t, err, apperrors.KindNotFound)

	if profiles.listByDeptCalls != 0 {
		t.Errorf("expected ListUserIDsByDepartment not to be called on a failed write, got %d calls", profiles.listByDeptCalls)
	}
	if len(cache.invalidateCalls) != 0 {
		t.Errorf("expected no cache invalidation on a failed write, got %+v", cache.invalidateCalls)
	}
}

func TestUpdateDepartment_CrossCompanyIsNotFound(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "other-company", "Engineering", nil))

	uc := NewUpdateDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), nil, newFakeOPAClient(true), nil)
	ctx := deptAdminCtx("company-1", "admin-1")

	_, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "dept-1",
		Patch: domain.DepartmentSettingsPatch{Name: "New Name"},
	})
	assertAppError(t, err, apperrors.KindNotFound)
}

func TestUpdateDepartment_LeadEditingOwnDepartment_Allowed(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["lead-1"] = mustUserProfile(t, "lead-1", "company-1", "dept-1", nil)
	opa := newFakeOPAClient(true)

	uc := NewUpdateDepartment(departments, profiles, newFakeProfileCache(), nil, opa, nil)
	ctx := withRole(withActor(withTenant(context.Background(), "company-1"), "lead-1"), "lead")

	if _, err := uc.Execute(ctx, UpdateDepartmentInput{ID: "dept-1", Patch: domain.DepartmentSettingsPatch{Name: "New Name"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opa.calls) != 1 || !opa.calls[0].sameDepartment {
		t.Errorf("expected OPA to be asked with sameDepartment=true, got %+v", opa.calls)
	}
}

func TestUpdateDepartment_LeadEditingDifferentDepartment_Denied(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-2", "company-1", "Sales", nil))
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["lead-1"] = mustUserProfile(t, "lead-1", "company-1", "dept-1", nil)
	// Fake OPA mimics the real policy: lead only allowed when sameDepartment.
	opa := &fakeOPAClient{}
	opa.allow = false

	uc := NewUpdateDepartment(departments, profiles, newFakeProfileCache(), nil, opa, nil)
	ctx := withRole(withActor(withTenant(context.Background(), "company-1"), "lead-1"), "lead")

	_, err := uc.Execute(ctx, UpdateDepartmentInput{ID: "dept-2", Patch: domain.DepartmentSettingsPatch{Name: "New Name"}})
	assertAppError(t, err, apperrors.KindPermissionDenied)
	if len(opa.calls) != 1 || opa.calls[0].sameDepartment {
		t.Errorf("expected OPA to be asked with sameDepartment=false, got %+v", opa.calls)
	}
}

func TestUpdateDepartment_AdminPassesRegardlessOfOwnDepartment(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-2", "company-1", "Sales", nil))
	profiles := newFakeUserProfileRepository()
	profiles.byUserID["admin-1"] = mustUserProfile(t, "admin-1", "company-1", "dept-1", nil)

	uc := NewUpdateDepartment(departments, profiles, newFakeProfileCache(), nil, newFakeOPAClient(true), nil)
	ctx := deptAdminCtx("company-1", "admin-1")

	if _, err := uc.Execute(ctx, UpdateDepartmentInput{ID: "dept-2", Patch: domain.DepartmentSettingsPatch{Name: "New Name"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDepartment_SettingSecurityKey_Rejected(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))

	uc := NewUpdateDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), nil, newFakeOPAClient(true), nil)
	ctx := deptAdminCtx("company-1", "admin-1")

	_, err := uc.Execute(ctx, UpdateDepartmentInput{
		ID:    "dept-1",
		Patch: domain.DepartmentSettingsPatch{SettingsJSON: `{"security":{"sessionTimeoutHours":24}}`},
	})
	assertAppError(t, err, apperrors.KindInvalidArgument)
}

func TestUpdateDepartment_PublishesAuditEventOnSuccess(t *testing.T) {
	departments := newFakeDepartmentRepository()
	_, _ = departments.Create(context.Background(), mustDepartment(t, "dept-1", "company-1", "Engineering", nil))
	audit := newFakeAuditPublisher()

	uc := NewUpdateDepartment(departments, newFakeUserProfileRepository(), newFakeProfileCache(), nil, newFakeOPAClient(true), audit)
	ctx := deptAdminCtx("company-1", "admin-1")

	if _, err := uc.Execute(ctx, UpdateDepartmentInput{ID: "dept-1", Patch: domain.DepartmentSettingsPatch{Name: "New Name"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "department.profile.updated" {
		t.Errorf("expected 1 audit event with action department.profile.updated, got %+v", audit.calls)
	}
}

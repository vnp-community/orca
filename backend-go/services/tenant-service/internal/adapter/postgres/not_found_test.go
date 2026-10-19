//go:build integration

// TASK-E2E-017: unknown or malformed ids answer found=false / empty (NotFound upstream), never a raw
// Postgres error that the usecase would turn into Internal.
package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestRepositories_UnknownOrMalformedID_AreNotFound(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	companies := NewCompanyRepository(pool)
	departments := NewDepartmentRepository(pool)
	teams := NewTeamRepository(pool)
	profiles := NewUserProfileRepository(pool)

	for _, id := range []string{uuid.NewString(), "abc"} {
		if _, found, err := companies.Get(ctx, id); err != nil || found {
			t.Errorf("companies.Get(%q) = found %v, err %v; want false, nil", id, found, err)
		}
		if ok, err := companies.Exists(ctx, id); err != nil || ok {
			t.Errorf("companies.Exists(%q) = %v, %v; want false, nil", id, ok, err)
		}
		if _, found, err := companies.Update(ctx, id, domain.CompanySettingsPatch{Name: "x"}); err != nil || found {
			t.Errorf("companies.Update(%q) = found %v, err %v; want false, nil", id, found, err)
		}
		if _, found, err := departments.Get(ctx, uuid.NewString(), id); err != nil || found {
			t.Errorf("departments.Get(%q) = found %v, err %v; want false, nil", id, found, err)
		}
		if _, found, err := departments.Update(ctx, uuid.NewString(), id, domain.DepartmentSettingsPatch{}); err != nil || found {
			t.Errorf("departments.Update(%q) = found %v, err %v; want false, nil", id, found, err)
		}
		if list, err := departments.List(ctx, id); err != nil || len(list) != 0 {
			t.Errorf("departments.List(%q) = %v, %v; want empty, nil", id, list, err)
		}
		if _, found, err := teams.Get(ctx, uuid.NewString(), id); err != nil || found {
			t.Errorf("teams.Get(%q) = found %v, err %v; want false, nil", id, found, err)
		}
		if list, err := teams.ListByCompany(ctx, id); err != nil || len(list) != 0 {
			t.Errorf("teams.ListByCompany(%q) = %v, %v; want empty, nil", id, list, err)
		}
		if list, err := teams.ListMembers(ctx, id); err != nil || len(list) != 0 {
			t.Errorf("teams.ListMembers(%q) = %v, %v; want empty, nil", id, list, err)
		}
		if removed, err := teams.RemoveMember(ctx, id, uuid.NewString()); err != nil || removed {
			t.Errorf("teams.RemoveMember(team %q) = %v, %v; want false, nil", id, removed, err)
		}
		if removed, err := teams.RemoveMember(ctx, uuid.NewString(), id); err != nil || removed {
			t.Errorf("teams.RemoveMember(user %q) = %v, %v; want false, nil", id, removed, err)
		}
		if layers, err := teams.ListUserTeamLayers(ctx, uuid.NewString(), id); err != nil || len(layers) != 0 {
			t.Errorf("teams.ListUserTeamLayers(user %q) = %v, %v; want empty, nil", id, layers, err)
		}
		if _, found, err := profiles.Get(ctx, uuid.NewString(), id); err != nil || found {
			t.Errorf("profiles.Get(%q) = found %v, err %v; want false, nil", id, found, err)
		}
		if ids, err := profiles.ListUserIDsByDepartment(ctx, uuid.NewString(), id); err != nil || len(ids) != 0 {
			t.Errorf("profiles.ListUserIDsByDepartment(%q) = %v, %v; want empty, nil", id, ids, err)
		}
		if ids, err := profiles.ListUserIDsByCompany(ctx, id); err != nil || len(ids) != 0 {
			t.Errorf("profiles.ListUserIDsByCompany(%q) = %v, %v; want empty, nil", id, ids, err)
		}
	}

	// A user id the store cannot represent is the caller's mistake, reported as a typed error.
	company, err := domain.NewCompany(uuid.NewString(), "Acme", nil)
	if err != nil {
		t.Fatalf("building company: %v", err)
	}
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	team, err := domain.NewTeam(uuid.NewString(), company.ID, "core", nil)
	if err != nil {
		t.Fatalf("building team: %v", err)
	}
	if _, err := teams.Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	member, err := domain.NewTeamMember(team.ID, "not-a-uuid", 0)
	if err != nil {
		t.Fatalf("building member: %v", err)
	}
	if err := teams.AddMember(ctx, member); !errors.Is(err, domain.ErrMalformedUserID) {
		t.Errorf("AddMember(malformed user) = %v; want ErrMalformedUserID", err)
	}
	profile, err := domain.NewUserProfile("not-a-uuid", company.ID, "", nil)
	if err != nil {
		t.Fatalf("building profile: %v", err)
	}
	if err := profiles.Upsert(ctx, profile); !errors.Is(err, domain.ErrMalformedUserID) {
		t.Errorf("Upsert(malformed user) = %v; want ErrMalformedUserID", err)
	}
}

//go:build integration

// TASK-E2E-017: unknown ids answer found=false / empty on every dialect, same as Postgres.
package mysql

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestRepositories_UnknownID_AreNotFound(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	companies := NewCompanyRepository(db)
	departments := NewDepartmentRepository(db)
	teams := NewTeamRepository(db)
	profiles := NewUserProfileRepository(db)

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
		if list, err := teams.ListMembers(ctx, id); err != nil || len(list) != 0 {
			t.Errorf("teams.ListMembers(%q) = %v, %v; want empty, nil", id, list, err)
		}
		if removed, err := teams.RemoveMember(ctx, id, uuid.NewString()); err != nil || removed {
			t.Errorf("teams.RemoveMember(team %q) = %v, %v; want false, nil", id, removed, err)
		}
		if _, found, err := profiles.Get(ctx, uuid.NewString(), id); err != nil || found {
			t.Errorf("profiles.Get(%q) = found %v, err %v; want false, nil", id, found, err)
		}
	}
}

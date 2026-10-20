//go:build integration

package postgres

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestPostgres_RequestRepositoryContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunRequestRepositoryContract(t, func(*testing.T) contracttest.Env { return f.contractEnv() })
}

func TestPostgres_EngineSettingsContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunEngineSettingsContract(t, NewProjectEngineSettingsRepository(New(f.app)))
}

func TestPostgres_OpenSpecChangeContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunOpenSpecChangeContract(t, f.contractEnv(), NewOpenSpecChangeRepository(New(f.app)))
}

func TestPostgres_PlanAutoJobContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunPlanAutoJobContract(t, f.contractEnv(), NewPlanAutoJobRepository(New(f.app)))
}

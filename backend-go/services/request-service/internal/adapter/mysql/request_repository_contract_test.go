//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_RequestRepositoryContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunRequestRepositoryContract(t, func(*testing.T) contracttest.Env { return f.contractEnv() })
}

func TestMySQL_EngineSettingsContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunEngineSettingsContract(t, NewProjectEngineSettingsRepository(New(f.db)))
}

func TestMySQL_OpenSpecChangeContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunOpenSpecChangeContract(t, f.contractEnv(), NewOpenSpecChangeRepository(New(f.db)))
}

func TestMySQL_PlanAutoJobContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunPlanAutoJobContract(t, f.contractEnv(), NewPlanAutoJobRepository(New(f.db)))
}

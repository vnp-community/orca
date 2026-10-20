//go:build integration

package postgres

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestPostgres_PlanAutoFlowContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunPlanAutoFlowContract(t, contracttest.PlanAutoFlowEnv{SolutionEnv: f.solutionEnv(), Jobs: NewPlanAutoJobRepository(New(f.app))})
}

func TestPostgres_PlanRevisionFlowContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunPlanRevisionFlowContract(t, contracttest.PlanAutoFlowEnv{SolutionEnv: f.solutionEnv(), Jobs: NewPlanAutoJobRepository(New(f.app))})
}

//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_PlanAutoFlowContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunPlanAutoFlowContract(t, contracttest.PlanAutoFlowEnv{SolutionEnv: f.solutionEnv(), Jobs: NewPlanAutoJobRepository(New(f.db))})
}

func TestMySQL_PlanRevisionFlowContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunPlanRevisionFlowContract(t, contracttest.PlanAutoFlowEnv{SolutionEnv: f.solutionEnv(), Jobs: NewPlanAutoJobRepository(New(f.db))})
}

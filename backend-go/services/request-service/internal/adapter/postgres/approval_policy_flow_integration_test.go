//go:build integration

package postgres

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestPostgres_ApprovalPolicyContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunApprovalPolicyContract(t, func(*testing.T) contracttest.ApprovalEnv { return f.approvalEnv() })
}

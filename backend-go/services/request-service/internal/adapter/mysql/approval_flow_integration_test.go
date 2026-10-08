//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_ApprovalFlowContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunApprovalFlowContract(t, func(*testing.T) contracttest.ApprovalEnv { return f.approvalEnv() })
}

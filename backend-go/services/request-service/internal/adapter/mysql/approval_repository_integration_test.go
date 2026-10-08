//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func (f *myFixture) approvalEnv() contracttest.ApprovalEnv {
	base := New(f.db)
	return contracttest.ApprovalEnv{
		LifecycleEnv: f.lifecycleEnv(),
		Approvals:    NewApprovalRepository(base),
		Approvers:    NewApproverRepository(base),
		Policies:     NewPolicyRepository(base),
		Locker:       NewRequestLocker(base),
	}
}

func TestMySQL_ApprovalRepositoryContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunApprovalRepositoryContract(t, func(*testing.T) contracttest.ApprovalEnv { return f.approvalEnv() })
}

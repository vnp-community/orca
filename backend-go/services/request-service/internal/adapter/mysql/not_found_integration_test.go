//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_ArtifactNotFoundContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunArtifactNotFoundContract(t, func(*testing.T) contracttest.ArtifactEnv { return f.artifactEnv() })
}

func TestMySQL_ApprovalNotFoundContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunApprovalNotFoundContract(t, func(*testing.T) contracttest.ApprovalEnv { return f.approvalEnv() })
}

func TestMySQL_RetentionNotFoundContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunRetentionNotFoundContract(t, func(*testing.T) contracttest.SecurityEnv { return f.securityEnv() })
}

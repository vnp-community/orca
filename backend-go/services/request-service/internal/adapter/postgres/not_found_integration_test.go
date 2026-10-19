//go:build integration

package postgres

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestPostgres_ArtifactNotFoundContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunArtifactNotFoundContract(t, func(*testing.T) contracttest.ArtifactEnv { return f.artifactEnv() })
}

func TestPostgres_ApprovalNotFoundContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunApprovalNotFoundContract(t, func(*testing.T) contracttest.ApprovalEnv { return f.approvalEnv() })
}

func TestPostgres_RetentionNotFoundContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunRetentionNotFoundContract(t, func(*testing.T) contracttest.SecurityEnv { return f.securityEnv() })
}

//go:build integration

package postgres

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestPostgres_RequestServiceRPCs(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunRequestServiceContract(t, func(*testing.T) contracttest.Env { return f.contractEnv() })
}

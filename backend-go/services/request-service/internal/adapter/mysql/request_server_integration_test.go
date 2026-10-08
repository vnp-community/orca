//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_RequestServiceRPCs(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunRequestServiceContract(t, func(*testing.T) contracttest.Env { return f.contractEnv() })
}

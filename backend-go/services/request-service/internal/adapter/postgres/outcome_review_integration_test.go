//go:build integration

package postgres

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestPostgres_OutcomeReviewContract(t *testing.T) {
	f := newMigratedPostgres(t)
	base := New(f.app)
	contracttest.RunOutcomeReviewContract(t, NewOutcomeReviewRepository(base), NewOutcomeReviewRunRepository(base))
}

func TestPostgres_ReviewContextStoreContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunReviewContextStoreContract(t, NewOutcomeReviewContextRepository(New(f.app)))
}

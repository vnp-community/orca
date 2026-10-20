//go:build integration

package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

func TestMySQL_OutcomeReviewContract(t *testing.T) {
	f := newMigratedMySQL(t)
	base := New(f.db)
	contracttest.RunOutcomeReviewContract(t, NewOutcomeReviewRepository(base), NewOutcomeReviewRunRepository(base))
}

func TestMySQL_ReviewContextStoreContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunReviewContextStoreContract(t, NewOutcomeReviewContextRepository(New(f.db)))
}

func TestMySQL_ReviewArtifactContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunReviewArtifactContract(t, NewReviewArtifactRepository(New(f.db)))
}

func TestMySQL_ReviewDecisionContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunReviewDecisionContract(t, NewOutcomeReviewDecisionRepository(New(f.db)))
}

func TestMySQL_ReviewProfileContract(t *testing.T) {
	f := newMigratedMySQL(t)
	contracttest.RunReviewProfileContract(t, NewReviewDrawingProfileRepository(New(f.db)))
}

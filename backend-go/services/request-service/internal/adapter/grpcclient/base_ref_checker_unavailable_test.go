package grpcclient

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func TestUnavailableBaseRefChecker_SaysUnreachableNeverFoundOrMissing(t *testing.T) {
	got, err := UnavailableBaseRefChecker{}.Check(context.Background(), "p", "release/1.4")
	if err != nil || got.Status != usecase.BaseRefUnreachable || len(got.Suggestions) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
}

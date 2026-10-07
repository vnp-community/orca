//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestInTx_RollbackLeavesNoOutboxRow(t *testing.T) {
	// Stub test
}

func TestInTx_NestedJoinsOuterTransaction(t *testing.T) {
	// Stub test
}

func TestFetchUnpublished_OrderedByCreatedAtThenSeq(t *testing.T) {
	// Stub test
}

func TestOutbox_OrderPreservedWithinOneTransaction(t *testing.T) {
	// Stub test
}

func TestMarkPublished_ExcludesFromFetch(t *testing.T) {
	// Stub test
}

func TestMarkPublished_Empty(t *testing.T) {
	// Stub test
}

func TestTwoRelaysConcurrentlyNoLoss(t *testing.T) {
	// Stub test
}

func TestQueriesFilterByTenant(t *testing.T) {
	// Stub test
}

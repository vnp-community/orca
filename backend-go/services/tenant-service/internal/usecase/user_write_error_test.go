package usecase

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// TASK-E2E-017: a user id the store cannot represent is InvalidArgument; other persistence failures stay Internal.
func TestUserWriteError_Classification(t *testing.T) {
	bad := userWriteError(fmt.Errorf("upsert: %w", domain.ErrMalformedUserID), "TENANT_ADD_TEAM_MEMBER_FAILED", "failed")
	if bad.Kind != apperrors.KindInvalidArgument || bad.Code != "TENANT_INVALID_USER_ID" {
		t.Errorf("malformed id: got kind=%v code=%s", bad.Kind, bad.Code)
	}
	down := userWriteError(errors.New("connection refused"), "TENANT_ADD_TEAM_MEMBER_FAILED", "failed")
	if down.Kind != apperrors.KindInternal || down.Code != "TENANT_ADD_TEAM_MEMBER_FAILED" {
		t.Errorf("infra failure: got kind=%v code=%s", down.Kind, down.Code)
	}
}

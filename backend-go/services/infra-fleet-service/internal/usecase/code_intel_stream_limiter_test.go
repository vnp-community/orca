package usecase

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestCodeIntelStreamLimiter_EnforcesLimitAndReleases(t *testing.T) {
	limiter := NewCodeIntelStreamLimiter(16)
	key := "tenant-1|ds-1"

	var releases []func()
	for i := 0; i < 16; i++ {
		rel, err := limiter.Acquire(key)
		if err != nil {
			t.Fatalf("acquire slot %d failed: %v", i, err)
		}
		releases = append(releases, rel)
	}

	// 17th acquire must fail
	_, err := limiter.Acquire(key)
	if err == nil {
		t.Fatal("expected 17th acquire to fail")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindFailedPrecondition || appErr.Code != "INFRA_CODEINTEL_STREAM_LIMIT" {
		t.Fatalf("expected KindFailedPrecondition and INFRA_CODEINTEL_STREAM_LIMIT, got %v", err)
	}

	// Different key is independent
	relOther, err := limiter.Acquire("tenant-1|ds-2")
	if err != nil {
		t.Fatalf("acquire for different dev server failed: %v", err)
	}
	relOther()

	// Release one slot for key
	releases[0]()
	// Idempotent release
	releases[0]()

	// Now another acquire should succeed
	relNew, err := limiter.Acquire(key)
	if err != nil {
		t.Fatalf("acquire after release failed: %v", err)
	}
	relNew()

	for _, rel := range releases[1:] {
		rel()
	}
}

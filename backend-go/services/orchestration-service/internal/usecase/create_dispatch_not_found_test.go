package usecase

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

// TASK-E2E-017: a dangling run/task id is the caller's mistake (NotFound); other repo failures stay Internal.
func TestCreateDispatchContext_RepositoryErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		repoErr  error
		wantKind apperrors.Kind
		wantCode string
	}{
		{"run missing", fmt.Errorf("insert: %w", ErrRunNotFound), apperrors.KindNotFound, "ORCH_RUN_NOT_FOUND"},
		{"task missing", fmt.Errorf("insert: %w", ErrTaskNotFound), apperrors.KindNotFound, "ORCH_TASK_NOT_FOUND"},
		{"infrastructure", errors.New("connection refused"), apperrors.KindInternal, "ORCH_CREATE_DISPATCH_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewCreateDispatchContext(&fakeDispatchContextRepository{err: tc.repoErr}, &synchronousSerializer{}, nil)
			_, err := uc.Execute(withTenant(t.Context(), "tenant-1"), CreateDispatchContextInput{Handle: "h", CoordinatorRunID: "run-1"})
			var appErr *apperrors.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("want AppError, got %v", err)
			}
			if appErr.Kind != tc.wantKind || appErr.Code != tc.wantCode {
				t.Errorf("got kind=%v code=%s, want kind=%v code=%s", appErr.Kind, appErr.Code, tc.wantKind, tc.wantCode)
			}
		})
	}
}

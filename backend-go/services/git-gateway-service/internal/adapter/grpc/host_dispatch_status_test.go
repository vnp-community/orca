package grpc

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

func TestToStatus_SurfacesHostDispatchCodeThroughGenericWrapper(t *testing.T) {
	cases := []struct {
		name     string
		inner    error
		wantCode codes.Code
		wantText string
	}{
		{"unbound", usecase.ErrRepoHostUnbound(), codes.FailedPrecondition, "GITGATEWAY_REPO_HOST_UNBOUND"},
		{"unreachable", usecase.ErrDevServerUnreachable("ds-1"), codes.Unavailable, "GITGATEWAY_DEV_SERVER_UNREACHABLE"},
		{"check failed", usecase.ErrHostCheckFailed(errors.New("x")), codes.Unavailable, "GITGATEWAY_HOST_CHECK_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", tc.inner)
			st := status.Convert(toStatus(wrapped))
			if st.Code() != tc.wantCode || !strings.Contains(st.Message(), tc.wantText) {
				t.Fatalf("got %v %q, want %v containing %q", st.Code(), st.Message(), tc.wantCode, tc.wantText)
			}
			if strings.Contains(st.Message(), "WORKTREE_RESOLVE_FAILED") {
				t.Errorf("generic wrapper code leaked: %q", st.Message())
			}
		})
	}
}

func TestToStatus_OrdinaryErrorsKeepOuterCode(t *testing.T) {
	err := apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", "failed", errors.New("boom"))
	st := status.Convert(toStatus(err))
	if st.Code() != codes.Internal || !strings.HasPrefix(st.Message(), "WORKTREE_CREATE_FAILED") {
		t.Fatalf("got %v %q", st.Code(), st.Message())
	}
}

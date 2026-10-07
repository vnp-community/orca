package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

type mockAuthorizer struct {
	allowed map[string]bool
	err     error
	calls   int
}

func (m *mockAuthorizer) AuthorizeWorktree(ctx context.Context, tenantID, projectID, worktreeRef, action string) (bool, error) {
	m.calls++
	if m.err != nil {
		return false, m.err
	}
	key := tenantID + ":" + projectID + ":" + worktreeRef
	return m.allowed[key], nil
}

func TestPushAuthorization_AuthorizeSelectors_RejectsUnauthorized(t *testing.T) {
	mock := &mockAuthorizer{
		allowed: map[string]bool{
			"t1:p1:w1": true,
			"t1:p1:w2": false, // unauthorized
		},
	}
	authz := NewPushAuthorization(mock, 10*time.Second)

	req := []*codeintelv1.WorktreeSelector{
		{ProjectId: "p1", WorktreeRef: "w1"},
		{ProjectId: "p1", WorktreeRef: "w2"},
	}

	err := authz.AuthorizeSelectors(context.Background(), "t1", req)
	if err == nil {
		t.Fatalf("expected PermissionDenied error, got nil")
	}

	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Errorf("expected KindPermissionDenied, got %v", err)
	}
}

func TestPushAuthorization_CanRead_CachesAndFailsClosedOnError(t *testing.T) {
	mock := &mockAuthorizer{
		allowed: map[string]bool{"t1:p1:w1": true},
	}
	authz := NewPushAuthorization(mock, 10*time.Second)

	// 1. Initial read succeeds
	if !authz.CanRead(context.Background(), "t1", "p1", "w1") {
		t.Fatalf("expected access allowed")
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 call, got %d", mock.calls)
	}

	// 2. Second read within cache window -> no additional authorizer call
	if !authz.CanRead(context.Background(), "t1", "p1", "w1") {
		t.Fatalf("expected access allowed from cache")
	}
	if mock.calls != 1 {
		t.Errorf("expected still 1 call due to cache, got %d", mock.calls)
	}

	// 3. Authorizer error -> fails closed (false)
	mockErr := &mockAuthorizer{err: errors.New("OPA timeout")}
	authzErr := NewPushAuthorization(mockErr, 10*time.Second)

	if authzErr.CanRead(context.Background(), "t1", "p1", "w1") {
		t.Errorf("expected lookup error to deny access (fail-closed)")
	}
}

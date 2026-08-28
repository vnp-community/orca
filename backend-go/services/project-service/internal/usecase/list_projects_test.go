package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestListProjects_EmptyPageToken_Succeeds(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, ListProjectsInput{})
	if err != nil {
		t.Fatalf("unexpected error with empty PageToken: %v", err)
	}
}

func TestListProjects_MalformedPageToken_ReturnsInvalidArgument(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	_, err := uc.Execute(ctx, ListProjectsInput{PageToken: "not-a-uuid"})
	assertAppError(t, err, apperrors.KindInvalidArgument, "PROJECT_INVALID_PAGE_TOKEN")
}

func TestListProjects_ValidUUIDPageToken_ReachesRepository(t *testing.T) {
	repo := newFakeProjectRepository()
	uc := NewListProjects(repo)

	ctx := withTenantAndUser(context.Background(), "tenant-1", "user-1")
	// A syntactically valid but nonexistent cursor should reach the
	// repository (fake or real) rather than being rejected by validation —
	// only non-UUID-shaped tokens should be.
	_, err := uc.Execute(ctx, ListProjectsInput{PageToken: "00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatalf("unexpected error with a well-formed (if nonexistent) cursor: %v", err)
	}
}

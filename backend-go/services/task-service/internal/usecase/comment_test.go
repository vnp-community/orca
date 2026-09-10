package usecase

import (
	"context"
	"testing"
)

func TestAddComment_RequiresTenantContext(t *testing.T) {
	uc := NewAddComment(&fakeCommentRepository{})
	if _, err := uc.Execute(context.Background(), "task-1", "hello"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestAddComment_RejectsEmptyContent(t *testing.T) {
	uc := NewAddComment(&fakeCommentRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, "task-1", ""); err == nil {
		t.Fatal("expected an error for empty content")
	}
}

func TestAddComment_PersistsAndReturnsComment(t *testing.T) {
	comments := &fakeCommentRepository{}
	uc := NewAddComment(comments)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, "task-1", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TaskID != "task-1" || got.AuthorID != "user-1" || got.Content != "hello world" {
		t.Errorf("unexpected comment: %+v", got)
	}
	if len(comments.comments) != 1 {
		t.Fatalf("expected 1 persisted comment, got %d", len(comments.comments))
	}
}

func TestListComments_RequiresTenantContext(t *testing.T) {
	uc := NewListComments(&fakeCommentRepository{})
	if _, _, err := uc.Execute(context.Background(), "task-1", "", 0); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListComments_ReturnsOnlyMatchingTask(t *testing.T) {
	comments := &fakeCommentRepository{}
	addUC := NewAddComment(comments)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := addUC.Execute(ctx, "task-1", "first"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := addUC.Execute(ctx, "task-2", "other task"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listUC := NewListComments(comments)
	got, _, err := listUC.Execute(ctx, "task-1", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Content != "first" {
		t.Errorf("unexpected comments: %+v", got)
	}
}

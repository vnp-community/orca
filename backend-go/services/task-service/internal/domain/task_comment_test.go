package domain

import "testing"

func TestNewTaskComment_RejectsEmptyContent(t *testing.T) {
	if _, err := NewTaskComment("c1", "task-1", "user-1", ""); err != ErrEmptyCommentBody {
		t.Fatalf("expected ErrEmptyCommentBody, got %v", err)
	}
}

func TestNewTaskComment_ValidComment(t *testing.T) {
	c, err := NewTaskComment("c1", "task-1", "user-1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != "c1" || c.TaskID != "task-1" || c.AuthorID != "user-1" || c.Content != "hello" {
		t.Errorf("unexpected comment: %+v", c)
	}
}

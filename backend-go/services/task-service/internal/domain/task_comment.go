package domain

import (
	"errors"
	"time"
)

// TaskComment is one task.task_comments row — mirrors task_edge.go's
// minimal-invariant-constructor shape. See SOL-TG-01's AddComment/
// ListComments design.
type TaskComment struct {
	ID        string
	TaskID    string
	AuthorID  string
	Content   string
	CreatedAt time.Time
}

// ErrEmptyCommentBody guards against a content-less comment.
var ErrEmptyCommentBody = errors.New("domain: comment content is required")

// NewTaskComment constructs a TaskComment, enforcing the one invariant that
// matters for a comment: non-empty content.
func NewTaskComment(id, taskID, authorID, content string) (TaskComment, error) {
	if content == "" {
		return TaskComment{}, ErrEmptyCommentBody
	}
	return TaskComment{ID: id, TaskID: taskID, AuthorID: authorID, Content: content}, nil
}

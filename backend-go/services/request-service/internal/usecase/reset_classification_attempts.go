package usecase

import (
	"context"
)

// ClassificationAttemptsReset zeroes classification_attempts on reopen so a reopened Request
// gets a fresh AI budget. It runs in the reopen transaction and re-reads the row, because the
// transition just bumped the version.
type ClassificationAttemptsReset struct{ repo RequestRepository }

var _ ClassificationAttemptsResetter = (*ClassificationAttemptsReset)(nil)

func NewClassificationAttemptsReset(repo RequestRepository) *ClassificationAttemptsReset {
	return &ClassificationAttemptsReset{repo: repo}
}

func (u *ClassificationAttemptsReset) ResetClassificationAttempts(ctx context.Context, requestID string) error {
	r, err := u.repo.Get(ctx, requestID)
	if err != nil || r.ClassificationAttempts == 0 {
		return err
	}
	r.ClassificationAttempts = 0
	_, err = u.repo.Update(ctx, r, r.Version)
	return err
}

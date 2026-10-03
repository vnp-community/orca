//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"
)

// backdateLink pretends a link started (and, if it ended, ended) `ago` ago.
func backdateLink(t *testing.T, repo *Repository, linkID string, ago time.Duration) {
	t.Helper()
	_, err := repo.pool.ExecContext(context.Background(), `
		UPDATE execution_links
		SET started_at = DATE_SUB(NOW(6), INTERVAL ? MICROSECOND),
		    completed_at = CASE WHEN completed_at IS NULL THEN NULL ELSE DATE_SUB(NOW(6), INTERVAL ? MICROSECOND) END
		WHERE id = ?
	`, ago.Microseconds(), ago.Microseconds(), linkID)
	if err != nil {
		t.Fatalf("backdate link: %v", err)
	}
}

// backdateTask pretends a task was last touched `ago` ago.
func backdateTask(t *testing.T, repo *Repository, taskID string, ago time.Duration) {
	t.Helper()
	_, err := repo.pool.ExecContext(context.Background(), `
		UPDATE tasks SET updated_at = DATE_SUB(NOW(6), INTERVAL ? MICROSECOND) WHERE id = ?
	`, ago.Microseconds(), taskID)
	if err != nil {
		t.Fatalf("backdate task: %v", err)
	}
}

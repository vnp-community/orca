//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"
)

// backdateLink pretends a link started (and, if it ended, ended) `ago` ago.
func backdateLink(t *testing.T, repo *Repository, linkID string, ago time.Duration) {
	t.Helper()
	_, err := repo.pool.Exec(context.Background(), `
		UPDATE task.execution_links
		SET started_at = now() - make_interval(secs => $1),
		    completed_at = CASE WHEN completed_at IS NULL THEN NULL ELSE now() - make_interval(secs => $1) END
		WHERE id = $2
	`, ago.Seconds(), linkID)
	if err != nil {
		t.Fatalf("backdate link: %v", err)
	}
}

// backdateTask pretends a task was last touched `ago` ago.
func backdateTask(t *testing.T, repo *Repository, taskID string, ago time.Duration) {
	t.Helper()
	_, err := repo.pool.Exec(context.Background(), `
		UPDATE task.tasks SET updated_at = now() - make_interval(secs => $1) WHERE id = $2
	`, ago.Seconds(), taskID)
	if err != nil {
		t.Fatalf("backdate task: %v", err)
	}
}

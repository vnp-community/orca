package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type stuckDiff struct{}

func (stuckDiff) BranchCompare(ctx context.Context, _, _ string) (ReviewDiff, error) {
	<-ctx.Done()
	return ReviewDiff{}, ctx.Err()
}

// A worktree whose diff never comes must cost one bounded wait and a partial fact, never the whole request.
func TestCollectDiff_SlowWorktreeIsAPartialFactNotAFailure(t *testing.T) {
	old := reviewDiffReadTimeout
	reviewDiffReadTimeout = 20 * time.Millisecond
	defer func() { reviewDiffReadTimeout = old }()

	c := &ReviewFactsCollector{Diff: stuckDiff{}}
	var f domain.Facts
	tree := domain.ExecutionTree{Leaves: []domain.TaskView{{ID: "t1", WorktreeID: "wt-1"}}}
	start := time.Now()
	c.collectDiff(context.Background(), domain.Request{ID: "r1"}, tree, &f)

	if time.Since(start) > 2*time.Second {
		t.Fatalf("the read was not bounded: %v", time.Since(start))
	}
	found := false
	for _, w := range f.Warnings {
		found = found || w == "partial_facts:diff:t1"
	}
	if !found {
		t.Fatalf("warnings = %v, want partial_facts:diff:t1", f.Warnings)
	}
}

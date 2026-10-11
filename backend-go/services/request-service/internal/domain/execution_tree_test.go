package domain

import "testing"

func phaseTree(statuses map[string]string, order ...string) ExecutionTree {
	var tasks []TaskView
	for _, id := range order {
		tasks = append(tasks, TaskView{ID: id, Type: TaskTypePhase, Status: statuses[id]})
	}
	return BuildExecutionTree(tasks)
}

func TestNextReadyPhase(t *testing.T) {
	open := map[string]string{"a": TaskStatusOpen, "b": TaskStatusOpen}
	bWaitsForA := []DependsEdge{{From: "b", To: "a"}}

	t.Run("without edges it is the first unstarted phase, like NextPhase", func(t *testing.T) {
		tree := phaseTree(open, "b", "a")
		got, ok := tree.NextReadyPhase(nil, nil)
		want, _ := tree.NextPhase(nil)
		if !ok || got.ID != want.ID || got.ID != "b" {
			t.Fatalf("got %v %v, want %s", got.ID, ok, want.ID)
		}
	})
	t.Run("a phase that waits for an unfinished phase is skipped even when listed first", func(t *testing.T) {
		got, ok := phaseTree(open, "b", "a").NextReadyPhase(nil, bWaitsForA)
		if !ok || got.ID != "a" {
			t.Fatalf("got %q %v, want a", got.ID, ok)
		}
	})
	t.Run("once the predecessor is done the dependent phase is next", func(t *testing.T) {
		done := map[string]string{"a": TaskStatusDone, "b": TaskStatusOpen}
		got, ok := phaseTree(done, "b", "a").NextReadyPhase(map[string]bool{"a": true}, bWaitsForA)
		if !ok || got.ID != "b" {
			t.Fatalf("got %q %v, want b", got.ID, ok)
		}
	})
	t.Run("a cancelled predecessor does not hold the dependent phase", func(t *testing.T) {
		cancelled := map[string]string{"a": TaskStatusCancelled, "b": TaskStatusOpen}
		got, ok := phaseTree(cancelled, "b", "a").NextReadyPhase(map[string]bool{"a": true}, bWaitsForA)
		if !ok || got.ID != "b" {
			t.Fatalf("got %q %v, want b", got.ID, ok)
		}
	})
	t.Run("a started predecessor that is still running leaves no phase to open", func(t *testing.T) {
		running := map[string]string{"a": TaskStatusInProgress, "b": TaskStatusOpen}
		if got, ok := phaseTree(running, "b", "a").NextReadyPhase(map[string]bool{"a": true}, bWaitsForA); ok {
			t.Fatalf("got %q, want none", got.ID)
		}
	})
	t.Run("an edge to something that is not a phase is ignored", func(t *testing.T) {
		got, ok := phaseTree(open, "b").NextReadyPhase(nil, []DependsEdge{{From: "b", To: "some-task"}})
		if !ok || got.ID != "b" {
			t.Fatalf("got %q %v, want b", got.ID, ok)
		}
	})
}

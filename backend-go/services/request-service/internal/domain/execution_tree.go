package domain

// ExecutionTree is a Request's task-service tasks arranged as Plan, Phases and working tasks. It is built
// from one ListTasks answer, so it is a snapshot: callers re-read before acting on it twice.
type ExecutionTree struct {
	Plan   *TaskView
	Phases []TaskView
	// Leaves are the working tasks, in the order task-service listed them.
	Leaves []TaskView
	byID   map[string]TaskView
}

func BuildExecutionTree(tasks []TaskView) ExecutionTree {
	t := ExecutionTree{byID: make(map[string]TaskView, len(tasks))}
	for _, task := range tasks {
		t.byID[task.ID] = task
		switch {
		case task.Type == TaskTypePlan:
			if t.Plan == nil {
				p := task
				t.Plan = &p
			}
		case task.Type == TaskTypePhase:
			t.Phases = append(t.Phases, task)
		case task.IsWorkingTask():
			t.Leaves = append(t.Leaves, task)
		}
	}
	return t
}

func (t ExecutionTree) HasPhases() bool { return len(t.Phases) > 0 }

func (t ExecutionTree) Task(id string) (TaskView, bool) {
	v, ok := t.byID[id]
	return v, ok
}

func (t ExecutionTree) Phase(id string) (TaskView, bool) {
	for _, p := range t.Phases {
		if p.ID == id {
			return p, true
		}
	}
	return TaskView{}, false
}

// LeavesOf lists the working tasks hanging directly under containerID.
func (t ExecutionTree) LeavesOf(containerID string) []TaskView {
	var out []TaskView
	for _, l := range t.Leaves {
		if l.ParentID == containerID {
			out = append(out, l)
		}
	}
	return out
}

// Container returns the task's direct parent when it is a phase or the plan.
func (t ExecutionTree) Container(task TaskView) (*TaskView, bool) {
	if task.ParentID == "" {
		return nil, false
	}
	p, ok := t.byID[task.ParentID]
	if !ok || !IsContainerTaskType(p.Type) {
		return nil, false
	}
	return &p, true
}

// InScope reports whether a working task takes part in execution right now: under a started Phase, directly
// under the Plan of a Request that has no Phases, or with no container at all (hotfix).
func (t ExecutionTree) InScope(task TaskView, started map[string]bool) bool {
	c, ok := t.Container(task)
	switch {
	case !ok:
		return task.ParentID == ""
	case c.Type == TaskTypePhase:
		return started[c.ID]
	default:
		return !t.HasPhases()
	}
}

func taskFinished(status string) bool {
	return status == TaskStatusDone || status == TaskStatusCancelled
}

// ScopeFinished is true when every working task is done or cancelled and at least one is done.
// With Phases it also needs every Phase finished, so an unstarted Phase keeps the Request open.
func (t ExecutionTree) ScopeFinished() bool {
	done := false
	for _, l := range t.Leaves {
		// A finished task never holds the Request open, even outside a recorded start (run by hand).
		if !taskFinished(l.Status) {
			return false // unfinished work, or work in a Phase nobody started yet
		}
		if l.Status == TaskStatusDone {
			done = true
		}
	}
	for _, p := range t.Phases {
		if !taskFinished(p.Status) {
			return false
		}
	}
	return done
}

// NextPhase is the first Phase, in listing order, that has not finished and has not been started.
func (t ExecutionTree) NextPhase(started map[string]bool) (TaskView, bool) {
	for _, p := range t.Phases {
		if !taskFinished(p.Status) && !started[p.ID] {
			return p, true
		}
	}
	return TaskView{}, false
}

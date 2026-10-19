package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PlanLimits bound what a model (or a client committing a proposal) can make task-service create.
type PlanLimits struct {
	MaxPhases            int
	MaxTasksPerContainer int
	MaxTasksTotal        int
	MaxTitleRunes        int
}

// DefaultPlanLimits are unmeasured proposals (SOL-012 section 2.3); operators tune them with REQUEST_PLAN_MAX_*.
var DefaultPlanLimits = PlanLimits{MaxPhases: 8, MaxTasksPerContainer: 20, MaxTasksTotal: 100, MaxTitleRunes: 200}

const (
	maxPlanTextRunes     = 8000
	maxPlanTaskHours     = 1000.0
	defaultPlanTaskType  = "task"
	planProposalSummaryN = 4000
)

// planTaskTypes are the leaf task types task-service accepts; plan and phase are containers the shape creates itself.
var planTaskTypes = map[string]bool{"task": true, "bug": true, "feature": true, "epic": true}

// NormalizePlanProposal trims text, defaults the task type and cleans labels, so validation and storage see one form.
func NormalizePlanProposal(p PlanProposal) PlanProposal {
	p.Title, p.Summary, p.Notes = strings.TrimSpace(p.Title), strings.TrimSpace(p.Summary), strings.TrimSpace(p.Notes)
	p.Tasks = normalizeProposalTasks(p.Tasks)
	phases := make([]PhaseProposal, len(p.Phases))
	for i, ph := range p.Phases {
		ph.Title, ph.Description = strings.TrimSpace(ph.Title), strings.TrimSpace(ph.Description)
		ph.Tasks = normalizeProposalTasks(ph.Tasks)
		phases[i] = ph
	}
	p.Phases = phases
	return p
}

func normalizeProposalTasks(in []TaskProposal) []TaskProposal {
	if len(in) == 0 {
		return nil
	}
	out := make([]TaskProposal, len(in))
	for i, t := range in {
		t.Title, t.Description, t.PromptTemplate = strings.TrimSpace(t.Title), strings.TrimSpace(t.Description), strings.TrimSpace(t.PromptTemplate)
		t.TaskType = strings.ToLower(strings.TrimSpace(t.TaskType))
		if t.TaskType == "" {
			t.TaskType = defaultPlanTaskType
		}
		seen := map[string]bool{}
		var labels []string
		for _, l := range t.Labels {
			l = strings.ToLower(strings.TrimSpace(l))
			if l != "" && !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
		t.Labels = labels
		out[i] = t
	}
	return out
}

// ValidatePlanProposal checks a normalized proposal against the shape of the Request's type and the limits.
// It runs on the model's answer and again on the proposal a client commits: neither is trusted.
func ValidatePlanProposal(reqType RequestType, shape PlanShape, lim PlanLimits, p PlanProposal) error {
	if shape == ShapeNone {
		return ErrPlanNotApplicable(reqType)
	}
	if err := checkTitle("plan title", p.Title, lim); err != nil {
		return err
	}
	if utf8.RuneCountInString(p.Summary) > planProposalSummaryN || utf8.RuneCountInString(p.Notes) > planProposalSummaryN {
		return ErrPlanInvalidField(fmt.Sprintf("summary and notes are limited to %d characters", planProposalSummaryN))
	}
	switch {
	case shape.AllowsPhases():
		if len(p.Tasks) > 0 {
			return ErrPlanMixedChildren()
		}
		if len(p.Phases) == 0 {
			return ErrPlanPhasesRequired()
		}
	case len(p.Phases) > 0:
		if len(p.Tasks) > 0 {
			return ErrPlanMixedChildren()
		}
		return ErrPlanPhasesNotAllowed()
	case len(p.Tasks) == 0:
		return ErrPlanEmpty("the Plan")
	}
	if len(p.Phases) > lim.MaxPhases {
		return ErrPlanTooLarge(fmt.Sprintf("%d Phases exceed the limit of %d", len(p.Phases), lim.MaxPhases))
	}

	total := len(p.Tasks)
	if err := checkContainer("the Plan", p.Tasks, lim); err != nil {
		return err
	}
	phaseDeps := make([][]int, len(p.Phases))
	for i, ph := range p.Phases {
		where := fmt.Sprintf("Phase %d", i+1)
		if err := checkTitle(where+" title", ph.Title, lim); err != nil {
			return err
		}
		if utf8.RuneCountInString(ph.Description) > maxPlanTextRunes {
			return ErrPlanInvalidField(where + " description is too long")
		}
		if len(ph.Tasks) == 0 {
			return ErrPlanEmpty(where)
		}
		if err := checkContainer(where, ph.Tasks, lim); err != nil {
			return err
		}
		total += len(ph.Tasks)
		phaseDeps[i] = ph.DependsOnPhaseIndices
	}
	if total > lim.MaxTasksTotal {
		return ErrPlanTooLarge(fmt.Sprintf("%d tasks exceed the limit of %d", total, lim.MaxTasksTotal))
	}
	if err := checkAcyclic("Phase", phaseDeps); err != nil {
		return err
	}
	if (reqType == RequestTypeBug || reqType == RequestTypeSecurity) && !hasRegressionTest(p) {
		return ErrPlanRegressionTestMissing()
	}
	return nil
}

func checkContainer(where string, tasks []TaskProposal, lim PlanLimits) error {
	if len(tasks) > lim.MaxTasksPerContainer {
		return ErrPlanTooLarge(fmt.Sprintf("%s has %d tasks, the limit is %d", where, len(tasks), lim.MaxTasksPerContainer))
	}
	deps := make([][]int, len(tasks))
	for i, t := range tasks {
		name := fmt.Sprintf("task %d of %s", i+1, where)
		if err := checkTitle(name+" title", t.Title, lim); err != nil {
			return err
		}
		if !planTaskTypes[t.TaskType] {
			return ErrPlanInvalidTaskType(t.TaskType)
		}
		if utf8.RuneCountInString(t.Description) > maxPlanTextRunes || utf8.RuneCountInString(t.PromptTemplate) > maxPlanTextRunes {
			return ErrPlanInvalidField(name + " text is too long")
		}
		if t.EstimatedHours < 0 || t.EstimatedHours > maxPlanTaskHours {
			return ErrPlanInvalidField(fmt.Sprintf("%s has estimated_hours %v; use 0 to %v", name, t.EstimatedHours, maxPlanTaskHours))
		}
		if len(t.Labels) > maxLabelsPerTask {
			return ErrPlanInvalidField(fmt.Sprintf("%s has more than %d labels", name, maxLabelsPerTask))
		}
		for _, l := range t.Labels {
			if utf8.RuneCountInString(l) > maxLabelRunes {
				return ErrPlanInvalidField(fmt.Sprintf("%s has a label longer than %d characters", name, maxLabelRunes))
			}
		}
		deps[i] = t.DependsOnIndices
	}
	return checkAcyclic("task in "+where, deps)
}

func checkTitle(name, title string, lim PlanLimits) error {
	n := utf8.RuneCountInString(title)
	if n == 0 {
		return ErrPlanInvalidTitle(name + " is required")
	}
	if n > lim.MaxTitleRunes {
		return ErrPlanInvalidTitle(fmt.Sprintf("%s is longer than %d characters", name, lim.MaxTitleRunes))
	}
	return nil
}

// checkAcyclic proves every dependency index points at a sibling other than the item itself and that the graph has
// no cycle (Kahn: whatever cannot be peeled off sits on a cycle). task-service checks again when it writes the edges.
func checkAcyclic(what string, deps [][]int) error {
	n := len(deps)
	indegree := make([]int, n)
	out := make([][]int, n)
	for i, ds := range deps {
		seen := map[int]bool{}
		for _, d := range ds {
			switch {
			case d < 0 || d >= n:
				return ErrPlanInvalidDependency(fmt.Sprintf("%s %d depends on index %d, which does not exist", what, i+1, d))
			case d == i:
				return ErrPlanInvalidDependency(fmt.Sprintf("%s %d depends on itself", what, i+1))
			case seen[d]:
				continue
			}
			seen[d] = true
			out[d] = append(out[d], i)
			indegree[i]++
		}
	}
	var queue []int
	for i, d := range indegree {
		if d == 0 {
			queue = append(queue, i)
		}
	}
	peeled := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		peeled++
		for _, next := range out[cur] {
			if indegree[next]--; indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if peeled != n {
		return ErrPlanInvalidDependency(fmt.Sprintf("%ss depend on each other in a cycle", what))
	}
	return nil
}

func hasRegressionTest(p PlanProposal) bool {
	for _, t := range flattenProposalTasks(p) {
		if containsString(t.Labels, PlanLabelTestRegression) {
			return true
		}
	}
	return false
}

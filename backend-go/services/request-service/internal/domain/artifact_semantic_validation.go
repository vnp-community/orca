package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Semantic codes beyond the schema ones (CR-REQ-027 section 2.8). They run after structure checks, server side.
const (
	CodeArtifactDependencyCycle   = "REQUEST_ARTIFACT_DEPENDENCY_CYCLE"
	CodeArtifactUnknownAC         = "REQUEST_ARTIFACT_UNKNOWN_AC"
	CodeArtifactACNotCovered      = "REQUEST_ARTIFACT_AC_NOT_COVERED"
	CodeArtifactTaskNoCheck       = "REQUEST_ARTIFACT_TASK_NO_CHECK"
	CodeArtifactTaskNoAC          = "REQUEST_ARTIFACT_TASK_NO_AC"
	CodeArtifactACUncoveredOption = "REQUEST_ARTIFACT_AC_UNCOVERED_BY_OPTION"
	CodeArtifactExemptNotAllowed  = "REQUEST_ARTIFACT_EXEMPT_NOT_ALLOWED"

	// Proposed caps, not measured.
	MaxPlanTasks     = 200
	MaxChecksPerTask = 20
	LabelRollback    = "rollback"
	LabelCheckPrefix = "check:"
)

// CoverageEntry mirrors solution.requirement_coverage[].
type CoverageEntry struct {
	ACID      string   `json:"ac_id"`
	OptionIDs []string `json:"option_ids"`
	Status    string   `json:"status"`
	Note      string   `json:"note"`
}

// SolutionCoverageView is the slice of a solution document the AC check needs.
type SolutionCoverageView struct {
	OptionIDs           []string
	RequirementCoverage []CoverageEntry
}

// ParseSolutionCoverage reads option ids and requirement_coverage from a solution options document.
func ParseSolutionCoverage(raw []byte) (SolutionCoverageView, error) {
	var doc struct {
		Options []struct {
			ID string `json:"id"`
		} `json:"options"`
		Coverage []CoverageEntry `json:"requirement_coverage"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return SolutionCoverageView{}, ErrRequestContentInvalid("solution options: " + err.Error())
	}
	v := SolutionCoverageView{RequirementCoverage: doc.Coverage}
	for _, o := range doc.Options {
		v.OptionIDs = append(v.OptionIDs, o.ID)
	}
	return v, nil
}

type TaskSpecView struct {
	ID                 string
	Labels             []string
	Satisfies          []string
	CheckIDs           []string
	ExemptFromCoverage bool
}

type PhaseSpecView struct{ ID string }

// DependsEdge is a depends_on edge: From waits for To. Both ends are tasks or both are phases.
type DependsEdge struct{ From, To string }

type PlanSpecs struct {
	Tasks      []TaskSpecView
	Phases     []PhaseSpecView
	TaskEdges  []DependsEdge
	PhaseEdges []DependsEdge
}

type ArtifactContext struct {
	Request        RequestContent
	Solution       *SolutionCoverageView
	ChosenOptionID string
	Plan           *PlanSpecs
}

// ValidateArtifactSemantics is pure: it reports every violation, in a stable order.
func ValidateArtifactSemantics(c ArtifactContext) []Violation {
	var out []Violation
	add := func(path, code, msg string) { out = append(out, Violation{Path: path, Code: code, Message: msg}) }

	active := map[string]bool{}
	for _, it := range c.Request.AcceptanceCriteria.Items {
		if it.Status == ACStatusActive {
			active[it.ID] = true
		}
	}
	if len(c.Request.AcceptanceCriteria.Items) > MaxAcceptanceCriteria {
		add("/acceptance_criteria", CodeArtifactLimitExceeded, fmt.Sprintf("more than %d acceptance criteria", MaxAcceptanceCriteria))
	}
	if c.Solution != nil {
		out = append(out, optionCoverageViolations(c, active)...)
	}
	if c.Plan != nil {
		out = append(out, planViolations(c.Plan, active)...)
	}
	return finalizeViolations(out)
}

func optionCoverageViolations(c ArtifactContext, active map[string]bool) []Violation {
	var out []Violation
	for i, e := range c.Solution.RequirementCoverage {
		if !active[e.ACID] {
			out = append(out, Violation{Path: fmt.Sprintf("/requirement_coverage/%d/ac_id", i), Code: CodeArtifactUnknownAC,
				Message: fmt.Sprintf("%s is not an active acceptance criterion", e.ACID)})
		}
	}
	if c.ChosenOptionID == "" {
		return out
	}
	for _, ac := range sortedKeys(active) {
		covered := false
		for i, e := range c.Solution.RequirementCoverage {
			if e.ACID != ac || !(len(e.OptionIDs) == 0 || containsString(e.OptionIDs, c.ChosenOptionID)) {
				continue
			}
			switch e.Status {
			case "covered", "partial":
				covered = true
			case "out_of_scope":
				if strings.TrimSpace(e.Note) == "" {
					out = append(out, Violation{Path: fmt.Sprintf("/requirement_coverage/%d/note", i), Code: CodeArtifactACUncoveredOption,
						Message: ac + " is out_of_scope and needs a note"})
				} else {
					covered = true
				}
			}
		}
		if !covered {
			out = append(out, Violation{Path: "/requirement_coverage", Code: CodeArtifactACUncoveredOption,
				Message: fmt.Sprintf("the chosen option %s does not answer %s", c.ChosenOptionID, ac)})
		}
	}
	return out
}

func planViolations(p *PlanSpecs, active map[string]bool) []Violation {
	var out []Violation
	if len(p.Tasks) > MaxPlanTasks {
		out = append(out, Violation{Path: "/tasks", Code: CodeArtifactLimitExceeded, Message: fmt.Sprintf("more than %d tasks", MaxPlanTasks)})
	}
	covered := map[string]bool{}
	for _, t := range p.Tasks {
		base := "/tasks/" + t.ID
		if len(t.CheckIDs) > MaxChecksPerTask {
			out = append(out, Violation{Path: base + "/checks", Code: CodeArtifactLimitExceeded, Message: fmt.Sprintf("more than %d checks", MaxChecksPerTask)})
		}
		for i, ac := range t.Satisfies {
			if !active[ac] {
				out = append(out, Violation{Path: fmt.Sprintf("%s/satisfies/%d", base, i), Code: CodeArtifactUnknownAC,
					Message: fmt.Sprintf("%s is unknown or retired", ac)})
			}
		}
		if t.ExemptFromCoverage {
			if !exemptAllowed(t.Labels) {
				out = append(out, Violation{Path: base + "/exempt_from_coverage", Code: CodeArtifactExemptNotAllowed,
					Message: "only tasks labelled rollback or check:* may be exempt"})
			}
			continue
		}
		if len(t.Satisfies) == 0 {
			out = append(out, Violation{Path: base + "/satisfies", Code: CodeArtifactTaskNoAC, Message: "task satisfies no acceptance criterion"})
		}
		if len(t.CheckIDs) == 0 {
			out = append(out, Violation{Path: base + "/checks", Code: CodeArtifactTaskNoCheck, Message: "task has no check"})
		}
		for _, ac := range t.Satisfies {
			covered[ac] = true
		}
	}
	for _, ac := range sortedKeys(active) {
		if !covered[ac] {
			out = append(out, Violation{Path: "/tasks", Code: CodeArtifactACNotCovered, Message: ac + " is covered by no task"})
		}
	}
	if id, ok := firstCycleNode(p.TaskEdges); ok {
		out = append(out, Violation{Path: "/tasks/" + id, Code: CodeArtifactDependencyCycle, Message: "dependency cycle through task " + id})
	}
	if id, ok := firstCycleNode(p.PhaseEdges); ok {
		out = append(out, Violation{Path: "/phases/" + id, Code: CodeArtifactDependencyCycle, Message: "dependency cycle through phase " + id})
	}
	return out
}

func exemptAllowed(labels []string) bool {
	for _, l := range labels {
		if l == LabelRollback || strings.HasPrefix(l, LabelCheckPrefix) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// firstCycleNode returns the smallest node (by id) that lies on a cycle, so the report is stable.
func firstCycleNode(edges []DependsEdge) (string, bool) {
	adj := map[string][]string{}
	nodes := map[string]bool{}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		nodes[e.From], nodes[e.To] = true, true
	}
	for k := range adj {
		sort.Strings(adj[k])
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var onCycle []string
	var dfs func(n string, stack []string) bool
	dfs = func(n string, stack []string) bool {
		color[n] = grey
		stack = append(stack, n)
		for _, m := range adj[n] {
			switch color[m] {
			case grey:
				for i := len(stack) - 1; i >= 0; i-- {
					onCycle = append(onCycle, stack[i])
					if stack[i] == m {
						break
					}
				}
				return true
			case white:
				if dfs(m, stack) {
					return true
				}
			}
		}
		color[n] = black
		return false
	}
	for _, n := range sortedKeys(nodes) {
		if color[n] == white && dfs(n, nil) {
			sort.Strings(onCycle)
			return onCycle[0], true
		}
	}
	return "", false
}

package domain

import (
	"reflect"
	"testing"
)

func semCtx(t *testing.T) ArtifactContext {
	t.Helper()
	c := RequestContent{Title: "t"}
	_, _ = c.AcceptanceCriteria.Add("a", "")
	_, _ = c.AcceptanceCriteria.Add("b", "")
	return ArtifactContext{Request: c}
}

func codesOf(vs []Violation) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Code)
	}
	return out
}

func hasCode(vs []Violation, code string) bool {
	for _, v := range vs {
		if v.Code == code {
			return true
		}
	}
	return false
}

func goodPlan() *PlanSpecs {
	return &PlanSpecs{Tasks: []TaskSpecView{
		{ID: "T1", Satisfies: []string{"AC-1"}, CheckIDs: []string{"c1"}},
		{ID: "T2", Satisfies: []string{"AC-2"}, CheckIDs: []string{"c1"}},
	}}
}

func TestSemantic_ValidPlanHasNoViolations(t *testing.T) {
	c := semCtx(t)
	c.Plan = goodPlan()
	if vs := ValidateArtifactSemantics(c); len(vs) != 0 {
		t.Fatalf("%+v", vs)
	}
}

func TestSemantic_UnknownAC(t *testing.T) {
	c := semCtx(t)
	c.Plan = goodPlan()
	c.Plan.Tasks[0].Satisfies = []string{"AC-1", "AC-9"}
	vs := ValidateArtifactSemantics(c)
	if len(vs) != 1 || vs[0].Code != CodeArtifactUnknownAC || vs[0].Path != "/tasks/T1/satisfies/1" {
		t.Fatalf("%+v", vs)
	}
}

func TestSemantic_RetiredAC(t *testing.T) {
	c := semCtx(t)
	_ = c.Request.AcceptanceCriteria.Retire("AC-2")
	c.Plan = goodPlan()
	vs := ValidateArtifactSemantics(c)
	if len(vs) != 1 || vs[0].Code != CodeArtifactUnknownAC || vs[0].Path != "/tasks/T2/satisfies/0" {
		t.Fatalf("a retired AC must be unknown to new work: %+v", vs)
	}
}

func TestSemantic_ACNotCovered_ExemptTaskExcluded(t *testing.T) {
	c := semCtx(t)
	c.Plan = goodPlan()
	c.Plan.Tasks[1] = TaskSpecView{ID: "T2", Satisfies: []string{"AC-2"}, CheckIDs: []string{"c1"}, ExemptFromCoverage: true, Labels: []string{"rollback"}}
	vs := ValidateArtifactSemantics(c)
	if len(vs) != 1 || vs[0].Code != CodeArtifactACNotCovered {
		t.Fatalf("an exempt task must not cover an AC: %+v", vs)
	}
}

func TestSemantic_TaskNoCheck(t *testing.T) {
	c := semCtx(t)
	c.Plan = goodPlan()
	c.Plan.Tasks[0].CheckIDs = nil
	vs := ValidateArtifactSemantics(c)
	if len(vs) != 1 || vs[0].Code != CodeArtifactTaskNoCheck || vs[0].Path != "/tasks/T1/checks" {
		t.Fatalf("%+v", vs)
	}
}

func TestSemantic_TaskNoAC(t *testing.T) {
	c := semCtx(t)
	c.Plan = goodPlan()
	c.Plan.Tasks = append(c.Plan.Tasks, TaskSpecView{ID: "T3", CheckIDs: []string{"c1"}})
	vs := ValidateArtifactSemantics(c)
	if len(vs) != 1 || vs[0].Code != CodeArtifactTaskNoAC || vs[0].Path != "/tasks/T3/satisfies" {
		t.Fatalf("%+v", vs)
	}
}

func TestSemantic_ExemptWithoutLabel(t *testing.T) {
	c := semCtx(t)
	c.Plan = goodPlan()
	c.Plan.Tasks = append(c.Plan.Tasks, TaskSpecView{ID: "T3", ExemptFromCoverage: true, Labels: []string{"backend"}})
	if vs := ValidateArtifactSemantics(c); len(vs) != 1 || vs[0].Code != CodeArtifactExemptNotAllowed {
		t.Fatalf("%+v", vs)
	}
	c.Plan.Tasks[2].Labels = []string{"check:lint"}
	if vs := ValidateArtifactSemantics(c); len(vs) != 0 {
		t.Fatalf("check:* is allowed: %+v", vs)
	}
}

func TestSemantic_DependencyCycle_ReportsStartNode(t *testing.T) {
	c := semCtx(t)
	c.Plan = goodPlan()
	c.Plan.TaskEdges = []DependsEdge{{"T2", "T1"}, {"T1", "T3"}, {"T3", "T2"}, {"T9", "T1"}}
	c.Plan.PhaseEdges = []DependsEdge{{"P1", "P2"}, {"P2", "P1"}}
	vs := ValidateArtifactSemantics(c)
	var cycles []Violation
	for _, v := range vs {
		if v.Code == CodeArtifactDependencyCycle {
			cycles = append(cycles, v)
		}
	}
	if len(cycles) != 2 || cycles[0].Path != "/phases/P1" || cycles[1].Path != "/tasks/T1" {
		t.Fatalf("%+v", cycles)
	}
	c.Plan.TaskEdges = []DependsEdge{{"T2", "T1"}}
	c.Plan.PhaseEdges = nil
	if vs := ValidateArtifactSemantics(c); len(vs) != 0 {
		t.Fatalf("a DAG has no cycle: %+v", vs)
	}
}

func optionView() *SolutionCoverageView {
	return &SolutionCoverageView{
		OptionIDs: []string{"opt-1", "opt-2"},
		RequirementCoverage: []CoverageEntry{
			{ACID: "AC-1", OptionIDs: []string{"opt-1"}, Status: "covered"},
			{ACID: "AC-2", OptionIDs: []string{"opt-2"}, Status: "partial"},
		},
	}
}

func TestSemantic_OptionLeavesACUncovered(t *testing.T) {
	c := semCtx(t)
	c.Solution, c.ChosenOptionID = optionView(), "opt-1"
	vs := ValidateArtifactSemantics(c)
	if len(vs) != 1 || vs[0].Code != CodeArtifactACUncoveredOption {
		t.Fatalf("opt-1 does not answer AC-2: %+v", vs)
	}
	c.ChosenOptionID = "opt-2"
	if vs := ValidateArtifactSemantics(c); len(vs) != 1 {
		t.Fatalf("opt-2 does not answer AC-1: %+v", vs)
	}
	c.ChosenOptionID = ""
	if vs := ValidateArtifactSemantics(c); len(vs) != 0 {
		t.Fatalf("nothing chosen yet: %+v", vs)
	}
}

func TestSemantic_OutOfScopeNeedsNote(t *testing.T) {
	c := semCtx(t)
	c.Solution = &SolutionCoverageView{RequirementCoverage: []CoverageEntry{
		{ACID: "AC-1", OptionIDs: []string{"opt-1"}, Status: "covered"},
		{ACID: "AC-2", Status: "out_of_scope"},
	}}
	c.ChosenOptionID = "opt-1"
	vs := ValidateArtifactSemantics(c)
	if len(vs) < 1 || !hasCode(vs, CodeArtifactACUncoveredOption) {
		t.Fatalf("%+v", vs)
	}
	c.Solution.RequirementCoverage[1].Note = "để giai đoạn sau"
	if vs := ValidateArtifactSemantics(c); len(vs) != 0 {
		t.Fatalf("%+v", vs)
	}
}

func TestSemantic_LimitExceeded(t *testing.T) {
	c := semCtx(t)
	p := &PlanSpecs{}
	for i := 0; i <= MaxPlanTasks; i++ {
		p.Tasks = append(p.Tasks, TaskSpecView{ID: "T" + string(rune('A'+i%26)) + string(rune('a'+i/26)), Satisfies: []string{"AC-1", "AC-2"}, CheckIDs: []string{"c1"}})
	}
	p.Tasks[0].CheckIDs = make([]string, MaxChecksPerTask+1)
	c.Plan = p
	vs := ValidateArtifactSemantics(c)
	n := 0
	for _, v := range vs {
		if v.Code == CodeArtifactLimitExceeded {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("want task-count and check-count limits, got %+v", vs)
	}
}

func TestSemantic_ReturnsAllViolationsDeterministically(t *testing.T) {
	c := semCtx(t)
	c.Plan = &PlanSpecs{
		Tasks: []TaskSpecView{
			{ID: "T1", Satisfies: []string{"AC-7"}},
			{ID: "T2", ExemptFromCoverage: true},
			{ID: "T3"},
		},
		TaskEdges: []DependsEdge{{"T1", "T3"}, {"T3", "T1"}},
	}
	first := ValidateArtifactSemantics(c)
	want := map[string]bool{
		CodeArtifactUnknownAC: true, CodeArtifactTaskNoCheck: true, CodeArtifactTaskNoAC: true,
		CodeArtifactExemptNotAllowed: true, CodeArtifactACNotCovered: true, CodeArtifactDependencyCycle: true,
	}
	got := map[string]bool{}
	for _, code := range codesOf(first) {
		got[code] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("codes = %v", codesOf(first))
	}
	for i := 0; i < 5; i++ {
		if !reflect.DeepEqual(first, ValidateArtifactSemantics(c)) {
			t.Fatal("order changed between runs")
		}
	}
}

func TestParseSolutionCoverage(t *testing.T) {
	raw := []byte(`{"options":[{"id":"opt-1"},{"id":"opt-2"}],"requirement_coverage":[{"ac_id":"AC-1","option_ids":["opt-1"],"status":"covered"}]}`)
	v, err := ParseSolutionCoverage(raw)
	if err != nil || len(v.OptionIDs) != 2 || len(v.RequirementCoverage) != 1 || v.RequirementCoverage[0].ACID != "AC-1" {
		t.Fatalf("%+v %v", v, err)
	}
	if _, err := ParseSolutionCoverage([]byte(`[`)); err == nil {
		t.Fatal("bad JSON must fail")
	}
}

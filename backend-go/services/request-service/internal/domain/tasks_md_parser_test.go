package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "openspec", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func violationCodes(v []Violation) []string {
	out := make([]string, len(v))
	for i, x := range v {
		out[i] = x.Code
	}
	return out
}

func hasViolation(v []Violation, code string) bool {
	for _, x := range v {
		if x.Code == code {
			return true
		}
	}
	return false
}

func TestParsePlanRegion_OKFixtureWithPhasesDependenciesAndVietnamese(t *testing.T) {
	p, v := ParsePlanRegion(fixture(t, "tasks_ok.md"), "REQ-142")
	if len(v) != 0 {
		t.Fatalf("violations: %+v", v)
	}
	if len(p.Phases) != 2 || len(p.Tasks) != 0 {
		t.Fatalf("phases=%d tasks=%d", len(p.Phases), len(p.Tasks))
	}
	if p.Phases[0].Title != "Nền tảng aiclient" || !strings.Contains(p.Phases[0].Description, "kiểm thử riêng") {
		t.Fatalf("phase 0: %+v", p.Phases[0])
	}
	t11, t12, t21 := p.Phases[0].Tasks[0], p.Phases[0].Tasks[1], p.Phases[1].Tasks[0]
	if t11.Title != "Dựng client tương thích OpenAI" || t11.TaskType != "feature" || t11.EstimatedHours != 3 ||
		strings.Join(t11.Satisfies, ",") != "AC-AI-01,AC-AI-02" || strings.Join(t11.Labels, ",") != "area:backend" {
		t.Fatalf("T1.1: %+v", t11)
	}
	if t12.EstimatedHours != 2.5 || len(t12.DependsOnIndices) != 1 || t12.DependsOnIndices[0] != 0 {
		t.Fatalf("T1.2 must depend on the first task: %+v", t12)
	}
	if len(t21.DependsOnIndices) != 0 || len(p.Phases[1].DependsOnPhaseIndices) != 1 || p.Phases[1].DependsOnPhaseIndices[0] != 0 {
		t.Fatalf("a dependency on an earlier phase becomes the phase's own dependency: task %+v phase %+v", t21.DependsOnIndices, p.Phases[1].DependsOnPhaseIndices)
	}
	for _, line := range []string{"Serves: FR-SYS-07", "Code: create `backend/shared/pkg/aiclient/client.go::Client`", "Verify: AC-AI-01", "Done when: client gọi được"} {
		if !strings.Contains(t11.Description, line) {
			t.Fatalf("description lost %q: %q", line, t11.Description)
		}
	}
}

func TestParsePlanRegion_NoPhaseFixtureUsesTheTasksContainer(t *testing.T) {
	p, v := ParsePlanRegion(fixture(t, "tasks_no_phase.md"), "REQ-7")
	if len(v) != 0 || len(p.Phases) != 0 || len(p.Tasks) != 2 {
		t.Fatalf("phases=%d tasks=%d v=%+v", len(p.Phases), len(p.Tasks), v)
	}
	if p.Tasks[0].TaskType != "bug" || len(p.Tasks[1].DependsOnIndices) != 1 {
		t.Fatalf("%+v", p.Tasks)
	}
}

func TestParsePlanRegion_ProseCheckboxesAndJSONOutsideTheRegionAreIgnored(t *testing.T) {
	p, v := ParsePlanRegion(fixture(t, "tasks_prose_checkbox.md"), "REQ-9")
	if len(v) != 0 {
		t.Fatalf("violations: %+v", v)
	}
	if len(p.Phases) != 1 || len(p.Phases[0].Tasks) != 1 || p.Phases[0].Tasks[0].Title != "Việc thật duy nhất" {
		t.Fatalf("only the task inside the region counts: %+v", p)
	}
}

func TestParsePlanRegion_Violations(t *testing.T) {
	cases := []struct {
		name string
		md   string
		ref  string
		want string
	}{
		{"dependency cycle", fixture(t, "tasks_cycle.md"), "REQ-1", "TASKSMD_DEPENDENCY_CYCLE"},
		{"unknown line", fixture(t, "tasks_unknown_line.md"), "REQ-1", "TASKSMD_UNKNOWN_LINE"},
		{"two regions", fixture(t, "tasks_two_regions.md"), "REQ-1", "TASKSMD_REGION_DUPLICATE"},
		{"wrong request ref", fixture(t, "tasks_ok.md"), "REQ-999", "TASKSMD_REQUEST_MISMATCH"},
		{"no region", "# nothing here", "REQ-1", "TASKSMD_REGION_MISSING"},
		{"no end marker", "<!-- orca:begin plan request=REQ-1 schema=1 -->\n## TASKS\n- [ ] T1.1 x\n", "REQ-1", "TASKSMD_REGION_MISSING"},
		{"pre-ticked task", planRegionDoc("## TASKS\n- [x] T1.1 x\n"), "REQ-1", "TASKSMD_PREDONE_TASK"},
		{"odd checkbox", planRegionDoc("## TASKS\n- [~] T1.1 x\n"), "REQ-1", "TASKSMD_BAD_CHECKBOX"},
		{"unknown attribute", planRegionDoc("## TASKS\n- [ ] T1.1 x [priority=high]\n"), "REQ-1", "TASKSMD_UNKNOWN_ATTR"},
		{"hours not a number", planRegionDoc("## TASKS\n- [ ] T1.1 x [h=abc]\n"), "REQ-1", "TASKSMD_BAD_ATTR"},
		{"negative hours", planRegionDoc("## TASKS\n- [ ] T1.1 x [h=-2]\n"), "REQ-1", "TASKSMD_BAD_ATTR"},
		{"unknown dependency", planRegionDoc("## TASKS\n- [ ] T1.1 x [depends=T1.9]\n"), "REQ-1", "TASKSMD_DEPENDENCY_UNKNOWN"},
		{"dependency on a later phase", planRegionDoc("## PH-1 a\n- [ ] T1.1 x [depends=T2.1]\n## PH-2 b\n- [ ] T2.1 y\n"), "REQ-1", "TASKSMD_DEPENDENCY_FORWARD"},
		{"self dependency", planRegionDoc("## TASKS\n- [ ] T1.1 x [depends=T1.1]\n"), "REQ-1", "TASKSMD_DEPENDENCY_CYCLE"},
		{"phase out of order", planRegionDoc("## PH-2 b\n- [ ] T2.1 x\n"), "REQ-1", "TASKSMD_PHASE_ORDER"},
		{"phases and TASKS mixed", planRegionDoc("## PH-1 a\n- [ ] T1.1 x\n## TASKS\n"), "REQ-1", "TASKSMD_MIXED_CHILDREN"},
		{"task id repeated", planRegionDoc("## TASKS\n- [ ] T1.1 x\n- [ ] T1.1 y\n"), "REQ-1", "TASKSMD_ID_ORDER"},
		{"task id skips a number", planRegionDoc("## TASKS\n- [ ] T1.1 x\n- [ ] T1.3 y\n"), "REQ-1", "TASKSMD_ID_ORDER"},
		{"task id names the wrong phase", planRegionDoc("## PH-1 a\n- [ ] T2.1 x\n"), "REQ-1", "TASKSMD_ID_ORDER"},
		{"task with no heading", planRegionDoc("- [ ] T1.1 x\n"), "REQ-1", "TASKSMD_TASK_OUTSIDE_CONTAINER"},
		{"title over 200 runes", planRegionDoc("## TASKS\n- [ ] T1.1 " + strings.Repeat("ế", 201) + "\n"), "REQ-1", "TASKSMD_TITLE_TOO_LONG"},
		{"over 1 MiB", "<!-- orca:begin plan request=REQ-1 schema=1 -->\n" + strings.Repeat("a", 1<<20) + "\n<!-- orca:end plan -->", "REQ-1", "TASKSMD_TOO_LARGE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, v := ParsePlanRegion(tc.md, tc.ref)
			if !hasViolation(v, tc.want) {
				t.Fatalf("want %s, got %v", tc.want, violationCodes(v))
			}
			for _, x := range v {
				if x.Line < 1 {
					t.Fatalf("every violation needs a 1-based line: %+v", x)
				}
			}
		})
	}
}

func planRegionDoc(body string) string {
	return "<!-- orca:begin plan request=REQ-1 schema=1 -->\n" + body + "<!-- orca:end plan -->\n"
}

func TestParsePlanRegion_ViolationLinesPointAtTheOffendingLine(t *testing.T) {
	_, v := ParsePlanRegion(fixture(t, "tasks_unknown_line.md"), "REQ-1")
	var lines []int
	for _, x := range v {
		if x.Code == "TASKSMD_UNKNOWN_LINE" {
			lines = append(lines, x.Line)
		}
	}
	if len(lines) != 2 || lines[0] != 5 || lines[1] != 7 {
		t.Fatalf("the stray line is 5 and the tab-indented one is 7, got %v", v)
	}
}

func TestParsePlanRegion_CRLFGivesTheSameProposalAsLF(t *testing.T) {
	crlf, v1 := ParsePlanRegion(fixture(t, "tasks_crlf.md"), "REQ-5")
	lf, v2 := ParsePlanRegion(strings.ReplaceAll(fixture(t, "tasks_crlf.md"), "\r\n", "\n"), "REQ-5")
	if len(v1) != 0 || len(v2) != 0 {
		t.Fatalf("violations %+v %+v", v1, v2)
	}
	if len(crlf.Phases) != 1 || crlf.Phases[0].Title != "CRLF" || crlf.Phases[0].Tasks[0].Title != "Việc" || strings.Contains(crlf.Phases[0].Tasks[0].Description, "\r") {
		t.Fatalf("crlf: %+v", crlf)
	}
	if PlanRegionDigest(fixture(t, "tasks_crlf.md")) != PlanRegionDigest(strings.ReplaceAll(fixture(t, "tasks_crlf.md"), "\r\n", "\n")) {
		t.Fatal("the digest must not depend on the line ending")
	}
	_ = lf
}

func TestParsePlanRegion_NFDInputEqualsNFC(t *testing.T) {
	nfd := "<!-- orca:begin plan request=REQ-1 schema=1 -->\n## TASKS\n- [ ] T1.1 Việc\n<!-- orca:end plan -->\n"
	p, v := ParsePlanRegion(nfd, "REQ-1")
	if len(v) != 0 || p.Tasks[0].Title != "Việc" {
		t.Fatalf("%+v %q", v, p.Tasks[0].Title)
	}
}

func TestParsePlanRegion_ReadsOnlyUpToTheFirstEndMarker(t *testing.T) {
	md := planRegionDoc("## TASKS\n- [ ] T1.1 x\n") + "- [ ] T1.2 after the end\n"
	p, v := ParsePlanRegion(md, "REQ-1")
	if len(v) != 0 || len(p.Tasks) != 1 {
		t.Fatalf("%+v %+v", v, p)
	}
}

func TestParsePlanRegion_NeverPanicsAndStaysUTF8(t *testing.T) {
	for _, md := range []string{"", "\x00", "<!-- orca:begin plan request=", planRegionDoc(""), planRegionDoc("## PH-\n"), planRegionDoc("## PH-99999999999999999999 x\n"), planRegionDoc("- [ ] T.. x\n"), planRegionDoc("## TASKS\n- [ ] T1.1 x [h=1e999]\n")} {
		p, _ := ParsePlanRegion(md, "REQ-1")
		for _, task := range p.Tasks {
			if !utf8.ValidString(task.Title) {
				t.Fatalf("invalid UTF-8 title from %q", md)
			}
		}
	}
}

func FuzzParsePlanRegion(f *testing.F) {
	for _, name := range []string{"tasks_ok.md", "tasks_no_phase.md", "tasks_prose_checkbox.md", "tasks_cycle.md", "tasks_unknown_line.md", "tasks_crlf.md", "tasks_two_regions.md"} {
		b, err := os.ReadFile(filepath.Join("testdata", "openspec", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(b))
	}
	f.Fuzz(func(t *testing.T, md string) {
		if len(md) > 1<<16 {
			t.Skip()
		}
		p, v := ParsePlanRegion(md, "REQ-142")
		if len(v) > 0 {
			return
		}
		// A clean parse must survive render -> parse with the same shape, or the Syncer would corrupt plans.
		out := RenderPlanRegion("REQ-142", p, nil)
		p2, v2 := ParsePlanRegion(out, "REQ-142")
		if len(v2) != 0 {
			t.Fatalf("rendered output does not parse: %v\n%s", violationCodes(v2), out)
		}
		if countTasks(p) != countTasks(p2) || len(p.Phases) != len(p2.Phases) {
			t.Fatalf("shape changed: %d/%d phases, %d/%d tasks", len(p.Phases), len(p2.Phases), countTasks(p), countTasks(p2))
		}
	})
}

func countTasks(p PlanProposal) int {
	n := len(p.Tasks)
	for _, ph := range p.Phases {
		n += len(ph.Tasks)
	}
	return n
}

func TestParsePlanRegion_DependenciesSurviveAnyNumberOfTasksInAPhase(t *testing.T) {
	// Pointers into a growing slice go stale; every task of a 9-task phase must keep its dependency.
	var b strings.Builder
	b.WriteString("## PH-1 Many\n")
	for i := 1; i <= 9; i++ {
		if i == 1 {
			b.WriteString("- [ ] T1.1 first\n")
			continue
		}
		fmt.Fprintf(&b, "- [ ] T1.%d task %d [depends=T1.%d]\n", i, i, i-1)
	}
	p, v := ParsePlanRegion(planRegionDoc(b.String()), "REQ-1")
	if len(v) != 0 {
		t.Fatal(v)
	}
	for i := 1; i < 9; i++ {
		got := p.Phases[0].Tasks[i].DependsOnIndices
		if len(got) != 1 || got[0] != i-1 {
			t.Fatalf("task %d lost its dependency: %v", i+1, got)
		}
	}
}

func TestParsePlanRegion_PhasesAreOrdered_AndMultiPhaseDependenciesAreCollected(t *testing.T) {
	md := planRegionDoc("## PH-1 a\n- [ ] T1.1 x\n## PH-2 b\n- [ ] T2.1 y\n## PH-3 c\n- [ ] T3.1 z [depends=T1.1,T2.1]\n")
	p, v := ParsePlanRegion(md, "REQ-1")
	if len(v) != 0 {
		t.Fatal(v)
	}
	if got := p.Phases[2].DependsOnPhaseIndices; len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("phase 3 starts after phases 1 and 2: %v", got)
	}
	if len(p.Phases[0].DependsOnPhaseIndices) != 0 {
		t.Fatalf("the first phase depends on nothing: %v", p.Phases[0].DependsOnPhaseIndices)
	}
}

func TestParsedPlanPassesPlanValidation(t *testing.T) {
	p, v := ParsePlanRegion(fixture(t, "tasks_ok.md"), "REQ-142")
	if len(v) != 0 {
		t.Fatal(v)
	}
	p.Title = "Plan"
	if err := ValidatePlanProposal(RequestTypeChangeRequest, ShapePlanPhases, DefaultPlanLimits, NormalizePlanProposal(p)); err != nil {
		t.Fatalf("a clean tasks.md must be a valid proposal: %v", err)
	}
	flat, v := ParsePlanRegion(fixture(t, "tasks_no_phase.md"), "REQ-7")
	if len(v) != 0 {
		t.Fatal(v)
	}
	flat.Title = "Plan"
	if err := ValidatePlanProposal(RequestTypeTask, ShapePlanTasks, DefaultPlanLimits, NormalizePlanProposal(flat)); err != nil {
		t.Fatalf("%v", err)
	}
}

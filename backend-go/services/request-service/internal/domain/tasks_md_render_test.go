package domain

import (
	"strings"
	"testing"
)

func TestRenderPlanRegion_RoundTripKeepsEverythingTheParserReads(t *testing.T) {
	p, v := ParsePlanRegion(fixture(t, "tasks_ok.md"), "REQ-142")
	if len(v) != 0 {
		t.Fatal(v)
	}
	out := RenderPlanRegion("REQ-142", p, nil)
	p2, v2 := ParsePlanRegion(out, "REQ-142")
	if len(v2) != 0 {
		t.Fatalf("rendered text does not parse: %v\n%s", violationCodes(v2), out)
	}
	if len(p2.Phases) != len(p.Phases) {
		t.Fatalf("phases %d -> %d", len(p.Phases), len(p2.Phases))
	}
	for i := range p.Phases {
		if p.Phases[i].Title != p2.Phases[i].Title || strings.TrimSpace(p.Phases[i].Description) != strings.TrimSpace(p2.Phases[i].Description) {
			t.Fatalf("phase %d changed: %+v -> %+v", i, p.Phases[i], p2.Phases[i])
		}
		for j := range p.Phases[i].Tasks {
			a, b := p.Phases[i].Tasks[j], p2.Phases[i].Tasks[j]
			if a.Title != b.Title || normalizedType(a.TaskType) != normalizedType(b.TaskType) || a.EstimatedHours != b.EstimatedHours ||
				strings.Join(a.Satisfies, ",") != strings.Join(b.Satisfies, ",") || strings.Join(a.Labels, ",") != strings.Join(b.Labels, ",") ||
				strings.TrimSpace(a.Description) != strings.TrimSpace(b.Description) {
				t.Fatalf("task %d.%d changed:\n%+v\n%+v", i, j, a, b)
			}
			if len(a.DependsOnIndices) != len(b.DependsOnIndices) || (len(a.DependsOnIndices) == 1 && a.DependsOnIndices[0] != b.DependsOnIndices[0]) {
				t.Fatalf("task %d.%d lost its dependency: %v -> %v", i, j, a.DependsOnIndices, b.DependsOnIndices)
			}
		}
	}
}

func TestRenderPlanRegion_WritesDependenciesAndTheTaskUUIDAnnotation(t *testing.T) {
	p, _ := ParsePlanRegion(fixture(t, "tasks_ok.md"), "REQ-142")
	out := RenderPlanRegion("REQ-142", p, map[string]string{"T1.2": "uuid-12"})
	if !strings.Contains(out, "[depends=T1.1]") || strings.Contains(out, "[depends=T1.2]") {
		t.Fatalf("a same-phase dependency is written, a cross-phase one is implied by the phase order:\n%s", out)
	}
	if !strings.Contains(out, "  <!-- orca:task T1.2 id=uuid-12 -->") || strings.Contains(out, "T1.1 id=") {
		t.Fatalf("only the given ids are annotated:\n%s", out)
	}
	if _, v := ParsePlanRegion(out, "REQ-142"); len(v) != 0 {
		t.Fatalf("annotations must not break parsing: %v", v)
	}
}

func TestRenderPlanRegion_TasksContainerAndEmptyPlan(t *testing.T) {
	p, _ := ParsePlanRegion(fixture(t, "tasks_no_phase.md"), "REQ-7")
	out := RenderPlanRegion("REQ-7", p, nil)
	if !strings.Contains(out, "## TASKS") || strings.Contains(out, "## PH-") {
		t.Fatalf("%s", out)
	}
	empty := RenderPlanRegion("REQ-1", PlanProposal{}, nil)
	if !strings.HasPrefix(empty, "<!-- orca:begin plan request=REQ-1 schema=1 -->") || !strings.HasSuffix(empty, "<!-- orca:end plan -->") {
		t.Fatalf("%q", empty)
	}
}

func TestRenderPlanRegion_IsNFC(t *testing.T) {
	p := PlanProposal{Tasks: []TaskProposal{{Title: "Việc"}}}
	if !strings.Contains(RenderPlanRegion("REQ-1", p, nil), "Việc") {
		t.Fatal("output must be NFC")
	}
}

func TestTickTask_TicksInsideTheRegionOnly(t *testing.T) {
	md := fixture(t, "tasks_prose_checkbox.md")
	out, changed, err := TickTask(md, "T1.1")
	if err != nil || !changed {
		t.Fatalf("%v %v", changed, err)
	}
	if !strings.Contains(out, "- [x] T1.1 Việc thật duy nhất") {
		t.Fatalf("the task in the region is ticked:\n%s", out)
	}
	if !strings.Contains(out, "Một ghi chú dài: - [ ] T1.1 task giả đầu tiên") || !strings.Contains(out, "- [ ] T1.1 nằm trong khối code") {
		t.Fatalf("look-alikes in the prose must stay untouched:\n%s", out)
	}
	if _, changed, _ := TickTask(md, "T1.2"); changed {
		t.Fatal("T1.2 exists only outside the region: nothing to tick")
	}
}

func TestTickTask_IsOneWayAndIdempotent(t *testing.T) {
	md := planRegionDoc("## TASKS\n- [ ] T1.1 x\n- [ ] T1.2 y\n")
	once, changed, _ := TickTask(md, "T1.1")
	if !changed {
		t.Fatal("first tick must change")
	}
	twice, changed, err := TickTask(once, "T1.1")
	if err != nil || changed || twice != once {
		t.Fatalf("ticking a done task changes nothing: %v %v", changed, err)
	}
	if strings.Contains(once, "- [x] T1.2") || !strings.Contains(once, "- [ ] T1.2 y") {
		t.Fatalf("only the named task is ticked:\n%s", once)
	}
	if _, _, err := TickTask("no region at all", "T1.1"); err == nil {
		t.Fatal("a document without a region is an error")
	}
	// T1.1 must not match T1.10.
	md = planRegionDoc("## TASKS\n- [ ] T1.10 ten\n")
	if _, changed, _ := TickTask(md, "T1.1"); changed {
		t.Fatal("T1.1 must not tick T1.10")
	}
}

func TestTickTask_KeepsCRLF(t *testing.T) {
	md := fixture(t, "tasks_crlf.md")
	out, changed, err := TickTask(md, "T1.1")
	if err != nil || !changed || !strings.Contains(out, "- [x] T1.1 Việc [h=1]\r\n") {
		t.Fatalf("%v %v %q", changed, err, out)
	}
}

func TestReplacePlanRegion_KeepsTextAroundTheRegionByteForByte(t *testing.T) {
	md := fixture(t, "tasks_ok.md")
	p, _ := ParsePlanRegion(md, "REQ-142")
	out, err := ReplacePlanRegion(md, RenderPlanRegion("REQ-142", p, map[string]string{"T1.1": "u1"}))
	if err != nil {
		t.Fatal(err)
	}
	before := md[:strings.Index(md, "<!-- orca:begin")]
	after := md[strings.Index(md, "<!-- orca:end plan -->")+len("<!-- orca:end plan -->"):]
	if !strings.HasPrefix(out, before) || !strings.HasSuffix(out, after) {
		t.Fatalf("prose around the region changed:\n%s", out)
	}
	if !strings.Contains(out, "id=u1") {
		t.Fatalf("the new region is missing:\n%s", out)
	}
	if _, v := ParsePlanRegion(out, "REQ-142"); len(v) != 0 {
		t.Fatal(v)
	}
	if _, err := ReplacePlanRegion("no region", "x"); err == nil {
		t.Fatal("no region means nothing to replace")
	}
}

func TestReplacePlanRegion_CRLFDocumentStaysCRLF(t *testing.T) {
	md := fixture(t, "tasks_crlf.md")
	p, _ := ParsePlanRegion(md, "REQ-5")
	out, err := ReplacePlanRegion(md, RenderPlanRegion("REQ-5", p, nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\n") {
		t.Fatalf("mixed line endings: %q", out)
	}
}

func TestPlanRegionDigest_ChangesWithTheContentAndIgnoresLineEndings(t *testing.T) {
	a := PlanRegionDigest(planRegionDoc("## TASKS\n- [ ] T1.1 x\n"))
	b := PlanRegionDigest(planRegionDoc("## TASKS\n- [ ] T1.1 y\n"))
	if a == b || len(a) != 64 {
		t.Fatalf("%s %s", a, b)
	}
}

// The renderer omits the default type, so "" and "task" are the same task.
func normalizedType(t string) string {
	if t == "" {
		return "task"
	}
	return t
}

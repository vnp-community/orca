package domain

import (
	"strings"
	"testing"
)

func TestSpecLabel_RoundTripsAndIgnoresOtherLabels(t *testing.T) {
	if got := SpecLabel("T12.3"); got != "spec:t12.3" {
		t.Fatal(got)
	}
	if ref, ok := SpecRefFromLabels([]string{"test:regression", "SPEC:T2.10 "}); !ok || ref != "T2.10" {
		t.Fatalf("%q %v", ref, ok)
	}
	for _, bad := range [][]string{nil, {"spec:t1"}, {"spec:t1.x"}, {"spec:t1.2.3"}, {"myspec:t1.2"}, {"test:regression"}} {
		if ref, ok := SpecRefFromLabels(bad); ok {
			t.Fatalf("%v read as %q", bad, ref)
		}
	}
}

func TestTickTasks_TicksOpenRefsOnceAndSkipsTheRest(t *testing.T) {
	md := "<!-- orca:begin plan request=REQ-1 schema=1 -->\n- [ ] T1.1 a\n- [x] T1.2 b\n- [ ] T1.3 c\n<!-- orca:end plan -->\n"
	out, ticked, err := TickTasks(md, []string{"T1.1", "T1.2", "T9.9", "T1.1"})
	if err != nil || len(ticked) != 1 || ticked[0] != "T1.1" {
		t.Fatalf("%v %v", ticked, err)
	}
	if !strings.Contains(out, "- [x] T1.1 a") || !strings.Contains(out, "- [ ] T1.3 c") {
		t.Fatal(out)
	}
	if _, _, err := TickTasks("no region", []string{"T1.1"}); err == nil {
		t.Fatal("a document without a region must fail")
	}
}

func TestArchiveScopeViolations(t *testing.T) {
	ok := []string{"openspec/changes/archive/2026-10-14-req-1-x/tasks.md", "openspec/changes/req-1-x/tasks.md", "openspec/specs/ai/spec.md"}
	if bad := ArchiveScopeViolations(ok); len(bad) != 0 {
		t.Fatal(bad)
	}
	in := []string{"src/a.go", "openspec/config.yaml", "openspec/changes/../src/a.go", "openspecx/specs/a", "openspec\\specs\\a.md", "../openspec/specs/a"}
	if bad := ArchiveScopeViolations(in); len(bad) != 5 {
		t.Fatalf("want 5 violations, got %v", bad)
	}
}

func TestSyncBackoff_GrowsAndStaysCapped(t *testing.T) {
	prev := SyncBackoff(1)
	for i := 2; i <= MaxSyncAttempts+3; i++ {
		d := SyncBackoff(i)
		if d < prev {
			t.Fatalf("attempt %d: %v shrank from %v", i, d, prev)
		}
		prev = d
	}
	if SyncBackoff(0) != SyncBackoff(1) {
		t.Fatal("a zero attempt reads as the first")
	}
}

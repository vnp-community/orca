package domain

import (
	"os"
	"strings"
	"testing"
)

func solutionWithReferences(t *testing.T, mutate func(*SolutionOptions)) SolutionOptions {
	t.Helper()
	raw, err := os.ReadFile("testdata/solution_options/valid_with_design_references.json")
	if err != nil {
		t.Fatal(err)
	}
	o, err := ParseSolutionOptions(raw)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&o)
	return o
}

func TestDesignReferences_ARecordedReadValidatesAndSurvivesTheRoundTrip(t *testing.T) {
	o := solutionWithReferences(t, func(*SolutionOptions) {})
	if err := o.Validate(2); err != nil {
		t.Fatal(err)
	}
	raw, err := o.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseSolutionOptions(raw)
	if err != nil || again.DesignReferences == nil || !again.DesignReferences.Available || again.DesignReferences.Snapshot != "kg-2026-10-14" || len(again.DesignReferences.Conflicts) != 1 {
		t.Fatalf("round trip lost design_references: %+v %v", again.DesignReferences, err)
	}
}

func TestDesignReferences_AbsentIsFineAndChangesNothingElse(t *testing.T) {
	o := solutionWithReferences(t, func(o *SolutionOptions) { o.DesignReferences = nil })
	if err := o.Validate(2); err != nil {
		t.Fatal(err)
	}
	raw, _ := o.Marshal()
	if strings.Contains(string(raw), "design_references") {
		t.Fatalf("a document without references grew one: %s", raw)
	}
}

func TestDesignReferences_RejectsWhatCouldFloodOrMislead(t *testing.T) {
	long := strings.Repeat("x", 1501)
	cases := map[string]func(*SolutionOptions){
		"snapshot too long":   func(o *SolutionOptions) { o.DesignReferences.Snapshot = strings.Repeat("s", 121) },
		"too many reads":      func(o *SolutionOptions) { o.DesignReferences.Read = make([]string, 81) },
		"empty read entry":    func(o *SolutionOptions) { o.DesignReferences.Read = []string{"MOD-a", ""} },
		"read entry too long": func(o *SolutionOptions) { o.DesignReferences.Read = []string{strings.Repeat("r", 201)} },
		"gap too long":        func(o *SolutionOptions) { o.DesignReferences.Gaps = []string{long} },
		"too many conflicts":  func(o *SolutionOptions) { o.DesignReferences.Conflicts = make([]string, 31) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			// Blank entries from make() are themselves invalid, which is the point for the two list-size cases.
			err := solutionWithReferences(t, mutate).Validate(2)
			if err == nil || !strings.Contains(err.Error(), "design_references") {
				t.Fatalf("err = %v, want a design_references error", err)
			}
		})
	}
}

func TestDesignReferences_WrongTypesAreRefusedAtParse(t *testing.T) {
	raw := `{"schema_version":1,"options":[],"recommendation":{"option_id":"","reason":""},"design_references":{"available":"yes"}}`
	if _, err := ParseSolutionOptions([]byte(raw)); err == nil {
		t.Fatal("a string for available was accepted")
	}
}

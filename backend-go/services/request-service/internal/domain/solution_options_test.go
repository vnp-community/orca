package domain

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func loadValidOptions(t *testing.T) SolutionOptions {
	t.Helper()
	raw, err := os.ReadFile("testdata/solution_options/valid_two_options.json")
	if err != nil {
		t.Fatal(err)
	}
	o, err := ParseSolutionOptions(raw)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestSolutionOptions_Validate_GoldenAccepted(t *testing.T) {
	if err := loadValidOptions(t).Validate(2); err != nil {
		t.Fatalf("golden should be valid: %v", err)
	}
}

func TestSolutionOptions_Validate_Table(t *testing.T) {
	cases := []struct {
		name   string
		min    int
		mutate func(o *SolutionOptions)
		want   string // substring of the error; empty means valid
	}{
		{"valid", 2, func(o *SolutionOptions) {}, ""},
		{"one option when min is two", 2, func(o *SolutionOptions) { o.Options = o.Options[:1] }, "number of options"},
		{"one option when min is one", 1, func(o *SolutionOptions) {
			o.Options = o.Options[:1]
		}, ""},
		{"five options", 2, func(o *SolutionOptions) {
			for i := 3; i <= 5; i++ {
				c := o.Options[1]
				c.ID = "opt-" + string(rune('0'+i))
				o.Options = append(o.Options, c)
			}
		}, "number of options"},
		{"zero recommended", 2, func(o *SolutionOptions) { o.Options[0].Recommended = false }, "exactly one option"},
		{"two recommended", 2, func(o *SolutionOptions) { o.Options[1].Recommended = true }, "exactly one option"},
		{"duplicate id", 2, func(o *SolutionOptions) { o.Options[1].ID = "opt-1" }, "duplicate option id"},
		{"bad id shape", 2, func(o *SolutionOptions) { o.Options[1].ID = "option-2" }, "opt-N"},
		{"unknown recommendation", 2, func(o *SolutionOptions) { o.Recommendation.OptionID = "opt-9" }, "recommendation.option_id"},
		{"recommendation points at non-recommended", 2, func(o *SolutionOptions) { o.Recommendation.OptionID = "opt-2" }, "recommendation.option_id"},
		{"missing effort size", 2, func(o *SolutionOptions) { o.Options[0].Effort.Size = "" }, "effort.size"},
		{"bad effort size", 2, func(o *SolutionOptions) { o.Options[0].Effort.Size = "XL" }, "effort.size"},
		{"negative hours", 2, func(o *SolutionOptions) { h := -1.0; o.Options[0].Effort.HoursEstimate = &h }, "hours_estimate"},
		{"zero hours ok", 2, func(o *SolutionOptions) { h := 0.0; o.Options[0].Effort.HoursEstimate = &h }, ""},
		{"empty title", 2, func(o *SolutionOptions) { o.Options[0].Title = "" }, "title"},
		{"title 121 runes", 2, func(o *SolutionOptions) { o.Options[0].Title = strings.Repeat("ạ", 121) }, "title"},
		{"title 120 runes of multibyte", 2, func(o *SolutionOptions) { o.Options[0].Title = strings.Repeat("ạ", 120) }, ""},
		{"summary 601", 2, func(o *SolutionOptions) { o.Options[0].Summary = strings.Repeat("a", 601) }, "summary"},
		{"approach 4001", 2, func(o *SolutionOptions) { o.Options[0].Approach = strings.Repeat("a", 4001) }, "approach"},
		{"approach empty", 2, func(o *SolutionOptions) { o.Options[0].Approach = "" }, "approach"},
		{"bad schema version", 2, func(o *SolutionOptions) { o.SchemaVersion = 2 }, "schema_version"},
		{"bad risk severity", 2, func(o *SolutionOptions) { o.Options[0].Risks[0].Severity = "extreme" }, "severity"},
		{"bad area kind", 2, func(o *SolutionOptions) { o.Options[0].AffectedAreas[0].Kind = "database" }, "kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := loadValidOptions(t)
			tc.mutate(&o)
			err := o.Validate(tc.min)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSolutionOptions_Validate_TotalSizeCap(t *testing.T) {
	o := loadValidOptions(t)
	o.Options = o.Options[:1]
	for i := 0; i < 400; i++ {
		o.Assumptions = append(o.Assumptions, Assumption{Text: strings.Repeat("x", 200)})
	}
	err := o.Validate(1)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("want size error, got %v", err)
	}
}

func TestParseSolutionOptions_RejectsDuplicateKeysAndOversize(t *testing.T) {
	if _, err := ParseSolutionOptions([]byte(`{"schema_version":1,"schema_version":2}`)); err == nil {
		t.Fatal("duplicate key must be rejected")
	}
	if _, err := ParseSolutionOptions([]byte(`not json`)); err == nil {
		t.Fatal("garbage must be rejected")
	}
	big := `{"schema_version":1,"assumptions":["` + strings.Repeat("a", MaxOptionsBytes) + `"]}`
	if _, err := ParseSolutionOptions([]byte(big)); err == nil {
		t.Fatal("oversize must be rejected")
	}
}

func TestSolutionOptions_IndexOfAndMinOptions(t *testing.T) {
	o := loadValidOptions(t)
	if i, ok := o.IndexOf("opt-2"); !ok || i != 1 {
		t.Fatalf("IndexOf(opt-2) = %d %v", i, ok)
	}
	if _, ok := o.IndexOf("opt-7"); ok {
		t.Fatal("unknown id must not resolve")
	}
	if MinOptionsFor(RequestTypeChangeRequest) != 2 || MinOptionsFor(RequestTypeRefactor) != 1 {
		t.Fatal("min options per type changed")
	}
}

func TestSolutionOptions_MarshalDropsUnknownFields(t *testing.T) {
	raw := `{"schema_version":1,"evil":"<script>","options":[{"id":"opt-1","title":"t","summary":"s","approach":"a","effort":{"size":"S"},"recommended":true,"junk":1}],"recommendation":{"option_id":"opt-1","reason":"r"}}`
	o, err := ParseSolutionOptions([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	out, err := o.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "evil") || strings.Contains(string(out), "junk") {
		t.Fatalf("unknown fields kept: %s", out)
	}
}

func TestSolutionOptions_StructuredItemsAndLegacyStrings(t *testing.T) {
	o := loadValidOptions(t)
	raw := []byte(`{"assumptions":["plain",{"id":"A-1","text":"db is postgres","needs_confirmation":true}],` +
		`"open_questions":["plain q",{"id":"Q-1","text":"which region","blocking":true}],` +
		`"requirement_coverage":[{"ac_id":"AC-1","option_ids":["opt-1"],"status":"covered","note":""}]}`)
	if err := json.Unmarshal(raw, &o); err != nil {
		t.Fatal(err)
	}
	if len(o.Assumptions) != 2 || o.Assumptions[0].ID != "" || !o.Assumptions[1].NeedsConfirmation || !o.OpenQuestions[1].Blocking {
		t.Fatalf("decoded: %+v %+v", o.Assumptions, o.OpenQuestions)
	}
	if err := o.Validate(1); err != nil {
		t.Fatalf("valid structured document rejected: %v", err)
	}
	out, _ := json.Marshal(o)
	if !strings.Contains(string(out), `"assumptions":["plain",{"id":"A-1"`) {
		t.Fatalf("legacy string item must stay a string: %s", out)
	}
	for name, mutate := range map[string]func(*SolutionOptions){
		"bad assumption id": func(x *SolutionOptions) { x.Assumptions[1].ID = "X-1" },
		"dup question id":   func(x *SolutionOptions) { x.OpenQuestions = append(x.OpenQuestions, x.OpenQuestions[1]) },
		"unknown option":    func(x *SolutionOptions) { x.RequirementCoverage[0].OptionIDs = []string{"opt-9"} },
		"bad status":        func(x *SolutionOptions) { x.RequirementCoverage[0].Status = "done" },
		"bad ac id":         func(x *SolutionOptions) { x.RequirementCoverage[0].ACID = "AC1" },
	} {
		c := o
		c.Assumptions = append([]Assumption(nil), o.Assumptions...)
		c.OpenQuestions = append([]OpenQuestion(nil), o.OpenQuestions...)
		c.RequirementCoverage = append([]CoverageEntry(nil), o.RequirementCoverage...)
		mutate(&c)
		if err := c.Validate(1); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

package domain

import (
	"os"
	"strings"
	"testing"
)

func readDoc(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/analysis_documents/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAnalysisDocuments_Golden(t *testing.T) {
	d, err := ParseDiagnosisDocument(readDoc(t, "diagnosis_valid.json"))
	if err != nil || d.Validate() != nil {
		t.Fatalf("diagnosis: %v %v", err, d.Validate())
	}
	f, err := ParseFindingsDocument(readDoc(t, "findings_valid.json"))
	if err != nil || f.Validate() != nil {
		t.Fatalf("findings: %v %v", err, f.Validate())
	}
	a, err := ParseAnswerDocument(readDoc(t, "answer_valid.json"))
	if err != nil || a.Validate() != nil {
		t.Fatalf("answer: %v %v", err, a.Validate())
	}
	for _, name := range []string{"diagnosis_invalid_no_statement.json", "findings_invalid_no_findings.json", "answer_invalid_empty.json"} {
		var verr error
		switch {
		case strings.HasPrefix(name, "diagnosis"):
			x, _ := ParseDiagnosisDocument(readDoc(t, name))
			verr = x.Validate()
		case strings.HasPrefix(name, "findings"):
			x, _ := ParseFindingsDocument(readDoc(t, name))
			verr = x.Validate()
		default:
			x, _ := ParseAnswerDocument(readDoc(t, name))
			verr = x.Validate()
		}
		if verr == nil {
			t.Errorf("%s must be invalid", name)
		}
	}
}

func validDiagnosis(t *testing.T) DiagnosisDocument {
	d, err := ParseDiagnosisDocument(readDoc(t, "diagnosis_valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDiagnosisDocument_Validate_Table(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(d *DiagnosisDocument)
		want   string
	}{
		{"valid", func(d *DiagnosisDocument) {}, ""},
		{"wrong kind", func(d *DiagnosisDocument) { d.Kind = "answer" }, "kind"},
		{"schema version", func(d *DiagnosisDocument) { d.SchemaVersion = 2 }, "schema_version"},
		{"no statement", func(d *DiagnosisDocument) { d.RootCause.Statement = "" }, "statement"},
		{"confidence 1.2", func(d *DiagnosisDocument) { d.RootCause.Confidence = 1.2 }, "confidence"},
		{"confidence negative", func(d *DiagnosisDocument) { d.RootCause.Confidence = -0.1 }, "confidence"},
		{"21 evidence", func(d *DiagnosisDocument) {
			d.RootCause.Evidence = make([]SupportingRef, 21)
		}, "evidence"},
		{"excerpt 601 runes", func(d *DiagnosisDocument) {
			d.RootCause.Evidence[0].Excerpt = strings.Repeat("ạ", 601)
		}, "excerpt"},
		{"bad evidence type", func(d *DiagnosisDocument) { d.RootCause.Evidence[0].Type = "rumor" }, "evidence"},
		{"bad severity", func(d *DiagnosisDocument) { d.Impact.Severity = "catastrophic" }, "severity"},
		{"duplicate fix id", func(d *DiagnosisDocument) {
			d.FixDirections = append(d.FixDirections, d.FixDirections[0])
		}, "fix_directions"},
		{"bad size", func(d *DiagnosisDocument) { d.SuggestedSize = "XL" }, "suggested_size"},
		{"bad reproducible", func(d *DiagnosisDocument) { d.Reproduction.Reproducible = "maybe" }, "reproducible"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validDiagnosis(t)
			tc.mutate(&d)
			err := d.Validate()
			if tc.want == "" && err != nil {
				t.Fatalf("want valid: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestFindingsDocument_Validate_Table(t *testing.T) {
	load := func() FindingsDocument {
		f, err := ParseFindingsDocument(readDoc(t, "findings_valid.json"))
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	cases := []struct {
		name   string
		mutate func(f *FindingsDocument)
		want   string
	}{
		{"valid", func(f *FindingsDocument) {}, ""},
		{"no findings", func(f *FindingsDocument) { f.Findings = nil }, "at least one"},
		{"no evidence and not in unknowns", func(f *FindingsDocument) { f.Findings[0].Evidence = nil }, "no evidence"},
		{"no evidence but listed in unknowns by id", func(f *FindingsDocument) {
			f.Findings[0].Evidence = nil
			f.Unknowns = []string{"f-1 chưa kiểm chứng được"}
		}, ""},
		{"confidence out of range", func(f *FindingsDocument) { f.Findings[0].Confidence = 2 }, "confidence"},
		{"bad follow-up type", func(f *FindingsDocument) { f.FollowUps[0].SuggestedType = "epic" }, "suggested_type"},
		{"empty statement", func(f *FindingsDocument) { f.Findings[0].Statement = "" }, "statement"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := load()
			tc.mutate(&f)
			err := f.Validate()
			if tc.want == "" && err != nil {
				t.Fatalf("want valid: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestAnswerDocument_Validate_CountsRunes(t *testing.T) {
	mk := func(md string) AnswerDocument {
		a, err := ParseAnswerDocument(readDoc(t, "answer_valid.json"))
		if err != nil {
			t.Fatal(err)
		}
		a.AnswerMarkdown = md
		return a
	}
	if err := mk(strings.Repeat("ạ", 8000)).Validate(); err != nil {
		t.Fatalf("8000 multibyte runes must pass: %v", err)
	}
	if err := mk(strings.Repeat("ạ", 8001)).Validate(); err == nil {
		t.Fatal("8001 runes must fail")
	}
	if err := mk("").Validate(); err == nil {
		t.Fatal("empty must fail")
	}
	a := mk("ok")
	a.Confidence = 1.5
	if a.Validate() == nil {
		t.Fatal("confidence 1.5 must fail")
	}
	a = mk("ok")
	a.SuggestedFollowUp.Type = "story"
	if a.Validate() == nil {
		t.Fatal("bad follow-up type must fail")
	}
}

func TestAnalysisDocument_RejectsDuplicateKeysAndOversize(t *testing.T) {
	if _, err := ParseAnswerDocument([]byte(`{"kind":"answer","kind":"answer"}`)); err == nil {
		t.Fatal("duplicate key must fail")
	}
	big := `{"schema_version":1,"kind":"answer","answer_markdown":"` + strings.Repeat("a", MaxOptionsBytes) + `"}`
	if _, err := ParseAnswerDocument([]byte(big)); err == nil {
		t.Fatal("oversize must fail")
	}
}

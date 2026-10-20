package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func acs(ids ...string) []AcceptanceCriterion {
	out := make([]AcceptanceCriterion, 0, len(ids))
	for _, id := range ids {
		out = append(out, AcceptanceCriterion{ID: id, Text: "text of " + id, Status: ACStatusActive})
	}
	return out
}

func TestClassifyRequirements_Table(t *testing.T) {
	tasks := map[string]CoveringTask{
		"t-done":   {Status: TaskStatusDone},
		"t-done2":  {Status: TaskStatusDone},
		"t-open":   {Status: TaskStatusOpen},
		"t-failed": {Status: TaskStatusDone, Failed: true},
	}
	cases := []struct {
		name  string
		in    RequirementInputs
		want  RequirementStatus
		needs bool
	}{
		{"design phase is planned even with coverage", RequirementInputs{Phase: ReviewPhaseDesign, Criteria: acs("AC-1"), DataOK: true,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t-done"}}, Tasks: tasks}, ReqPlanned, false},
		{"all covering tasks done", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: true,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t-done"}, {ACID: "AC-1", TaskID: "t-done2"}}, Tasks: tasks}, ReqDelivered, false},
		{"duplicate rows for one task count once", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: true,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t-done", CheckID: "a"}, {ACID: "AC-1", TaskID: "t-done", CheckID: "b"}}, Tasks: tasks}, ReqDelivered, false},
		{"one of two done is partial", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: true,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t-done"}, {ACID: "AC-1", TaskID: "t-open"}}, Tasks: tasks}, ReqPartial, false},
		{"done but failed is partial, never delivered", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: true,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t-failed"}}, Tasks: tasks}, ReqPartial, false},
		{"none done is not covered", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: true,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t-open"}}, Tasks: tasks}, ReqNotCovered, true},
		{"no coverage row is not covered", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: true, Tasks: tasks}, ReqNotCovered, true},
		{"unreadable data is unknown, never delivered", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: false,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t-done"}}, Tasks: tasks}, ReqUnknown, false},
		{"task missing from the map is not delivered", RequirementInputs{Phase: ReviewPhaseDelivered, Criteria: acs("AC-1"), DataOK: true,
			Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "ghost"}}, Tasks: tasks}, ReqNotCovered, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyRequirements(tc.in)
			if len(got) != 1 || got[0].Status != tc.want {
				t.Fatalf("status = %+v, want %s", got, tc.want)
			}
			if got[0].Item.NeedsDecision != tc.needs {
				t.Fatalf("needsDecision = %v, want %v", got[0].Item.NeedsDecision, tc.needs)
			}
			if got[0].Item.Origin != OriginFact {
				t.Fatalf("requirement items are facts")
			}
		})
	}
}

func TestClassifyRequirements_SkipsRetiredAndSortsNumerically(t *testing.T) {
	in := RequirementInputs{Phase: ReviewPhaseDesign, DataOK: true, Criteria: []AcceptanceCriterion{
		{ID: "AC-10", Text: "ten", Status: ACStatusActive}, {ID: "AC-2", Text: "two", Status: ACStatusActive}, {ID: "AC-3", Text: "gone", Status: ACStatusRetired},
	}}
	got := ClassifyRequirements(in)
	if len(got) != 2 || !strings.HasPrefix(got[0].Item.Title, "AC-2") || !strings.HasPrefix(got[1].Item.Title, "AC-10") {
		t.Fatalf("got %+v", got)
	}
	v := CountRequirements(got)
	if v.Total != 2 || v.Planned != 2 || v.Delivered != 0 {
		t.Fatalf("counts %+v", v)
	}
}

func TestClassifyChangedPaths_AndReleaseFlags(t *testing.T) {
	pc := ClassifyChangedPaths([]ChangedPath{
		{Path: "backend-go/services/request-service/migrations/postgres/0105_x.up.sql", Added: 10},
		{Path: "deploy/dev/.env.example", Added: 2},
		{Path: "frontend/src/a.ts", Added: 5, Removed: 1},
		{Path: "../etc/passwd"}, {Path: "/abs/path"}, {Path: ""},
	})
	if !pc.Migration || !pc.Config || pc.Files != 3 || pc.Added != 17 || pc.Removed != 1 {
		t.Fatalf("classes %+v", pc)
	}
	yes := true
	if got := ReleaseFlags(pc, &yes); !reflect.DeepEqual(got, []string{FlagMigration, FlagConfig, FlagBreaking}) {
		t.Fatalf("flags %v", got)
	}
	if got := ReleaseFlags(pc, nil); !reflect.DeepEqual(got, []string{FlagMigration, FlagConfig}) {
		t.Fatalf("unmeasured breaking must not appear: %v", got)
	}
	if ClassifyChangedPaths(nil).Migration {
		t.Fatalf("no paths, no migration")
	}
}

func baseFacts(phase ReviewPhase) Facts {
	f := Facts{Phase: phase, Subject: ReviewSubject{BaseRef: "main"}, Requests: RequirementInputs{Phase: phase, Criteria: acs("AC-1", "AC-2"), DataOK: true,
		Coverage: []CoverageRow{{ACID: "AC-1", TaskID: "t1"}}, Tasks: map[string]CoveringTask{"t1": {Status: TaskStatusDone}}}}
	if phase == ReviewPhaseDelivered {
		f.Subject.Heads = []ReviewHead{{Branch: "b2", Commit: "c2"}, {Branch: "b1", Commit: "c1"}}
		f.ChangedRead = true
		f.Changed = []ChangedPath{{Path: "backend-go/x/migrations/0105.sql", Added: 3}, {Path: "a/b.go", Added: 1}}
	}
	return f
}

func TestBuildOutcomeReview_DeterministicAndOrderIndependent(t *testing.T) {
	a := baseFacts(ReviewPhaseDelivered)
	b := baseFacts(ReviewPhaseDelivered)
	b.Subject.Heads = []ReviewHead{{Branch: "b1", Commit: "c1"}, {Branch: "b2", Commit: "c2"}}
	b.Changed = []ChangedPath{{Path: "a/b.go", Added: 1}, {Path: "backend-go/x/migrations/0105.sql", Added: 3}}
	b.Requests.Criteria = []AcceptanceCriterion{b.Requests.Criteria[1], b.Requests.Criteria[0]}
	ra, err := BuildOutcomeReview("req-1", ReviewBuildInput{Facts: a, KG: KGInfo{Available: true, Snapshot: "s1"}})
	if err != nil {
		t.Fatal(err)
	}
	rb, err := BuildOutcomeReview("req-1", ReviewBuildInput{Facts: b, KG: KGInfo{Available: true, Snapshot: "s1"}})
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(ra.Items)
	jb, _ := json.Marshal(rb.Items)
	if string(ja) != string(jb) {
		t.Fatalf("items depend on input order")
	}
	if ra.InputsDigest == "" || ra.InputsDigest != rb.InputsDigest {
		t.Fatalf("digest must not depend on order: %q vs %q", ra.InputsDigest, rb.InputsDigest)
	}
	b.Requests.Coverage = nil
	rc, _ := BuildOutcomeReview("req-1", ReviewBuildInput{Facts: b, KG: KGInfo{Available: true}})
	if rc.InputsDigest == ra.InputsDigest {
		t.Fatalf("a different input must change the digest")
	}
	if !reflect.DeepEqual(ra.Verdict.Release.Flags, []string{FlagMigration}) {
		t.Fatalf("flags %v", ra.Verdict.Release.Flags)
	}
	if ra.Verdict.Release.Risk != "unknown" || ra.Verdict.Quality.Gate != "absent" {
		t.Fatalf("no assessment and no gate must read unknown and absent: %+v", ra.Verdict)
	}
}

func TestBuildOutcomeReview_DesignPhaseHasNoDiffFacts(t *testing.T) {
	f := baseFacts(ReviewPhaseDesign)
	f.PlanTargetsSet = true
	f.PlanTargets = []string{"backend-go/a.go", "backend-go/b.go", "frontend/c.ts"}
	r, err := BuildOutcomeReview("req-1", ReviewBuildInput{Facts: f})
	if err != nil {
		t.Fatal(err)
	}
	if r.Phase != ReviewPhaseDesign || len(r.Verdict.Release.Flags) != 0 {
		t.Fatalf("design review carries no release flags: %+v", r.Verdict.Release)
	}
	if r.Verdict.Requirements.Planned != 2 || r.Verdict.Requirements.Delivered != 0 {
		t.Fatalf("requirements %+v", r.Verdict.Requirements)
	}
	var plan int
	for _, it := range r.Items {
		if it.Section == SectionConfig {
			plan++
		}
		if it.NeedsDecision {
			t.Fatalf("a design review asks for no decision: %+v", it)
		}
	}
	if plan != 2 {
		t.Fatalf("one plan-target row per area, got %d", plan)
	}
	if !contains(r.Warnings, "kg_unavailable") {
		t.Fatalf("kg absent must warn: %v", r.Warnings)
	}
}

func TestBuildOutcomeReview_AgentItemsNeedEvidenceAndAreRedacted(t *testing.T) {
	f := baseFacts(ReviewPhaseDelivered)
	items := []ReviewItem{
		{Section: SectionArchitecture, Change: ChangeAdded, Title: "token sk-secret-123 leaked", Severity: SeverityDeviation,
			Evidence: []ReviewEvidence{{Kind: EvidenceKG, Ref: "ADR-07"}}},
		{Section: SectionFlow, Change: ChangeAdded, Title: "no proof", Severity: SeverityDeviation},
	}
	r, err := BuildOutcomeReview("req-1", ReviewBuildInput{Facts: f, AgentItems: items, KG: KGInfo{Available: true},
		Redact: func(s string) string { return strings.ReplaceAll(s, "sk-secret-123", "[REDACTED]") }})
	if err != nil {
		t.Fatal(err)
	}
	var withProof, without *ReviewItem
	for i := range r.Items {
		switch r.Items[i].Section {
		case SectionArchitecture:
			withProof = &r.Items[i]
		case SectionFlow:
			without = &r.Items[i]
		}
	}
	if withProof == nil || without == nil {
		t.Fatalf("items missing: %+v", r.Items)
	}
	if withProof.Unverified || strings.Contains(withProof.Title, "sk-secret") || withProof.Origin != OriginAgent {
		t.Fatalf("proof item %+v", withProof)
	}
	if !without.Unverified {
		t.Fatalf("an agent item without evidence must be unverified")
	}
}

func TestBuildOutcomeReview_CapsItemsAndText(t *testing.T) {
	f := baseFacts(ReviewPhaseDesign)
	var items []ReviewItem
	for i := 0; i < MaxReviewItems+20; i++ {
		items = append(items, ReviewItem{Section: SectionContracts, Change: ChangeAdded, Title: strings.Repeat("é", 400) + string(rune('A'+i%26)) + strings.Repeat("x", i), Severity: SeverityInfo})
	}
	r, err := BuildOutcomeReview("req-1", ReviewBuildInput{Facts: f, AgentItems: items})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != MaxReviewItems || !r.Truncated || !contains(r.Warnings, "truncated:items") {
		t.Fatalf("items %d truncated %v warnings %v", len(r.Items), r.Truncated, r.Warnings)
	}
	for _, it := range r.Items {
		if n := len([]rune(it.Title)); n > MaxReviewTitleRunes {
			t.Fatalf("title has %d runes", n)
		}
	}
}

func TestReviewItemKey_StableAndHeadKeyOrderIndependent(t *testing.T) {
	if ReviewItemKey(SectionFlow, "edge", "a->b") != ReviewItemKey(SectionFlow, "edge", "a->b") {
		t.Fatalf("key must be stable")
	}
	if ReviewItemKey(SectionFlow, "edge", "a->b") == ReviewItemKey(SectionArchitecture, "edge", "a->b") {
		t.Fatalf("section is part of the key")
	}
	h1 := []ReviewHead{{Branch: "a", Commit: "1"}, {Branch: "b", Commit: "2"}}
	h2 := []ReviewHead{{Branch: "b", Commit: "2"}, {Branch: "a", Commit: "1"}}
	if HeadKey(ReviewPhaseDelivered, h1) != HeadKey(ReviewPhaseDelivered, h2) {
		t.Fatalf("head key depends on order")
	}
	if HeadKey(ReviewPhaseDesign, h1) != ReviewDesignHeadKey {
		t.Fatalf("a design review has the design head key")
	}
	if k := HeadKey(ReviewPhaseDelivered, nil); k != ReviewNoCodeHeadKey || k == ReviewDesignHeadKey {
		t.Fatalf("a delivered review without code must not collide with the design review: %q", k)
	}
}

func TestBuildOutcomeReview_RejectsBadPhase(t *testing.T) {
	if _, err := BuildOutcomeReview("r", ReviewBuildInput{Facts: Facts{Phase: "nope"}}); err == nil {
		t.Fatalf("an unknown phase must be refused")
	}
}

func contains(in []string, s string) bool {
	for _, v := range in {
		if v == s {
			return true
		}
	}
	return false
}

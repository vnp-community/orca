package domain

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// repoProbe is a repository made of the given files; a directory exists when a file lies under it.
type repoProbe struct {
	files map[string]bool
	dirs  map[string]bool
	err   error
}

func newRepoProbe(files ...string) *repoProbe {
	p := &repoProbe{files: map[string]bool{}, dirs: map[string]bool{}}
	for _, f := range files {
		p.files[f] = true
		for d := path.Dir(f); d != "." && d != "/"; d = path.Dir(d) {
			p.dirs[d] = true
		}
	}
	return p
}

func (p *repoProbe) Probe(rel string) (bool, bool, error) {
	return p.files[rel] || p.dirs[rel], p.dirs[rel], p.err
}

var sampleRepo = []string{
	"backend/services/platform/system-setting/internal/config/config.go",
	"backend/services/platform/system-setting/internal/domain/ai_config.go",
	"backend/services/platform/system-setting/internal/handler/router.go",
	"backend/services/platform/system-setting/internal/usecase/ai/doc.go",
	"backend/services/platform/system-setting/internal/repository/mysql/other.go",
	"backend/shared/pkg/netguard/netguard.go",
	"docs/SRS.md",
}

func sevenACs() []string {
	return []string{"AC-AI-01", "AC-AI-02", "AC-AI-03", "AC-AI-04", "AC-AI-05", "AC-AI-06", "AC-AI-07"}
}

func readBundleFixture(t *testing.T, dir string) (PlanProposal, map[string]string) {
	t.Helper()
	root := filepath.Join("testdata", "openspec_bundle", dir)
	md, err := os.ReadFile(filepath.Join(root, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	p, v := ParsePlanRegion(string(md), "REQ-900")
	if len(v) != 0 {
		t.Fatalf("fixture %s does not parse: %+v", dir, v)
	}
	specs := map[string]string{}
	matches, _ := filepath.Glob(filepath.Join(root, "specs", "*", "spec.md"))
	for _, m := range matches {
		b, _ := os.ReadFile(m)
		rel, _ := filepath.Rel(root, m)
		specs[filepath.ToSlash(rel)] = string(b)
	}
	return p, specs
}

func bundleCodes(v []BundleViolation) []string {
	out := make([]string, len(v))
	for i, x := range v {
		out[i] = x.Code
	}
	return out
}

func TestCheckBundle_AcceptsTheGoodSampleFromTheSpike(t *testing.T) {
	p, specs := readBundleFixture(t, "good")
	v, err := CheckBundle(BundleInput{Proposal: p, Specs: specs, ACIDs: sevenACs(), MinTasks: 6, MaxTasks: 30, Probe: newRepoProbe(sampleRepo...)})
	if err != nil || len(v) != 0 {
		t.Fatalf("the spike's good bundle must pass cleanly: %v %v", v, err)
	}
}

func mutate(t *testing.T, p PlanProposal, edit func(tasks []TaskProposal)) PlanProposal {
	t.Helper()
	out := p
	out.Phases = nil
	for _, ph := range p.Phases {
		cp := ph
		cp.Tasks = append([]TaskProposal(nil), ph.Tasks...)
		out.Phases = append(out.Phases, cp)
	}
	edit(out.Phases[0].Tasks)
	return out
}

func TestCheckBundle_EachCodeHasAFailingAndAPassingExample(t *testing.T) {
	good, specs := readBundleFixture(t, "good")
	probe := newRepoProbe(sampleRepo...)
	base := func() BundleInput {
		return BundleInput{Proposal: good, Specs: specs, ACIDs: sevenACs(), MinTasks: 6, MaxTasks: 30, Probe: probe}
	}
	cases := []struct {
		name string
		in   func() BundleInput
		code string
	}{
		{"too few tasks", func() BundleInput { in := base(); in.MinTasks = 25; return in }, BundleTaskCount},
		{"too many tasks", func() BundleInput { in := base(); in.MaxTasks = 5; return in }, BundleTaskCount},
		{"task without Verify", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) {
				ts[0].Description = strings.ReplaceAll(ts[0].Description, "Verify:", "Check:")
			})
			return in
		}, BundleTaskIncomplete},
		{"task with an empty Done when", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) {
				ts[1].Description = "Serves: FR-1\nCode: modify `docs/SRS.md::x`\nVerify: AC-AI-01 go test\nDone when:"
			})
			return in
		}, BundleTaskIncomplete},
		{"Code without a target", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) {
				ts[0].Description = "Serves: FR-1\nCode: do the thing\nVerify: v\nDone when: d"
			})
			return in
		}, BundleCodeTarget},
		{"modify of an invented file", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) {
				ts[0].Description = "Serves: FR-1\nCode: modify `backend/nope/ghost.go::Ghost`\nVerify: v\nDone when: d"
			})
			return in
		}, BundleCodeTarget},
		{"create over an existing file", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) {
				ts[0].Description = "Serves: FR-1\nCode: create `docs/SRS.md::x`\nVerify: v\nDone when: d"
			})
			return in
		}, BundleCodeTarget},
		{"create deep inside an invented tree", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) {
				ts[0].Description = "Serves: FR-1\nCode: create `backend/invented/deep/tree/x.go::X`\nVerify: v\nDone when: d"
			})
			return in
		}, BundleCodeTarget},
		{"path escaping the repo", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) {
				ts[0].Description = "Serves: FR-1\nCode: modify `../outside.go::X`\nVerify: v\nDone when: d"
			})
			return in
		}, BundleCodeTarget},
		{"ac= names an id the Request does not have", func() BundleInput {
			in := base()
			in.Proposal = mutate(t, good, func(ts []TaskProposal) { ts[0].Satisfies = []string{"AC-AI-99"} })
			return in
		}, BundleACUnknown},
		{"a criterion without a scenario", func() BundleInput {
			in := base()
			in.ACIDs = append(sevenACs(), "AC-AI-08")
			in.Proposal = mutate(t, good, func(ts []TaskProposal) { ts[0].Satisfies = append(ts[0].Satisfies, "AC-AI-08") })
			return in
		}, BundleACNotCovered},
		{"a criterion no task claims", func() BundleInput {
			in := base()
			in.Proposal = good
			in.Proposal = mutateAll(good, func(t *TaskProposal) { t.Satisfies = filterOut(t.Satisfies, "AC-AI-04") })
			return in
		}, BundleACNoTask},
		{"no spec files at all", func() BundleInput { in := base(); in.Specs = nil; return in }, BundleSpecsMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := CheckBundle(tc.in())
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, x := range v {
				found = found || x.Code == tc.code
				if x.Where == "" || x.Message == "" {
					t.Fatalf("a finding must say where and what: %+v", x)
				}
			}
			if !found {
				t.Fatalf("want %s, got %v", tc.code, bundleCodes(v))
			}
		})
	}
}

func mutateAll(p PlanProposal, edit func(t *TaskProposal)) PlanProposal {
	out := p
	out.Phases = nil
	for _, ph := range p.Phases {
		cp := ph
		cp.Tasks = append([]TaskProposal(nil), ph.Tasks...)
		for i := range cp.Tasks {
			edit(&cp.Tasks[i])
		}
		out.Phases = append(out.Phases, cp)
	}
	return out
}

func filterOut(in []string, drop string) []string {
	var out []string
	for _, s := range in {
		if s != drop {
			out = append(out, s)
		}
	}
	return out
}

func TestCheckBundle_ScenarioNamesMatchOnTheWholeIdOnly(t *testing.T) {
	specs := map[string]string{"specs/x/spec.md": "#### Scenario: AC-10 something else\n#### Scenario: AC-2: with a colon\n#### Scenario:AC-3\n"}
	p := PlanProposal{Tasks: []TaskProposal{{Title: "t", Description: "Serves: a\nCode: create `src/x.go::X`\nVerify: v\nDone when: d", Satisfies: []string{"AC-1", "AC-2", "AC-3"}}}}
	v, _ := CheckBundle(BundleInput{Proposal: p, Specs: specs, ACIDs: []string{"AC-1", "AC-2", "AC-3"}, MinTasks: 1, MaxTasks: 5, Probe: newRepoProbe("src/y.go")})
	var uncovered []string
	for _, x := range v {
		if x.Code == BundleACNotCovered {
			uncovered = append(uncovered, x.Where)
		}
	}
	if len(uncovered) != 1 || uncovered[0] != "AC-1" {
		t.Fatalf("AC-1 is not covered by AC-10; AC-2 and AC-3 are: %v", v)
	}
}

func TestCheckBundle_ATaskMayModifyAFileAnEarlierTaskCreates(t *testing.T) {
	p := PlanProposal{Tasks: []TaskProposal{
		{Title: "a", Description: "Serves: a\nCode: create `src/new/a.go::A`\nVerify: v\nDone when: d"},
		{Title: "b", Description: "Serves: a\nCode: modify `src/new/a.go::A`; create `src/new/a_test.go::TestA`\nVerify: v\nDone when: d"},
		{Title: "c", Description: "Serves: a\nCode: modify `src/new/later.go::L`\nVerify: v\nDone when: d"},
	}}
	v, _ := CheckBundle(BundleInput{Proposal: p, MinTasks: 1, MaxTasks: 5, Probe: newRepoProbe("src/old.go")})
	if len(v) != 1 || v[0].Where != "T1.3" || v[0].Code != BundleCodeTarget {
		t.Fatalf("only the file nobody creates is a fault: %v", v)
	}
}

func TestCheckBundle_APlanWithoutAnyCriteriaNeedsNoSpecsButRejectsAcAttributes(t *testing.T) {
	p := PlanProposal{Tasks: []TaskProposal{{Title: "a", Description: "Serves: a\nCode: create `src/x.go::X`\nVerify: v\nDone when: d", Satisfies: []string{"AC-1"}}}}
	v, _ := CheckBundle(BundleInput{Proposal: p, MinTasks: 1, MaxTasks: 5, Probe: newRepoProbe("src/y.go")})
	if len(v) != 1 || v[0].Code != BundleACUnknown {
		t.Fatalf("%v", v)
	}
}

func TestCheckBundle_ProbeErrorsAreErrorsNotFindings(t *testing.T) {
	p := PlanProposal{Tasks: []TaskProposal{{Title: "a", Description: "Serves: a\nCode: modify `src/x.go::X`\nVerify: v\nDone when: d"}}}
	probe := newRepoProbe()
	probe.err = os.ErrDeadlineExceeded
	if _, err := CheckBundle(BundleInput{Proposal: p, MinTasks: 1, MaxTasks: 5, Probe: probe}); err == nil {
		t.Fatal("an unreadable workspace must not read as an invalid bundle")
	}
}

func TestBundleViolationString(t *testing.T) {
	if got := (BundleViolation{Code: "C", Where: "T1.1", Message: "m"}).String(); got != "C (T1.1): m" {
		t.Fatal(got)
	}
	if got := (BundleViolation{Code: "C", Message: "m"}).String(); got != "C: m" {
		t.Fatal(got)
	}
}

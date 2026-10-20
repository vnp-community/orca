package domain

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The OpenSpec change under testdata/openspec_vietnamese is written in Vietnamese the way the Output language rule asks:
// prose translated, every token that openspec or Orca's parsers match literally left in English. These tests pin which tokens
// those are, by showing what breaks when each one is translated.

const (
	vietnameseChangeID = "them-dang-nhap"
	vietnameseRequest  = "REQ-901"
)

func vietnameseACs() []string { return []string{"AC-1", "AC-2"} }

func vietnameseRepo() *repoProbe {
	return newRepoProbe("services/auth-service/internal/handler/router.go", "docs/SRS.md")
}

func readVietnameseFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "openspec_vietnamese", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// checkVietnamese runs the parser and the bundle check over the given tasks.md and spec text.
func checkVietnamese(t *testing.T, tasks, spec string) (PlanProposal, []Violation, []BundleViolation) {
	t.Helper()
	p, parse := ParsePlanRegion(tasks, vietnameseRequest)
	if len(parse) != 0 {
		return p, parse, nil
	}
	specs := map[string]string{"specs/dang-nhap/spec.md": spec}
	got, err := CheckBundle(BundleInput{Proposal: p, Specs: specs, ACIDs: vietnameseACs(), MinTasks: 2, MaxTasks: 10, Probe: vietnameseRepo()})
	if err != nil {
		t.Fatal(err)
	}
	return p, nil, got
}

func hasBundleCode(vs []BundleViolation, code string) bool {
	for _, v := range vs {
		if v.Code == code {
			return true
		}
	}
	return false
}

func TestOpenSpecVietnamese_TasksMdParsesWithAccentedTitles(t *testing.T) {
	p, v := ParsePlanRegion(readVietnameseFile(t, "tasks.md"), vietnameseRequest)
	if len(v) != 0 {
		t.Fatalf("the Vietnamese tasks.md must parse without violations: %+v", v)
	}
	if len(p.Phases) != 2 || len(p.Phases[0].Tasks) != 2 || len(p.Phases[1].Tasks) != 2 {
		t.Fatalf("expected two phases of two tasks, got %+v", p)
	}
	if got, want := p.Phases[0].Title, "Đăng nhập bằng tài khoản công ty"; got != want {
		t.Fatalf("phase title lost its accents: %q, want %q", got, want)
	}
	if got, want := p.Phases[1].Tasks[0].Title, "Thêm bộ đếm lần nhập sai mật khẩu"; got != want {
		t.Fatalf("task title lost its accents: %q, want %q", got, want)
	}
	if !strings.Contains(p.Phases[0].Description, "Xong khi kiểm thử của riêng nó đạt") {
		t.Fatalf("phase description lost its text: %q", p.Phases[0].Description)
	}
	if deps := p.Phases[0].Tasks[1].DependsOnIndices; len(deps) != 1 || deps[0] != 0 {
		t.Fatalf("[depends=T1.1] must point at the first task of the phase, got %v", deps)
	}
	if sat := p.Phases[1].Tasks[0].Satisfies; len(sat) != 1 || sat[0] != "AC-2" {
		t.Fatalf("[ac=AC-2] was not read: %v", sat)
	}
}

func TestOpenSpecVietnamese_BundlePassesCheckBundle(t *testing.T) {
	_, parse, got := checkVietnamese(t, readVietnameseFile(t, "tasks.md"), readVietnameseFile(t, "specs/dang-nhap/spec.md"))
	if len(parse) != 0 || len(got) != 0 {
		t.Fatalf("the Vietnamese bundle must pass cleanly: parse=%+v bundle=%v", parse, got)
	}
}

func TestOpenSpecVietnamese_ScenarioNameMatchesTheAcceptanceIdOnABoundary(t *testing.T) {
	names := []string{"AC-1 Đăng nhập thành công bằng tài khoản công ty", "AC-10 Trường hợp khác"}
	if !anyScenarioStartsWith(names, "AC-1") {
		t.Fatal("'AC-1 Đăng nhập ...' must satisfy AC-1")
	}
	if anyScenarioStartsWith(names[1:], "AC-1") {
		t.Fatal("'AC-10 ...' must not satisfy AC-1")
	}
	if !anyScenarioStartsWith([]string{"AC-2: Khoá tài khoản"}, "AC-2") {
		t.Fatal("'AC-2: ...' must satisfy AC-2")
	}
	// The id ends at a boundary; a Vietnamese letter right after it is a boundary, so it still counts as the id.
	if !anyScenarioStartsWith([]string{"AC-1 đăng nhập"}, "AC-1") {
		t.Fatal("a lower-case Vietnamese word after the id must not hide the id")
	}
}

func TestOpenSpecVietnamese_TranslatedGrammarIsRejected(t *testing.T) {
	tasks := readVietnameseFile(t, "tasks.md")
	spec := readVietnameseFile(t, "specs/dang-nhap/spec.md")
	cases := []struct {
		name  string
		tasks string
		spec  string
		check func(t *testing.T, parse []Violation, bundle []BundleViolation)
	}{
		{
			name: "scenario heading translated",
			spec: strings.ReplaceAll(spec, "#### Scenario:", "#### Kịch bản:"),
			check: func(t *testing.T, parse []Violation, bundle []BundleViolation) {
				if !hasBundleCode(bundle, BundleACNotCovered) {
					t.Fatalf("a translated '#### Kịch bản:' must leave the acceptance criteria uncovered: %v", bundle)
				}
			},
		},
		{
			name:  "Done when label translated",
			tasks: strings.Replace(tasks, "  Done when:", "  Xong khi:", 1),
			check: func(t *testing.T, parse []Violation, bundle []BundleViolation) {
				if !hasBundleCode(bundle, BundleTaskIncomplete) {
					t.Fatalf("a task without the 'Done when:' line must be incomplete: %v", bundle)
				}
			},
		},
		{
			name:  "Serves label translated",
			tasks: strings.Replace(tasks, "  Serves:", "  Phục vụ:", 1),
			check: func(t *testing.T, parse []Violation, bundle []BundleViolation) {
				if !hasBundleCode(bundle, BundleTaskIncomplete) {
					t.Fatalf("a task without the 'Serves:' line must be incomplete: %v", bundle)
				}
			},
		},
		{
			name:  "unknown ASCII attribute",
			tasks: strings.Replace(tasks, "[type=feature] [h=3] [ac=AC-1]", "[type=feature] [h=3] [nhan=quan-trong] [ac=AC-1]", 1),
			check: func(t *testing.T, parse []Violation, bundle []BundleViolation) {
				if len(parse) != 1 || parse[0].Code != "TASKSMD_UNKNOWN_ATTR" {
					t.Fatalf("an unknown attribute must be TASKSMD_UNKNOWN_ATTR: %+v", parse)
				}
			},
		},
		{
			// The attribute grammar is ASCII: a translated key is not read as an attribute at all, it stays in the title, and the
			// acceptance criterion it was meant to carry is then served by no task.
			name:  "ac attribute key translated",
			tasks: strings.ReplaceAll(tasks, "[ac=AC-1]", "[tiêu_chí=AC-1]"),
			check: func(t *testing.T, parse []Violation, bundle []BundleViolation) {
				if len(parse) != 0 {
					t.Fatalf("a non-ASCII key is part of the title, not a parse error: %+v", parse)
				}
				if !hasBundleCode(bundle, BundleACNoTask) {
					t.Fatalf("AC-1 must be left without a task: %v", bundle)
				}
			},
		},
		{
			name:  "code verb translated",
			tasks: strings.Replace(tasks, "Code: create `services/auth-service/internal/handler/company_login.go", "Code: tạo `services/auth-service/internal/handler/company_login.go", 1),
			check: func(t *testing.T, parse []Violation, bundle []BundleViolation) {
				if !hasBundleCode(bundle, BundleCodeTarget) {
					t.Fatalf("'tạo' is not a verb of the Code: line: %v", bundle)
				}
			},
		},
		{
			name:  "phase heading translated",
			tasks: strings.Replace(tasks, "## PH-1 ", "## GIAI-ĐOẠN-1 ", 1),
			check: func(t *testing.T, parse []Violation, bundle []BundleViolation) {
				if len(parse) == 0 {
					t.Fatal("a translated phase heading must be refused by the parser")
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tk, sp := c.tasks, c.spec
			if tk == "" {
				tk = tasks
			}
			if sp == "" {
				sp = spec
			}
			_, parse, bundle := checkVietnamese(t, tk, sp)
			c.check(t, parse, bundle)
		})
	}
}

// runOpenSpecValidate copies the change into a temp repository and returns the parsed `openspec validate --strict --json` answer.
func runOpenSpecValidate(t *testing.T, spec string) (valid bool, messages []string) {
	t.Helper()
	root := t.TempDir()
	change := filepath.Join(root, "openspec", "changes", vietnameseChangeID)
	if err := os.MkdirAll(filepath.Join(change, "specs", "dang-nhap"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, text := range map[string]string{
		"proposal.md":             readVietnameseFile(t, "proposal.md"),
		"tasks.md":                readVietnameseFile(t, "tasks.md"),
		"specs/dang-nhap/spec.md": spec,
	} {
		if err := os.WriteFile(filepath.Join(change, filepath.FromSlash(rel)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init failed (%v): %s", err, out)
	}
	cmd := exec.Command("openspec", "validate", vietnameseChangeID, "--strict", "--json")
	cmd.Dir = root
	// A failing validation exits non-zero but still prints the JSON; only a missing document is an error here.
	out, runErr := cmd.Output()
	var doc struct {
		Items []struct {
			Valid  bool `json:"valid"`
			Issues []struct {
				Message string `json:"message"`
			} `json:"issues"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || len(doc.Items) != 1 {
		t.Fatalf("openspec validate printed no usable JSON (run error %v): %s", runErr, out)
	}
	for _, i := range doc.Items[0].Issues {
		messages = append(messages, i.Message)
	}
	return doc.Items[0].Valid, messages
}

func TestOpenSpecVietnamese_OpenSpecValidateAcceptsTheBundleAndRefusesTranslatedGrammar(t *testing.T) {
	if _, err := exec.LookPath("openspec"); err != nil {
		t.Skip("openspec CLI not installed: the CLI half of the Vietnamese proof is skipped (the parser half above still runs)")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	spec := readVietnameseFile(t, "specs/dang-nhap/spec.md")

	if valid, msgs := runOpenSpecValidate(t, spec); !valid {
		t.Fatalf("openspec validate --strict must accept the Vietnamese change: %v", msgs)
	}

	t.Run("SHALL and MUST replaced by phải", func(t *testing.T) {
		valid, msgs := runOpenSpecValidate(t, strings.NewReplacer("SHALL", "phải", "MUST", "phải").Replace(spec))
		if valid || !strings.Contains(strings.Join(msgs, "\n"), "must contain SHALL or MUST") {
			t.Fatalf("a requirement without SHALL or MUST must be refused with that message, valid=%v msgs=%v", valid, msgs)
		}
	})
	t.Run("delta heading translated", func(t *testing.T) {
		valid, msgs := runOpenSpecValidate(t, strings.Replace(spec, "## ADDED Requirements", "## YÊU CẦU THÊM MỚI", 1))
		if valid || !strings.Contains(strings.Join(msgs, "\n"), "No delta sections found") {
			t.Fatalf("a translated delta heading must be refused with 'No delta sections found', valid=%v msgs=%v", valid, msgs)
		}
	})
	t.Run("scenario heading translated passes openspec but not Orca", func(t *testing.T) {
		translated := strings.ReplaceAll(spec, "#### Scenario:", "#### Kịch bản:")
		if valid, msgs := runOpenSpecValidate(t, translated); !valid {
			t.Fatalf("openspec does not read scenario headings, so it accepts this: %v", msgs)
		}
		_, parse, bundle := checkVietnamese(t, readVietnameseFile(t, "tasks.md"), translated)
		if len(parse) != 0 || !hasBundleCode(bundle, BundleACNotCovered) {
			t.Fatalf("Orca must still refuse it as REQUEST_SPEC_AC_NOT_COVERED: parse=%+v bundle=%v", parse, bundle)
		}
	})
}

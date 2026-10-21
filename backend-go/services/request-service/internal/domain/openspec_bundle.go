package domain

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Codes of the findings CheckBundle reports. They travel in the retry note and the failed run's message, never as RPC errors.
const (
	BundleTaskCount       = "REQUEST_SPEC_TASK_COUNT"
	BundleTaskIncomplete  = "REQUEST_SPEC_TASK_INCOMPLETE"
	BundleCodeTarget      = "REQUEST_SPEC_CODE_TARGET_INVALID"
	BundleACNotCovered    = "REQUEST_SPEC_AC_NOT_COVERED"
	BundleACUnknown       = "REQUEST_SPEC_AC_UNKNOWN"
	BundleACNoTask        = "REQUEST_SPEC_AC_NO_TASK"
	BundleValidation      = "REQUEST_SPEC_VALIDATION_FAILED"
	BundleSpecsMissing    = "REQUEST_SPEC_SPECS_MISSING"
	bundleScenarioHeading = "#### Scenario:"
)

// BundleViolation is one fault in the written change, worded for the agent that has to fix it.
type BundleViolation struct {
	Code    string
	Where   string
	Message string
}

func (v BundleViolation) String() string {
	if v.Where == "" {
		return v.Code + ": " + v.Message
	}
	return v.Code + " (" + v.Where + "): " + v.Message
}

// PathProbe says what exists in the proposal worktree, which is the repository at its base ref.
type PathProbe interface {
	Probe(rel string) (exists, isDir bool, err error)
}

// BundleInput is a parsed tasks.md, the spec files and what the Request promised.
type BundleInput struct {
	Proposal PlanProposal
	// Specs maps a spec file's path (relative to the change) to its text.
	Specs map[string]string
	// ACIDs are the Request's active acceptance criteria ids.
	ACIDs []string
	// MinTasks and MaxTasks are the task count the Request's size calls for.
	MinTasks, MaxTasks int
	Probe              PathProbe
}

var (
	codeTargetRe   = regexp.MustCompile("(modify|create)\\s+`([^`]+)`")
	scenarioNameRe = regexp.MustCompile(`(?m)^####\s+Scenario:\s*(.+?)\s*$`)
)

// descriptionKeys are the four lines every task carries so that another agent can act on it alone.
var descriptionKeys = []string{"Serves:", "Code:", "Verify:", "Done when:"}

// CheckBundle runs the checks that need only the files: counts, the four description lines, code targets, and the
// acceptance criteria against tasks and scenarios. `openspec validate` is run by the caller.
func CheckBundle(in BundleInput) ([]BundleViolation, error) {
	var out []BundleViolation
	tasks := flattenTasks(in.Proposal)
	if len(tasks) < in.MinTasks || len(tasks) > in.MaxTasks {
		out = append(out, BundleViolation{Code: BundleTaskCount, Where: "tasks.md",
			Message: fmt.Sprintf("%d tasks; a Request of this size needs between %d and %d", len(tasks), in.MinTasks, in.MaxTasks)})
	}
	known := map[string]bool{}
	for _, id := range in.ACIDs {
		known[id] = true
	}
	satisfied := map[string]bool{}
	created := map[string]bool{}
	for _, t := range tasks {
		lines := descriptionLines(t.task.Description)
		for _, key := range descriptionKeys {
			if strings.TrimSpace(lines[key]) == "" {
				out = append(out, BundleViolation{Code: BundleTaskIncomplete, Where: t.id, Message: "the description has no " + key + " line"})
			}
		}
		if code := lines["Code:"]; code != "" {
			vs, err := checkCodeTargets(t.id, code, created, in.Probe)
			if err != nil {
				return nil, err
			}
			out = append(out, vs...)
		}
		for _, ac := range t.task.Satisfies {
			if !known[ac] {
				out = append(out, BundleViolation{Code: BundleACUnknown, Where: t.id, Message: fmt.Sprintf("ac=%s is not an acceptance criterion of this Request (%s)", ac, joinOrNone(in.ACIDs))})
				continue
			}
			satisfied[ac] = true
		}
	}

	names := scenarioNames(in.Specs)
	if len(in.ACIDs) > 0 && len(in.Specs) == 0 {
		out = append(out, BundleViolation{Code: BundleSpecsMissing, Where: "specs/", Message: "the change has no specs/<capability>/spec.md; the acceptance criteria need scenarios"})
	}
	for _, ac := range in.ACIDs {
		if !anyScenarioStartsWith(names, ac) {
			out = append(out, BundleViolation{Code: BundleACNotCovered, Where: ac, Message: "no `#### Scenario:` whose name starts with " + ac})
		}
		if !satisfied[ac] {
			out = append(out, BundleViolation{Code: BundleACNoTask, Where: ac, Message: "no task lists " + ac + " in ac="})
		}
	}
	return out, nil
}

type labelledTask struct {
	id   string
	task TaskProposal
}

// flattenTasks lists tasks in document order with the T<phase>.<n> id they were written under.
func flattenTasks(p PlanProposal) []labelledTask {
	var out []labelledTask
	if len(p.Phases) > 0 {
		for i, ph := range p.Phases {
			for j, t := range ph.Tasks {
				out = append(out, labelledTask{id: fmt.Sprintf("T%d.%d", i+1, j+1), task: t})
			}
		}
		return out
	}
	for j, t := range p.Tasks {
		out = append(out, labelledTask{id: fmt.Sprintf("T1.%d", j+1), task: t})
	}
	return out
}

// descriptionLines maps each of the four keys to the text after it; a key that appears twice keeps the first.
func descriptionLines(desc string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(desc, "\n") {
		line = strings.TrimSpace(line)
		for _, key := range descriptionKeys {
			if strings.HasPrefix(line, key) {
				if _, seen := out[key]; !seen {
					out[key] = strings.TrimSpace(strings.TrimPrefix(line, key))
				}
			}
		}
	}
	return out
}

func checkCodeTargets(taskID, code string, created map[string]bool, probe PathProbe) ([]BundleViolation, error) {
	matches := codeTargetRe.FindAllStringSubmatch(code, -1)
	if len(matches) == 0 {
		return []BundleViolation{{Code: BundleCodeTarget, Where: taskID, Message: "Code: must name targets as the word modify or create followed by `<path>::<symbol>` in backticks, for example modify `main.go::Handler`; the verb goes outside the backticks"}}, nil
	}
	var out []BundleViolation
	bad := func(msg string) {
		out = append(out, BundleViolation{Code: BundleCodeTarget, Where: taskID, Message: msg})
	}
	for _, m := range matches {
		verb, target := m[1], m[2]
		rel := target
		if i := strings.Index(rel, "::"); i >= 0 {
			rel = rel[:i]
		}
		rel = path.Clean(strings.TrimSpace(rel))
		if rel == "." || rel == "" || path.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
			bad("`" + target + "` is not a path inside the repository")
			continue
		}
		exists, isDir, err := probe.Probe(rel)
		if err != nil {
			return nil, err
		}
		switch verb {
		case "modify":
			if (!exists || isDir) && !created[rel] {
				bad("modify `" + rel + "`: that file does not exist in the repository and no earlier task creates it")
			}
		case "create":
			switch {
			case exists && !isDir && !created[rel]:
				bad("create `" + rel + "`: that file already exists; use modify")
			case isDir:
				bad("create `" + rel + "`: that is a directory")
			default:
				if ok, err := parentPlausible(rel, created, probe); err != nil {
					return nil, err
				} else if !ok {
					bad("create `" + rel + "`: neither its directory nor that directory's parent exists, so the path looks invented")
				}
			}
			created[rel] = true
		}
	}
	return out, nil
}

// parentPlausible accepts a new file in an existing directory or in a new directory directly below an existing one
// (a new package); deeper invention is refused. Files created by earlier tasks count as existing.
func parentPlausible(rel string, created map[string]bool, probe PathProbe) (bool, error) {
	dir := path.Dir(rel)
	for i := 0; i < 2; i++ {
		if dir == "." || dir == "/" || dir == "" {
			return true, nil
		}
		if createdUnder(created, dir) {
			return true, nil
		}
		exists, isDir, err := probe.Probe(dir)
		if err != nil {
			return false, err
		}
		if exists && isDir {
			return true, nil
		}
		dir = path.Dir(dir)
	}
	return false, nil
}

func createdUnder(created map[string]bool, dir string) bool {
	for p := range created {
		if strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

func scenarioNames(specs map[string]string) []string {
	var names []string
	for _, text := range specs {
		for _, m := range scenarioNameRe.FindAllStringSubmatch(text, -1) {
			names = append(names, m[1])
		}
	}
	sort.Strings(names)
	return names
}

// anyScenarioStartsWith matches "AC-1 does x" and "AC-1: does x" but not "AC-10 ...": the id must end at a boundary.
func anyScenarioStartsWith(names []string, id string) bool {
	for _, n := range names {
		if !strings.HasPrefix(n, id) {
			continue
		}
		rest := n[len(id):]
		if rest == "" {
			return true
		}
		c := rest[0]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == '_') {
			return true
		}
	}
	return false
}

func joinOrNone(ids []string) string {
	if len(ids) == 0 {
		return "none listed"
	}
	return strings.Join(ids, ", ")
}

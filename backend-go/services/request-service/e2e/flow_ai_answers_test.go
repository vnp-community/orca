//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/e2e/stubs"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// The stub model answers the analysis and planning prompts with documents that pass the service's own validation.
// It reads the Request's type, size and acceptance criteria from the prompt, so one set of answers serves every
// scenario; the classification prompt is left to the stub's built-in marker reader.
const goldenDir = "../internal/domain/testdata/"

var (
	promptTypeSize  = regexp.MustCompile(`(?m)^type: (\w+)\nsize: (\w+)$`)
	promptCriterion = regexp.MustCompile(`(?m)^- (AC-[\w-]+): `)
	promptMinOpts   = regexp.MustCompile(`Give at least (\d+) options`)
)

func installFlowAnswers(script *stubs.AgentScript) {
	script.AddHandler(func(method string, params map[string]any) (any, bool) {
		if method != "ai.complete" {
			return nil, false
		}
		prompt, _ := params["prompt"].(string)
		var doc any
		switch {
		case strings.Contains(prompt, "proposing solutions for the request below"):
			doc = solutionAnswer(prompt)
		case strings.Contains(prompt, "Diagnose the problem described in <request>"):
			doc = golden("analysis_documents/diagnosis_valid.json")
		case strings.Contains(prompt, "Investigate the question in <request>"):
			doc = golden("analysis_documents/findings_valid.json")
		case strings.Contains(prompt, "Answer the question in <request>"):
			doc = golden("analysis_documents/answer_valid.json")
		case strings.Contains(prompt, "executable work plan"):
			doc = planAnswer(prompt)
		default:
			return nil, false
		}
		b, err := json.Marshal(doc)
		if err != nil {
			panic(err)
		}
		return map[string]any{"content": string(b)}, true
	})
}

func golden(rel string) map[string]any {
	raw, err := os.ReadFile(goldenDir + rel)
	if err != nil {
		panic(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		panic(fmt.Sprintf("%s: %v", rel, err))
	}
	return m
}

// solutionAnswer has the number of options the prompt asks for and covers every acceptance criterion it lists,
// with no open question, so approving it needs no Clarification.
func solutionAnswer(prompt string) map[string]any {
	doc := golden("solution_options/valid_two_options.json")
	n := 2
	if m := promptMinOpts.FindStringSubmatch(prompt); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil && v > n {
			n = v
		}
	}
	options := doc["options"].([]any)
	for len(options) < n {
		o := cloneMap(options[1].(map[string]any))
		o["id"] = fmt.Sprintf("opt-%d", len(options)+1)
		options = append(options, o)
	}
	doc["options"] = options
	doc["assumptions"], doc["open_questions"] = []any{}, []any{}
	var coverage []any
	for _, m := range promptCriterion.FindAllStringSubmatch(prompt, -1) {
		coverage = append(coverage, map[string]any{"ac_id": m[1], "option_ids": []string{"opt-1"}, "status": "covered", "note": ""})
	}
	if coverage != nil {
		doc["requirement_coverage"] = coverage
	}
	return doc
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type planTaskSpec struct {
	title        string
	labels       []string
	irreversible bool
	afterPrev    bool
}

// planSpecs are the work items per type; the labels are the ones the type policies check (CR-REQ-012, 014).
func planSpecs(t domain.RequestType) []planTaskSpec {
	switch t {
	case domain.RequestTypeBug, domain.RequestTypeSecurity:
		return []planTaskSpec{{title: "Fix the defect"}, {title: "Add the regression test", labels: []string{domain.PlanLabelTestRegression}, afterPrev: true}, {title: "Update the notes"}}
	case domain.RequestTypePerformance:
		return []planTaskSpec{{title: "Measure the baseline", labels: []string{domain.PolicyLabelCheckBaseline}}, {title: "Optimise the hot path", afterPrev: true},
			{title: "Measure again", labels: []string{domain.PolicyLabelCheckAfter}, afterPrev: true}}
	case domain.RequestTypeRefactor:
		return []planTaskSpec{{title: "Run the existing tests", labels: []string{domain.PolicyLabelCheckTestsPre}}, {title: "Restructure the module", afterPrev: true},
			{title: "Run the tests again", labels: []string{domain.PolicyLabelCheckTestsAfter}, afterPrev: true}}
	case domain.RequestTypeOpsRequest:
		return []planTaskSpec{{title: "Take a backup"}, {title: "Apply the migration", irreversible: true, afterPrev: true},
			{title: "Restore the backup if it failed", labels: []string{domain.PolicyLabelRollback}, afterPrev: true}}
	case domain.RequestTypeTask, domain.RequestTypeDocs:
		return []planTaskSpec{{title: "Do the work"}}
	}
	return []planTaskSpec{{title: "Implement the change"}, {title: "Add the tests", afterPrev: true}, {title: "Update the documentation"}}
}

// planAnswer fits the shape the type and size get: phases for change_request (and bug/refactor size L), a flat list otherwise.
func planAnswer(prompt string) map[string]any {
	m := promptTypeSize.FindStringSubmatch(prompt)
	if m == nil {
		panic("plan prompt without type and size:\n" + prompt)
	}
	typ, size := domain.RequestType(m[1]), domain.RequestSize(m[2])
	flow, err := domain.FlowFor(typ)
	if err != nil {
		panic(err)
	}
	specs := planSpecs(typ)
	taskOf := func(sp planTaskSpec, at int) map[string]any {
		t := map[string]any{"title": sp.title, "description": sp.title + ": done when the checks pass", "task_type": "task", "irreversible": sp.irreversible}
		if sp.labels != nil {
			t["labels"] = sp.labels
		}
		if sp.afterPrev && at > 0 {
			t["depends_on_indices"] = []int{at - 1}
		}
		return t
	}
	list := func(from, to int) []any {
		out := []any{}
		for i := from; i < to; i++ {
			out = append(out, taskOf(specs[i], i-from))
		}
		return out
	}
	doc := map[string]any{"title": "Plan for " + string(typ), "summary": "Stub plan for the " + string(typ) + " scenario"}
	if domain.PlanShapeFor(flow, size) == domain.ShapePlanPhases {
		cut := min(2, len(specs))
		doc["phases"] = []any{
			map[string]any{"title": "Phase 1", "description": "first slice", "tasks": list(0, cut)},
			map[string]any{"title": "Phase 2", "description": "second slice", "depends_on_phase_indices": []int{0}, "tasks": list(cut, len(specs))},
		}
		return doc
	}
	doc["tasks"] = list(0, len(specs))
	return doc
}

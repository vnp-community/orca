package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// Limits of a document sent to tdd-agent (testspec_start refuses more; we say so first instead of cutting).
const (
	testSpecMaxFiles     = 40
	testSpecMaxFileBytes = 256 * 1024
	testSpecMaxTotal     = 2 * 1024 * 1024
)

var testSpecFileName = regexp.MustCompile(`^(?:tasks\.md|proposal\.md|design\.md|specs/[A-Za-z0-9._-]+/spec\.md)$`)

// TestSpecContextBuilder assembles what the dev server's tdd-agent reads besides its knowledge graph: the Solution or the
// Plan (an OpenSpec change) of the Request. Which one is the dev server's choice (its context_mode); this only builds it.
type TestSpecContextBuilder struct {
	Solutions SolutionLister
	Changes   OpenSpecChangeReader
	Redactor  TestSpecRedactor
}

// SolutionLister is the slice of SolutionStore the builder reads; OpenSpecChangeReader (get_execution_summary.go) the slice of OpenSpecChangeRepository.
type SolutionLister interface {
	ListByRequest(ctx context.Context, f SolutionListFilter) ([]domain.Solution, error)
}

// TestSpecContext is the document of one mode. Skip is set (and the rest empty) when the Request has nothing to send.
type TestSpecContext struct {
	Kind     string
	Ref      string
	Document json.RawMessage
	Files    map[string]string
	Skip     string
	SkipText string
}

// TestSpecRef is the short reference tdd-agent puts in the codes of the nodes it adds: REQ-<number>.
func TestSpecRefOf(req domain.Request) string { return fmt.Sprintf("REQ-%d", req.Number) }

func (b *TestSpecContextBuilder) Build(ctx context.Context, req domain.Request, mode string) (TestSpecContext, error) {
	switch mode {
	case "solution":
		return b.solution(ctx, req)
	case "plan":
		return b.plan(ctx, req)
	}
	return TestSpecContext{Skip: domain.TestSpecModeNone, SkipText: "the dev server's tdd-agent has context_mode " + mode}, nil
}

func (b *TestSpecContextBuilder) clean(ctx context.Context, s string) string {
	if b.Redactor == nil {
		return s
	}
	return b.Redactor.Apply(ctx, s)
}

// solution sends the Solution as Orca stores it (the shape tdd-agent reads: SOLUTION 14.3 dạng 1), plus the acceptance criteria of
// the Request. A Solution that is not approved is sent too; tdd-agent notes that and keeps its assumptions as proposals.
func (b *TestSpecContextBuilder) solution(ctx context.Context, req domain.Request) (TestSpecContext, error) {
	list, err := b.Solutions.ListByRequest(ctx, SolutionListFilter{RequestID: req.ID, Kind: domain.SolutionKindSolution})
	if err != nil {
		return TestSpecContext{}, err
	}
	sol, ok := pickSolution(list)
	if !ok || len(sol.OptionsJSON) == 0 {
		return TestSpecContext{Skip: domain.TestSpecNoSolution, SkipText: "the Request has no Solution to send"}, nil
	}
	type ac struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	var criteria []ac
	if c, err := domain.ContentFromRequest(req); err == nil {
		for _, it := range c.AcceptanceCriteria.Items {
			if it.Status == domain.ACStatusActive {
				criteria = append(criteria, ac{ID: it.ID, Text: b.clean(ctx, it.Text)})
			}
		}
	}
	doc := map[string]any{
		"id": sol.ID, "status": string(sol.Status),
		"options":             json.RawMessage(b.clean(ctx, string(sol.OptionsJSON))),
		"acceptance_criteria": criteria,
		"request":             map[string]string{"ref": TestSpecRefOf(req), "title": b.clean(ctx, req.Title)},
	}
	if sol.ChosenOption != nil {
		doc["chosenOption"] = *sol.ChosenOption
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return TestSpecContext{}, fmt.Errorf("test specs: marshal solution: %w", err)
	}
	if len(raw) > testSpecMaxFileBytes {
		return TestSpecContext{Skip: domain.TestSpecTooLarge, SkipText: fmt.Sprintf("the Solution is %d bytes (at most %d)", len(raw), testSpecMaxFileBytes)}, nil
	}
	return TestSpecContext{Kind: "solution", Ref: TestSpecRefOf(req), Document: raw}, nil
}

// pickSolution prefers an approved Solution, then the newest one that is still open; superseded and rejected ones count only
// when nothing else is left.
func pickSolution(list []domain.Solution) (domain.Solution, bool) {
	if len(list) == 0 {
		return domain.Solution{}, false
	}
	rank := func(s domain.Solution) int {
		switch s.Status {
		case domain.SolutionStatusApproved:
			return 3
		case domain.SolutionStatusProposed, domain.SolutionStatusDraft:
			return 2
		}
		return 1
	}
	sorted := append([]domain.Solution(nil), list...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if ri, rj := rank(sorted[i]), rank(sorted[j]); ri != rj {
			return ri > rj
		}
		if sorted[i].Seq != sorted[j].Seq {
			return sorted[i].Seq > sorted[j].Seq
		}
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})
	return sorted[0], true
}

// plan sends the files of the Request's OpenSpec change from the database copy, so it works after the worktree is gone.
func (b *TestSpecContextBuilder) plan(ctx context.Context, req domain.Request) (TestSpecContext, error) {
	change, found, err := b.Changes.GetByRequest(ctx, req.ID)
	if err != nil {
		return TestSpecContext{}, err
	}
	if !found || len(change.Bundle) == 0 {
		return TestSpecContext{Skip: domain.TestSpecNoPlanBundle, SkipText: "the Request has no OpenSpec change (it does not run on the openspec engine)"}, nil
	}
	files := map[string]string{}
	total := 0
	for name, text := range change.Bundle {
		if !testSpecFileName.MatchString(name) {
			continue
		}
		text = b.clean(ctx, text)
		if len(text) > testSpecMaxFileBytes {
			return TestSpecContext{Skip: domain.TestSpecTooLarge, SkipText: fmt.Sprintf("%s is %d bytes (at most %d)", name, len(text), testSpecMaxFileBytes)}, nil
		}
		total += len(text)
		files[name] = text
	}
	if _, ok := files["tasks.md"]; !ok {
		return TestSpecContext{Skip: domain.TestSpecNoPlanBundle, SkipText: "the OpenSpec change has no tasks.md"}, nil
	}
	if len(files) > testSpecMaxFiles || total > testSpecMaxTotal {
		return TestSpecContext{Skip: domain.TestSpecTooLarge, SkipText: fmt.Sprintf("%d files, %d bytes (at most %d files, %d bytes)", len(files), total, testSpecMaxFiles, testSpecMaxTotal)}, nil
	}
	return TestSpecContext{Kind: "plan", Ref: TestSpecRefOf(req), Files: files}, nil
}

// testSpecReasonFromAgent maps what tdd-agent answered to a status and reason code of the row.
func testSpecReasonFromAgent(code string) (domain.TestSpecStatus, string) {
	switch {
	case code == "LLM_NOT_CONFIGURED" || code == "LLM_KEY_MISSING" || code == "LLM_MODEL_MISSING":
		return domain.TestSpecBlocked, domain.TestSpecLLMMissing
	case code == "WRITE_DISABLED":
		return domain.TestSpecBlocked, domain.TestSpecWriteDisabled
	case code == "TESTSPEC_DISABLED":
		return domain.TestSpecSkipped, domain.TestSpecDisabledOnDev
	case code == "TOOL_NOT_FOUND" || code == "MCP_TOOL_NOT_ALLOWED" || code == "MCP_SERVER_NOT_ALLOWED" || code == "MCP_SERVER_NOT_INSTALLED":
		return domain.TestSpecBlocked, domain.TestSpecToolMissing
	case strings.HasPrefix(code, "TESTSPEC_CONTEXT_"), strings.HasPrefix(code, "TESTSPEC_KG_MCP"):
		return domain.TestSpecBlocked, code
	}
	return domain.TestSpecFailed, code
}

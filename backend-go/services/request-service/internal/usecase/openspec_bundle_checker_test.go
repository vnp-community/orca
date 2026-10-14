package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type recordingExec struct {
	res  ExecResult
	err  error
	seen []ExecInput
}

func (r *recordingExec) Exec(_ context.Context, _ AnalysisConnection, in ExecInput) (ExecResult, error) {
	r.seen = append(r.seen, in)
	return r.res, r.err
}

const validJSON = `{"items":[{"id":"req-900","type":"change","valid":true,"issues":[]}],"summary":{"totals":{"items":1,"passed":1,"failed":0}}}`

var goodRepoFiles = []string{
	"backend/services/platform/system-setting/internal/config/config.go",
	"backend/services/platform/system-setting/internal/domain/ai_config.go",
	"backend/services/platform/system-setting/internal/handler/router.go",
	"backend/services/platform/system-setting/internal/usecase/ai/doc.go",
	"backend/services/platform/system-setting/internal/repository/mysql/other.go",
	"backend/shared/pkg/netguard/netguard.go",
	"docs/SRS.md",
}

func goodBundleKit(t *testing.T, validate ExecResult) (*OpenSpecBundleChecker, *fakeWorkspace, BundleCheckInput, *recordingExec) {
	t.Helper()
	root := filepath.Join("..", "domain", "testdata", "openspec_bundle", "good")
	md, err := os.ReadFile(filepath.Join(root, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join(root, "specs", "ai-foundation", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	ws := newFakeWorkspace()
	ws.seedRepo(goodRepoFiles...)
	ws.files["openspec/changes/req-900/tasks.md"] = md
	ws.files["openspec/changes/req-900/specs/ai-foundation/spec.md"] = spec
	proposal, v := domain.ParsePlanRegion(string(md), "REQ-900")
	if len(v) != 0 {
		t.Fatal(v)
	}
	var acs []string
	for _, id := range []string{"AC-AI-01", "AC-AI-02", "AC-AI-03", "AC-AI-04", "AC-AI-05", "AC-AI-06", "AC-AI-07"} {
		acs = append(acs, `{"id":"`+id+`","text":"x","status":"active"}`)
	}
	req := domain.Request{ID: "r", ProjectID: "p", Number: 900, Size: domain.RequestSizeL, AcceptanceCriteriaJSON: []byte("[" + strings.Join(acs, ",") + "]")}
	exec := &recordingExec{res: validate}
	checker := NewOpenSpecBundleChecker(exec, fakeConns{conn: AnalysisConnection{ConnectionID: "c", RepoPath: "/srv/repo"}}, domain.DefaultPlanLimits)
	in := BundleCheckInput{Request: req, Result: ProposalResult{ChangeID: "req-900", Ref: "REQ-900", Workspace: ws.ws}, Proposal: proposal, Workspace: ws}
	return checker, ws, in, exec
}

func TestOpenSpecBundleChecker_AcceptsTheGoodBundleAndValidatesInTheWorktree(t *testing.T) {
	c, ws, in, exec := goodBundleKit(t, ExecResult{Stdout: validJSON})
	findings, err := c.Check(context.Background(), in)
	if err != nil || len(findings) != 0 {
		t.Fatalf("%v %v", findings, err)
	}
	if len(exec.seen) != 1 || exec.seen[0].Binary != "openspec" || strings.Join(exec.seen[0].Args, " ") != "validate req-900 --strict --json" || exec.seen[0].Cwd != ws.ws.Path {
		t.Fatalf("validate must run in the worktree: %+v", exec.seen)
	}
}

func TestOpenSpecBundleChecker_ReportsStructureFindingsWithWhere(t *testing.T) {
	c, ws, in, _ := goodBundleKit(t, ExecResult{Stdout: validJSON})
	delete(ws.files, "openspec/changes/req-900/specs/ai-foundation/spec.md")
	findings, err := c.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, f := range findings {
		codes = append(codes, f.Code)
	}
	if !strings.Contains(strings.Join(codes, ","), domain.BundleSpecsMissing) || !strings.Contains(strings.Join(codes, ","), domain.BundleACNotCovered) {
		t.Fatalf("a change without spec files must be refused: %v", findings)
	}
	for _, f := range findings {
		if f.Code == domain.BundleACNotCovered && !strings.HasPrefix(f.Message, "AC-AI-") {
			t.Fatalf("the finding must name the criterion first: %q", f.Message)
		}
	}
}

func TestOpenSpecBundleChecker_AnInvalidChangeBecomesFindings(t *testing.T) {
	issues := ""
	for i := 0; i < 12; i++ {
		issues += `{"level":"ERROR","path":"x/spec.md","message":"problem ` + strings.Repeat("é", 400) + `"},`
	}
	out := `{"items":[{"id":"req-900","valid":false,"issues":[` + strings.TrimSuffix(issues, ",") + `]}]}`
	c, _, in, _ := goodBundleKit(t, ExecResult{Stdout: out, ExitCode: 1})
	findings, err := c.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 8 {
		t.Fatalf("the note stays short: %d findings", len(findings))
	}
	for _, f := range findings {
		if f.Code != domain.BundleValidation || len([]rune(f.Message)) > 301 || !strings.Contains(f.Message, "x/spec.md") {
			t.Fatalf("%+v", f)
		}
	}
}

func TestOpenSpecBundleChecker_InvalidWithoutIssuesStillFails(t *testing.T) {
	c, _, in, _ := goodBundleKit(t, ExecResult{Stdout: `{"items":[{"id":"req-900","valid":false,"issues":[]}]}`, ExitCode: 1})
	findings, err := c.Check(context.Background(), in)
	if err != nil || len(findings) != 1 || findings[0].Code != domain.BundleValidation {
		t.Fatalf("%v %v", findings, err)
	}
}

func TestOpenSpecBundleChecker_ARefusalWithoutJSONIsAFinding_ButASilentSuccessIsNot(t *testing.T) {
	c, _, in, _ := goodBundleKit(t, ExecResult{Stdout: "Unknown item 'req-900'\nDid you mean: x?", ExitCode: 1})
	findings, err := c.Check(context.Background(), in)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0].Message, "Unknown item") {
		t.Fatalf("%v %v", findings, err)
	}
	c, _, in, _ = goodBundleKit(t, ExecResult{Stdout: "all good", ExitCode: 0})
	if _, err := c.Check(context.Background(), in); err == nil {
		t.Fatal("exit 0 with no readable result is an unverified bundle, not a clean one")
	}
}

func TestOpenSpecBundleChecker_ACheckThatCouldNotRunIsAnErrorNotAFinding(t *testing.T) {
	for name, res := range map[string]ExecResult{"timeout": {TimedOut: true, ExitCode: -1}, "no binary": {SpawnError: "ENOENT", ExitCode: -1}} {
		c, _, in, _ := goodBundleKit(t, res)
		if _, err := c.Check(context.Background(), in); err == nil {
			t.Fatalf("%s must be an error", name)
		}
	}
	c, _, in, _ := goodBundleKit(t, ExecResult{Stdout: validJSON})
	in.Workspace = nil
	if _, err := c.Check(context.Background(), in); err == nil {
		t.Fatal("no workspace, no check")
	}
}

func TestOpenSpecBundleChecker_UsesOnlyActiveCriteria(t *testing.T) {
	c, _, in, _ := goodBundleKit(t, ExecResult{Stdout: validJSON})
	in.Request.AcceptanceCriteriaJSON = []byte(`[{"id":"AC-AI-01","text":"x","status":"active"},{"id":"AC-AI-99","text":"retired","status":"removed"}]`)
	findings, err := c.Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if strings.Contains(f.Message, "AC-AI-99") {
			t.Fatalf("a retired criterion must not be demanded: %v", findings)
		}
	}
}

func TestWorkspaceProbe(t *testing.T) {
	ws := newFakeWorkspace()
	ws.seedRepo("a/b/c.go")
	p := workspaceProbe{ctx: context.Background(), ws: ws}
	if e, d, _ := p.Probe("a/b/c.go"); !e || d {
		t.Fatal("file")
	}
	if e, d, _ := p.Probe("a/b"); !e || !d {
		t.Fatal("dir")
	}
	if e, _, _ := p.Probe("a/x"); e {
		t.Fatal("missing")
	}
}

package usecase

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TestScriptsView is what the Execution tab reads: whether the lane is on, the current attempt and the earlier ones.
type TestScriptsView struct {
	RequestID string
	Enabled   bool
	Current   *domain.TestScriptRun
	History   []domain.TestScriptRun
}

// GetTestScripts reads the state of the test-script lane. It reads only the lane's own table.
type GetTestScripts struct {
	Requests RequestReader
	Runs     TestScriptRunRepository
	Settings TestScriptSettings
}

func (uc *GetTestScripts) Execute(ctx context.Context, requestID string) (TestScriptsView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return TestScriptsView{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return TestScriptsView{}, domain.ErrRequestNotFound(requestID)
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return TestScriptsView{}, err
	}
	view := TestScriptsView{RequestID: req.ID, Enabled: uc.Settings.Enabled(ctx, req.ProjectID)}
	all, err := uc.Runs.List(ctx, req.ID)
	if err != nil {
		return TestScriptsView{}, err
	}
	for i := range all {
		if all[i].SupersededAt == nil && view.Current == nil {
			cur := all[i]
			view.Current = &cur
			continue
		}
		view.History = append(view.History, all[i])
	}
	return view, nil
}

// GetTestScriptReport reads one page of the tsa-report.md of the finished job, from the dev server (it lives in the worktree).
type GetTestScriptReport struct {
	Requests RequestReader
	Runs     TestScriptRunRepository
	Agent    TestScriptAgent
	Resolver AnalysisConnectionResolver
}

func (uc *GetTestScriptReport) Execute(ctx context.Context, requestID string, offset int) (TestScriptText, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return TestScriptText{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return TestScriptText{}, domain.ErrRequestNotFound(requestID)
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return TestScriptText{}, err
	}
	run, found, err := uc.Runs.Current(ctx, req.ID)
	if err != nil {
		return TestScriptText{}, err
	}
	if !found || run.TSAJobID == "" || (run.State != domain.TestScriptCompleted && run.State != domain.TestScriptNeedsReview) {
		return TestScriptText{}, domain.ErrTestScriptNotFound(req.ID)
	}
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return TestScriptText{}, err
	}
	if offset < 0 {
		offset = 0
	}
	return uc.Agent.Report(ctx, conn, run.TSAJobID, offset)
}

// GetTestScriptFile reads one page of an output of the finished job: a generated script, assertion-manifest.json or tsa-report.md.
// Orca checks the path against what the job reported before asking (the agent checks again): nothing else of the worktree is read.
type GetTestScriptFile struct {
	Requests RequestReader
	Runs     TestScriptRunRepository
	Agent    TestScriptAgent
	Resolver AnalysisConnectionResolver
}

func (uc *GetTestScriptFile) Execute(ctx context.Context, requestID, path string, offset int) (TestScriptText, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return TestScriptText{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return TestScriptText{}, domain.ErrRequestNotFound(requestID)
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return TestScriptText{}, err
	}
	run, found, err := uc.Runs.Current(ctx, req.ID)
	if err != nil {
		return TestScriptText{}, err
	}
	if !found || run.TSAJobID == "" || (run.State != domain.TestScriptCompleted && run.State != domain.TestScriptNeedsReview) {
		return TestScriptText{}, domain.ErrTestScriptNotFound(req.ID)
	}
	if !testScriptOutput(run.Summary, path) {
		return TestScriptText{}, domain.ErrTestScriptFileInvalid(path)
	}
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return TestScriptText{}, err
	}
	if offset < 0 {
		offset = 0
	}
	return uc.Agent.File(ctx, conn, run.TSAJobID, path, offset)
}

// testScriptOutput is true when path is a file the job reported writing (summary.files, manifest, report).
func testScriptOutput(summary json.RawMessage, path string) bool {
	if path == "" || strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		return false
	}
	var s domain.TestScriptSummary
	if json.Unmarshal(summary, &s) != nil {
		return false
	}
	if path == s.Manifest || path == s.Report {
		return true
	}
	for _, f := range s.Files {
		if f.Path == path {
			return true
		}
	}
	return false
}

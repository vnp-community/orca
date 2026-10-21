package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TestSpecsView is what the Execution tab reads: whether the lane is on for the project, the current attempt and the earlier ones.
type TestSpecsView struct {
	RequestID string
	Enabled   bool
	Current   *domain.TestSpecRun
	History   []domain.TestSpecRun
	// Join is what execution does about the lane right now; nil when the gate is not wired.
	Join *TestSpecJoinView
}

// GetTestSpecs reads the state of the test-spec lane. It reads only the lane's own table.
type GetTestSpecs struct {
	Requests RequestReader
	Runs     TestSpecRunRepository
	Settings TestSpecSettings
	Join     *TestSpecJoin
}

func (uc *GetTestSpecs) Execute(ctx context.Context, requestID string) (TestSpecsView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return TestSpecsView{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return TestSpecsView{}, domain.ErrRequestNotFound(requestID)
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return TestSpecsView{}, err
	}
	view := TestSpecsView{RequestID: req.ID, Enabled: uc.Settings.Enabled(ctx, req.ProjectID)}
	all, err := uc.Runs.List(ctx, req.ID)
	if err != nil {
		return TestSpecsView{}, err
	}
	for i := range all {
		if all[i].SupersededAt == nil && view.Current == nil {
			cur := all[i]
			view.Current = &cur
			continue
		}
		view.History = append(view.History, all[i])
	}
	if uc.Join != nil && view.Enabled {
		j, err := uc.Join.View(ctx, req)
		if err != nil {
			return TestSpecsView{}, err
		}
		view.Join = &j
	}
	return view, nil
}

// GetTestSpecDocument reads one page of a document the finished run wrote, from the dev server (the documents live in the repository).
type GetTestSpecDocument struct {
	Requests RequestReader
	Runs     TestSpecRunRepository
	Agent    TestSpecAgent
	Resolver AnalysisConnectionResolver
}

func (uc *GetTestSpecDocument) Execute(ctx context.Context, requestID, document string, offset int) (TestSpecText, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return TestSpecText{}, domain.ErrRequestTenantRequired()
	}
	if !domain.ValidTestSpecDocument(document) {
		return TestSpecText{}, domain.ErrTestSpecDocumentInvalid(document)
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return TestSpecText{}, domain.ErrRequestNotFound(requestID)
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return TestSpecText{}, err
	}
	run, found, err := uc.Runs.Current(ctx, req.ID)
	if err != nil {
		return TestSpecText{}, err
	}
	if !found || run.Feature == "" || run.State != domain.TestSpecCompleted {
		return TestSpecText{}, domain.ErrTestSpecNotFound(req.ID)
	}
	conn, err := uc.Resolver.ResolveForProject(ctx, req.ProjectID)
	if err != nil {
		return TestSpecText{}, err
	}
	if offset < 0 {
		offset = 0
	}
	return uc.Agent.Document(ctx, conn, run.Feature, document, offset)
}

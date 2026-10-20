package usecase

import (
	"context"
	"path"
	"strings"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanBundleFile is one file of the authored change as it was committed.
type PlanBundleFile struct {
	Path    string
	Content string
	// Omitted is true when the file was too large to keep; Content then holds the size and hash marker.
	Omitted bool
}

// GetPlanBundleFile reads one file of a Request's authored OpenSpec change from the database copy, so it works after the
// worktree is gone. Only paths inside the change are answered.
type GetPlanBundleFile struct {
	requests  RequestRepository
	changes   OpenSpecChangeRepository
	runs      AnalysisRunStore
	revisions OpenSpecChangeRevisionRepository
}

func NewGetPlanBundleFile(requests RequestRepository, changes OpenSpecChangeRepository) *GetPlanBundleFile {
	return &GetPlanBundleFile{requests: requests, changes: changes}
}

// WithRevisions lets a file of an older version be read; without it only the current change is known.
func (uc *GetPlanBundleFile) WithRevisions(runs AnalysisRunStore, revisions OpenSpecChangeRevisionRepository) *GetPlanBundleFile {
	uc.runs, uc.revisions = runs, revisions
	return uc
}

// Execute reads a file of the current change, or of the numbered version when revision is positive. The newest version is
// the current change, so a request for it reads the current row.
func (uc *GetPlanBundleFile) Execute(ctx context.Context, requestID, requested string, revision int) (PlanBundleFile, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return PlanBundleFile{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.requests.Get(ctx, requestID)
	if err != nil {
		return PlanBundleFile{}, err
	}
	change, found, err := uc.changes.GetByRequest(ctx, req.ID)
	if err != nil {
		return PlanBundleFile{}, err
	}
	if !found {
		return PlanBundleFile{}, domain.ErrPlanBundleNotFound(req.ID)
	}
	rel, ok := bundleRelPath(requested, change.ChangeID)
	if !ok {
		return PlanBundleFile{}, domain.ErrPlanBundlePathInvalid(requested)
	}
	bundle := change.Bundle
	if revision != 0 {
		if bundle, err = uc.versionBundle(ctx, req.ID, revision, bundle); err != nil {
			return PlanBundleFile{}, err
		}
	}
	content, found := bundle[rel]
	if !found {
		return PlanBundleFile{}, domain.ErrPlanBundleFileNotFound(rel)
	}
	if isOmittedMarker(content) {
		return PlanBundleFile{Path: rel, Content: content, Omitted: true}, nil
	}
	// A row stored before the masking existed holds the raw text: mask on read so no secret leaves the service.
	content, _ = RedactSecrets(content)
	return PlanBundleFile{Path: rel, Content: content}, nil
}

// versionBundle is the text of the numbered version: the current change for the newest one, the stored copy for the
// others. A version whose text was dropped (or never kept) answers with a code of its own, not "file not found".
func (uc *GetPlanBundleFile) versionBundle(ctx context.Context, requestID string, revision int, current map[string]string) (map[string]string, error) {
	if revision < 0 || uc.runs == nil {
		return nil, domain.ErrPlanRevisionNotFound(revision)
	}
	runs, err := uc.runs.ListSucceeded(ctx, requestID, domain.RunKindPlan)
	if err != nil {
		return nil, err
	}
	switch {
	case revision > len(runs):
		return nil, domain.ErrPlanRevisionNotFound(revision)
	case revision == len(runs):
		return current, nil
	case uc.revisions == nil:
		return nil, domain.ErrPlanRevisionTextPruned()
	}
	row, found, err := uc.revisions.Get(ctx, requestID, revision)
	if err != nil {
		return nil, err
	}
	if !found || !row.TextKept {
		return nil, domain.ErrPlanRevisionTextPruned()
	}
	return row.Bundle, nil
}

// bundleRelPath turns a requested path into one relative to the change. It accepts the relative form and the full form
// (openspec/changes/<id>/...), and refuses anything that could point elsewhere. Matching is exact, never by prefix of a
// directory that merely starts like the change's.
func bundleRelPath(requested, changeID string) (string, bool) {
	p := strings.ReplaceAll(requested, "\\", "/")
	if p == "" || strings.ContainsRune(p, 0) || len(p) > 512 || path.IsAbs(p) {
		return "", false
	}
	c := path.Clean(p)
	if c == "." || c == ".." || strings.HasPrefix(c, "../") {
		return "", false
	}
	if full := "openspec/changes/" + changeID + "/"; strings.HasPrefix(c, full) {
		c = strings.TrimPrefix(c, full)
	}
	if c == "" || strings.HasPrefix(c, "openspec/") {
		return "", false
	}
	return c, true
}

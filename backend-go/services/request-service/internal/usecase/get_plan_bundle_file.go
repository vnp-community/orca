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
	requests RequestRepository
	changes  OpenSpecChangeRepository
}

func NewGetPlanBundleFile(requests RequestRepository, changes OpenSpecChangeRepository) *GetPlanBundleFile {
	return &GetPlanBundleFile{requests: requests, changes: changes}
}

func (uc *GetPlanBundleFile) Execute(ctx context.Context, requestID, requested string) (PlanBundleFile, error) {
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
	content, found := change.Bundle[rel]
	if !found {
		return PlanBundleFile{}, domain.ErrPlanBundleFileNotFound(rel)
	}
	return PlanBundleFile{Path: rel, Content: content, Omitted: strings.HasPrefix(content, "<omitted size=")}, nil
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

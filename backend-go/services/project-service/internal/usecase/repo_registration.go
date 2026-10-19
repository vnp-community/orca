package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const (
	// CodeRepoAlreadyRegistered: the same repo (tenant, dev server, normalised
	// path) is already in a project. The frontend parses its fixed message.
	CodeRepoAlreadyRegistered = "PROJECT_REPO_ALREADY_REGISTERED"
	// CodeRepoRegistrationConflict: a concurrent request took the location and
	// then vanished before it could be named; the caller may retry.
	CodeRepoRegistrationConflict = "PROJECT_REPO_REGISTRATION_CONFLICT"
	// CodeImportNestedDuplicatePath: two candidates of one import are the same repo.
	CodeImportNestedDuplicatePath = "PROJECT_IMPORT_NESTED_DUPLICATE_PATH"
)

// repoLocationGuard decides whether a (dev server, path) may be registered
// as a new repo. The lookup reads real columns, so repos created before
// registration keys existed still occupy their location; the unique key index
// is only the backstop for two requests passing the lookup together.
type repoLocationGuard struct {
	repos   RepoRepository
	members MembershipRepository
}

// ensureFree returns nil when no other repo of the tenant sits at that
// location on that host. excludeRepoID is the repo being moved, if any.
func (g repoLocationGuard) ensureFree(ctx context.Context, tenantID, devServerID, url, excludeRepoID string) error {
	conflict, found, err := g.repos.FindRepoAtLocation(ctx, tenantID, devServerID, domain.NormalizeRepoLocation(url), excludeRepoID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REPO_LOOKUP_FAILED", "failed to check whether the repo is already registered", err)
	}
	if !found {
		return nil
	}
	return g.alreadyRegistered(ctx, url, devServerID, conflict)
}

// translateWriteError turns the unique-index rejection of a lost race into the
// same error a pre-check would have produced. Other errors return nil so the
// caller keeps its own wrapping.
func (g repoLocationGuard) translateWriteError(ctx context.Context, err error, tenantID, devServerID, url, excludeRepoID string) error {
	if !errors.Is(err, domain.ErrRepoAlreadyRegistered) {
		return nil
	}
	conflict, found, lookupErr := g.repos.FindRepoAtLocation(ctx, tenantID, devServerID, domain.NormalizeRepoLocation(url), excludeRepoID)
	if lookupErr != nil || !found {
		return apperrors.New(apperrors.KindAborted, CodeRepoRegistrationConflict, "this repository was registered by a concurrent request; retry", err)
	}
	return g.alreadyRegistered(ctx, url, devServerID, conflict)
}

func (g repoLocationGuard) alreadyRegistered(ctx context.Context, url, devServerID string, c domain.RepoConflict) error {
	host := devServerID
	if host == "" {
		host = "local"
	}
	if !g.callerMaySee(ctx, c) {
		// Same code, no name/id: the project is private to someone else.
		return apperrors.New(apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered,
			fmt.Sprintf("repo %s on dev server %s is already registered in another project", url, host), nil)
	}
	return apperrors.New(apperrors.KindAlreadyExists, CodeRepoAlreadyRegistered,
		fmt.Sprintf("repo %s on dev server %s is already registered in project \"%s\" (%s)", url, host, c.ProjectName, c.ProjectID), nil)
}

// callerMaySee keeps a private project's name and id from leaking to tenant
// members who could not list it anyway.
func (g repoLocationGuard) callerMaySee(ctx context.Context, c domain.RepoConflict) bool {
	if c.ProjectVisibility != domain.VisibilityPrivate {
		return true
	}
	if callerGlobalRole(ctx) == "admin" {
		return true
	}
	userID, ok := tenant.UserID(ctx)
	if !ok || g.members == nil {
		return false
	}
	_, err := g.members.GetMembership(ctx, c.ProjectID, userID)
	return err == nil
}

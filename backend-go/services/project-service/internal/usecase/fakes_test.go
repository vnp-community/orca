package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// Shared in-memory fakes used across this package's *_test.go files — the
// "test against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
// Kept in one file so every usecase test builds against an identical fake
// shape instead of each test file redefining its own copy.

// fakeProjectRepository is an in-memory ProjectRepository.
type fakeProjectRepository struct {
	projects map[string]domain.Project
	members  []domain.ProjectMember

	createErr        error
	updateDevServErr error
	updateErr        error
	deleteErr        error
	getMembershipErr error
}

func newFakeProjectRepository() *fakeProjectRepository {
	return &fakeProjectRepository{projects: map[string]domain.Project{}}
}

func (f *fakeProjectRepository) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	if f.createErr != nil {
		return domain.Project{}, f.createErr
	}
	f.projects[p.ID] = p
	return p, nil
}

func (f *fakeProjectRepository) Get(ctx context.Context, tenantID, id string) (domain.Project, error) {
	p, ok := f.projects[id]
	if !ok || p.TenantID != tenantID {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	return p, nil
}

func (f *fakeProjectRepository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	var out []domain.Project
	for _, p := range f.projects {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, "", nil
}

func (f *fakeProjectRepository) AddMember(ctx context.Context, m domain.ProjectMember) error {
	f.members = append(f.members, m)
	return nil
}

func (f *fakeProjectRepository) UpdateDevServerID(ctx context.Context, tenantID, projectID, devServerID string) (domain.Project, error) {
	if f.updateDevServErr != nil {
		return domain.Project{}, f.updateDevServErr
	}
	p, ok := f.projects[projectID]
	if !ok || p.TenantID != tenantID {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	p.DevServerID = devServerID
	f.projects[projectID] = p
	return p, nil
}

func (f *fakeProjectRepository) UpdateProject(ctx context.Context, tenantID, projectID string, patch domain.ProjectUpdatePatch) (domain.Project, error) {
	if f.updateErr != nil {
		return domain.Project{}, f.updateErr
	}
	p, ok := f.projects[projectID]
	if !ok || p.TenantID != tenantID {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	if patch.Name != "" {
		p.Name = patch.Name
	}
	if patch.Description != "" {
		p.Description = patch.Description
	}
	if patch.DefaultBranch != "" {
		p.DefaultBranch = patch.DefaultBranch
	}
	if patch.Visibility != "" {
		p.Visibility = patch.Visibility
	}
	f.projects[projectID] = p
	return p, nil
}

func (f *fakeProjectRepository) DeleteProject(ctx context.Context, tenantID, projectID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	p, ok := f.projects[projectID]
	if !ok || p.TenantID != tenantID {
		return domain.ErrProjectNotFound
	}
	delete(f.projects, projectID)
	return nil
}

// GetMembership implements usecase.ProjectRepository.GetMembership (and,
// structurally, usecase.MembershipRepository) by scanning f.members —
// callers add fixture rows directly via `repo.members = append(...)`.
func (f *fakeProjectRepository) GetMembership(ctx context.Context, projectID, userID string) (domain.ProjectMember, error) {
	if f.getMembershipErr != nil {
		return domain.ProjectMember{}, f.getMembershipErr
	}
	for _, m := range f.members {
		if m.ProjectID == projectID && m.UserID == userID {
			return m, nil
		}
	}
	return domain.ProjectMember{}, domain.ErrMembershipNotFound
}

// fakeOPAClient is an in-memory usecase.OPAClient. decide, when set,
// computes the answer from the actual inputs the usecase under test passed
// in — letting tests exercise realistic owner-only/any-member/global-admin
// gating without re-deriving project.rego's logic ad hoc per test (the real
// mapping is proven correct at the policy layer by
// policy/orca-authz/project_test.rego, run via `opa test`). Falls back to
// the static allow/err fields when decide is nil. Every call is recorded in
// calls, so a test can assert exactly what the usecase asked OPA to decide.
type fakeOPAClient struct {
	decide func(callerProjectRole, callerGlobalRole, action string) bool

	allow bool
	err   error

	calls []opaDecisionCall
}

type opaDecisionCall struct {
	CallerProjectRole string
	CallerGlobalRole  string
	Action            string
}

func (f *fakeOPAClient) Decision(ctx context.Context, callerProjectRole, callerGlobalRole, action string) (bool, error) {
	f.calls = append(f.calls, opaDecisionCall{CallerProjectRole: callerProjectRole, CallerGlobalRole: callerGlobalRole, Action: action})
	if f.err != nil {
		return false, f.err
	}
	if f.decide != nil {
		return f.decide(callerProjectRole, callerGlobalRole, action), nil
	}
	return f.allow, nil
}

// projectRegoDecide mirrors policy/orca-authz/project.rego's action_roles
// table (owner_only -> {owner}, any_member -> {owner, member}) plus its
// global-admin override — used by fakeOPAClient.decide in authorization
// tests that need realistic role-gating instead of a single static
// allow/deny answer.
func projectRegoDecide(callerProjectRole, callerGlobalRole, action string) bool {
	if callerGlobalRole == "admin" {
		return true
	}
	switch action {
	case projectActionOwnerOnly:
		return callerProjectRole == "owner"
	case projectActionAnyMember:
		return callerProjectRole == "owner" || callerProjectRole == "member"
	default:
		return false
	}
}

// allowAllOPA is the fake used by tests that aren't exercising
// authorization itself — every Decision call succeeds, so pre-existing
// business-logic assertions (execution-checker guards, field-mask
// semantics, etc.) aren't coupled to authorization plumbing.
func allowAllOPA() *fakeOPAClient { return &fakeOPAClient{allow: true} }

// fakeExecutionChecker implements both WorkflowExecutionChecker and
// TaskExecutionChecker — the two ports have identical shape but are kept
// distinct types in usecase/ports.go so a fake for one doesn't couple to the
// other service's contract.
type fakeExecutionChecker struct {
	active bool
	err    error
}

func (f *fakeExecutionChecker) HasActiveExecutions(ctx context.Context, projectID string) (bool, error) {
	return f.active, f.err
}

// fakeRepoRepository is an in-memory RepoRepository.
type fakeRepoRepository struct {
	repos map[string]domain.Repo

	addErr     error
	listErr    error
	reorderErr error
	removeErr  error
	getErr     error
	updateErr  error
}

func newFakeRepoRepository() *fakeRepoRepository {
	return &fakeRepoRepository{repos: map[string]domain.Repo{}}
}

func (f *fakeRepoRepository) AddRepo(ctx context.Context, repo domain.Repo) (domain.Repo, error) {
	if f.addErr != nil {
		return domain.Repo{}, f.addErr
	}
	next := int32(0)
	for _, r := range f.repos {
		if r.ProjectID == repo.ProjectID && r.Position >= next {
			next = r.Position + 1
		}
	}
	repo.Position = next
	f.repos[repo.ID] = repo
	return repo, nil
}

func (f *fakeRepoRepository) ListRepos(ctx context.Context, projectID string) ([]domain.Repo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.Repo
	for _, r := range f.repos {
		if r.ProjectID == projectID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepoRepository) ReorderRepos(ctx context.Context, projectID string, idsInOrder []string) error {
	if f.reorderErr != nil {
		return f.reorderErr
	}
	for i, id := range idsInOrder {
		r, ok := f.repos[id]
		if !ok {
			return domain.ErrRepoNotFound
		}
		r.Position = int32(i)
		f.repos[id] = r
	}
	return nil
}

// GetRepo implements usecase.RepoRepository.GetRepo.
func (f *fakeRepoRepository) GetRepo(ctx context.Context, repoID string) (domain.Repo, error) {
	if f.getErr != nil {
		return domain.Repo{}, f.getErr
	}
	r, ok := f.repos[repoID]
	if !ok {
		return domain.Repo{}, domain.ErrRepoNotFound
	}
	return r, nil
}

// Update implements usecase.RepoRepository.Update.
func (f *fakeRepoRepository) Update(ctx context.Context, repo domain.Repo) (domain.Repo, error) {
	if f.updateErr != nil {
		return domain.Repo{}, f.updateErr
	}
	if _, ok := f.repos[repo.ID]; !ok {
		return domain.Repo{}, domain.ErrRepoNotFound
	}
	f.repos[repo.ID] = repo
	return repo, nil
}

func (f *fakeRepoRepository) RemoveRepo(ctx context.Context, repoID string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	if _, ok := f.repos[repoID]; !ok {
		return domain.ErrRepoNotFound
	}
	delete(f.repos, repoID)
	return nil
}

// fakeWorktreeRepository is an in-memory WorktreeRepository.
type fakeWorktreeRepository struct {
	worktrees map[string]domain.Worktree

	recordCreatedErr error
	recordRemovedErr error
	listErr          error
	setActivationErr error
	renameErr        error
}

func newFakeWorktreeRepository() *fakeWorktreeRepository {
	return &fakeWorktreeRepository{worktrees: map[string]domain.Worktree{}}
}

func (f *fakeWorktreeRepository) RecordWorktreeCreated(ctx context.Context, wt domain.Worktree) (domain.Worktree, error) {
	if f.recordCreatedErr != nil {
		return domain.Worktree{}, f.recordCreatedErr
	}
	f.worktrees[wt.ID] = wt
	return wt, nil
}

func (f *fakeWorktreeRepository) RecordWorktreeRemoved(ctx context.Context, worktreeID string) error {
	if f.recordRemovedErr != nil {
		return f.recordRemovedErr
	}
	if _, ok := f.worktrees[worktreeID]; !ok {
		return domain.ErrWorktreeNotFound
	}
	delete(f.worktrees, worktreeID)
	return nil
}

func (f *fakeWorktreeRepository) ListWorktrees(ctx context.Context, projectID string) ([]domain.Worktree, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.Worktree
	for _, wt := range f.worktrees {
		if wt.ProjectID == projectID {
			out = append(out, wt)
		}
	}
	return out, nil
}

func (f *fakeWorktreeRepository) SetWorktreeActivation(ctx context.Context, worktreeID string, active bool) (domain.Worktree, error) {
	if f.setActivationErr != nil {
		return domain.Worktree{}, f.setActivationErr
	}
	wt, ok := f.worktrees[worktreeID]
	if !ok {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	wt.Active = active
	f.worktrees[worktreeID] = wt
	return wt, nil
}

func (f *fakeWorktreeRepository) RenameWorktree(ctx context.Context, worktreeID, branch string) (domain.Worktree, error) {
	if f.renameErr != nil {
		return domain.Worktree{}, f.renameErr
	}
	wt, ok := f.worktrees[worktreeID]
	if !ok {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	wt.Branch = branch
	f.worktrees[worktreeID] = wt
	return wt, nil
}

// fakeProjectGroupRepository is an in-memory ProjectGroupRepository.
type fakeProjectGroupRepository struct {
	groups map[string]domain.ProjectGroup

	createErr error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeProjectGroupRepository() *fakeProjectGroupRepository {
	return &fakeProjectGroupRepository{groups: map[string]domain.ProjectGroup{}}
}

func (f *fakeProjectGroupRepository) CreateProjectGroup(ctx context.Context, g domain.ProjectGroup) (domain.ProjectGroup, error) {
	if f.createErr != nil {
		return domain.ProjectGroup{}, f.createErr
	}
	f.groups[g.ID] = g
	return g, nil
}

func (f *fakeProjectGroupRepository) GetProjectGroup(ctx context.Context, tenantID, id string) (domain.ProjectGroup, error) {
	g, ok := f.groups[id]
	if !ok || g.TenantID != tenantID {
		return domain.ProjectGroup{}, domain.ErrProjectGroupNotFound
	}
	return g, nil
}

func (f *fakeProjectGroupRepository) UpdateProjectGroup(ctx context.Context, tenantID, id, name string) (domain.ProjectGroup, error) {
	if f.updateErr != nil {
		return domain.ProjectGroup{}, f.updateErr
	}
	g, ok := f.groups[id]
	if !ok || g.TenantID != tenantID {
		return domain.ProjectGroup{}, domain.ErrProjectGroupNotFound
	}
	g.Name = name
	f.groups[id] = g
	return g, nil
}

func (f *fakeProjectGroupRepository) DeleteProjectGroup(ctx context.Context, tenantID, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	g, ok := f.groups[id]
	if !ok || g.TenantID != tenantID {
		return domain.ErrProjectGroupNotFound
	}
	delete(f.groups, id)
	return nil
}

func (f *fakeProjectGroupRepository) ListProjectGroups(ctx context.Context, tenantID string) ([]domain.ProjectGroup, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.ProjectGroup
	for _, g := range f.groups {
		if g.TenantID == tenantID {
			out = append(out, g)
		}
	}
	return out, nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func withTenantAndUser(ctx context.Context, tenantID, userID string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(ctx, tenantID), userID)
}

// assertAppError asserts err is an *apperrors.AppError with the given Kind
// and Code.
func assertAppError(t *testing.T, err error, kind apperrors.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Kind != kind {
		t.Errorf("expected Kind=%v, got %v", kind, ae.Kind)
	}
	if ae.Code != code {
		t.Errorf("expected code %q, got %q", code, ae.Code)
	}
}

// assertFailedPrecondition asserts err is a KindFailedPrecondition
// PROJECT_HAS_ACTIVE_WORKFLOWS AppError — the shared shape RebindDevServer
// and DeleteProject's guard both return.
func assertFailedPrecondition(t *testing.T, err error) {
	t.Helper()
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS")
}

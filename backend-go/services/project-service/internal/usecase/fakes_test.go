package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
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

	// countOwners is CountOwners's canned return value — tests set this
	// directly to exercise AssertNotLastOwnerRemoval's boundary without
	// needing to seed exactly-matching owner rows in members.
	countOwners     int
	countOwnersErr  error
	listMembersErr  error
	removeMemberErr error
	updateRoleErr   error

	// removeMemberCalled/updateMemberRoleCalled let tests assert the
	// ownerless guard fires BEFORE any repository mutation — see
	// TestRemoveMember_RejectsWhenWouldBeOwnerless's doc comment.
	removeMemberCalled     bool
	updateMemberRoleCalled bool
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

// ListMembers implements usecase.ProjectRepository.ListMembers.
func (f *fakeProjectRepository) ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	if f.listMembersErr != nil {
		return nil, f.listMembersErr
	}
	var out []domain.ProjectMember
	for _, m := range f.members {
		if m.ProjectID == projectID {
			out = append(out, m)
		}
	}
	return out, nil
}

// RemoveMember implements usecase.ProjectRepository.RemoveMember.
func (f *fakeProjectRepository) RemoveMember(ctx context.Context, projectID, userID string) error {
	f.removeMemberCalled = true
	if f.removeMemberErr != nil {
		return f.removeMemberErr
	}
	for i, m := range f.members {
		if m.ProjectID == projectID && m.UserID == userID {
			f.members = append(f.members[:i], f.members[i+1:]...)
			return nil
		}
	}
	return domain.ErrMembershipNotFound
}

// UpdateMemberRole implements usecase.ProjectRepository.UpdateMemberRole.
func (f *fakeProjectRepository) UpdateMemberRole(ctx context.Context, projectID, userID string, role domain.ProjectRole) (domain.ProjectMember, error) {
	f.updateMemberRoleCalled = true
	if f.updateRoleErr != nil {
		return domain.ProjectMember{}, f.updateRoleErr
	}
	for i, m := range f.members {
		if m.ProjectID == projectID && m.UserID == userID {
			f.members[i].Role = role
			return f.members[i], nil
		}
	}
	return domain.ProjectMember{}, domain.ErrMembershipNotFound
}

// CountOwners implements usecase.ProjectRepository.CountOwners — returns
// f.countOwners directly (a settable fixture value), NOT derived from
// f.members, so tests can exercise AssertNotLastOwnerRemoval's boundary
// precisely.
func (f *fakeProjectRepository) CountOwners(ctx context.Context, projectID string) (int, error) {
	if f.countOwnersErr != nil {
		return 0, f.countOwnersErr
	}
	return f.countOwners, nil
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

	upsertLeafErr error
	// upsertLeafCalled records whether UpsertLeafGroupForProject was
	// invoked — MoveProject's test plan asserts the usecase itself never
	// branches on "does a leaf group already exist" (that's the
	// repository's find-or-create job).
	upsertLeafCalled bool

	importNestedErr error
	// importNestedGroups/importNestedProjects are ImportNested's canned
	// return values — one pair per candidate, set by the test.
	importNestedGroups   []domain.ProjectGroup
	importNestedProjects []domain.Project
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

// fakeFolderWorkspaceRepository is an in-memory FolderWorkspaceRepository.
// repoPathExists is a single static flag (not per-path) — good enough for
// this package's tests, which never need two different paths to disagree
// on repo-collision within the same test.
type fakeFolderWorkspaceRepository struct {
	workspaces map[string]domain.FolderWorkspace

	repoPathExists bool

	findByPathCalls     int
	repoPathExistsCalls int
}

func newFakeFolderWorkspaceRepository() *fakeFolderWorkspaceRepository {
	return &fakeFolderWorkspaceRepository{workspaces: map[string]domain.FolderWorkspace{}}
}

func (f *fakeFolderWorkspaceRepository) Create(ctx context.Context, fw domain.FolderWorkspace) (domain.FolderWorkspace, error) {
	for _, existing := range f.workspaces {
		if existing.TenantID == fw.TenantID && existing.DevServerID == fw.DevServerID && existing.Path == fw.Path {
			return domain.FolderWorkspace{}, domain.ErrPathAlreadyRegistered
		}
	}
	f.workspaces[fw.ID] = fw
	return fw, nil
}

func (f *fakeFolderWorkspaceRepository) Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error) {
	fw, ok := f.workspaces[id]
	if !ok {
		return domain.FolderWorkspace{}, domain.ErrFolderWorkspaceNotFound
	}
	fw.Name = name
	f.workspaces[id] = fw
	return fw, nil
}

func (f *fakeFolderWorkspaceRepository) Delete(ctx context.Context, id string) error {
	if _, ok := f.workspaces[id]; !ok {
		return domain.ErrFolderWorkspaceNotFound
	}
	delete(f.workspaces, id)
	return nil
}

func (f *fakeFolderWorkspaceRepository) ListByTenant(ctx context.Context, tenantID string) ([]domain.FolderWorkspace, error) {
	var out []domain.FolderWorkspace
	for _, fw := range f.workspaces {
		if fw.TenantID == tenantID {
			out = append(out, fw)
		}
	}
	return out, nil
}

func (f *fakeFolderWorkspaceRepository) FindByPath(ctx context.Context, tenantID, devServerID, path string) (*domain.FolderWorkspace, error) {
	f.findByPathCalls++
	for _, fw := range f.workspaces {
		if fw.TenantID == tenantID && fw.DevServerID == devServerID && fw.Path == path {
			found := fw
			return &found, nil
		}
	}
	return nil, nil
}

func (f *fakeFolderWorkspaceRepository) RepoPathExists(ctx context.Context, tenantID, devServerID, path string) (bool, error) {
	f.repoPathExistsCalls++
	return f.repoPathExists, nil
}

func (f *fakeFolderWorkspaceRepository) Get(ctx context.Context, id string) (*domain.FolderWorkspace, error) {
	fw, ok := f.workspaces[id]
	if !ok {
		return nil, nil
	}
	return &fw, nil
}

// UpsertLeafGroupForProject implements
// usecase.ProjectGroupRepository.UpsertLeafGroupForProject.
func (f *fakeProjectGroupRepository) UpsertLeafGroupForProject(ctx context.Context, tenantID, projectID, projectName, targetParentGroupID string) (domain.ProjectGroup, error) {
	f.upsertLeafCalled = true
	if f.upsertLeafErr != nil {
		return domain.ProjectGroup{}, f.upsertLeafErr
	}
	for _, g := range f.groups {
		if g.ProjectID == projectID {
			g.ParentGroupID = targetParentGroupID
			f.groups[g.ID] = g
			return g, nil
		}
	}
	g := domain.ProjectGroup{ID: "leaf-" + projectID, TenantID: tenantID, Name: projectName, ParentGroupID: targetParentGroupID, ProjectID: projectID}
	f.groups[g.ID] = g
	return g, nil
}

// ImportNested implements usecase.ProjectGroupRepository.ImportNested.
func (f *fakeProjectGroupRepository) ImportNested(ctx context.Context, tenantID, createdBy, devServerID, parentGroupID string, candidates []domain.NestedRepoCandidate) ([]domain.ProjectGroup, []domain.Project, error) {
	if f.importNestedErr != nil {
		return nil, nil, f.importNestedErr
	}
	return f.importNestedGroups, f.importNestedProjects, nil
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

// fakeDevServerRelay is an in-memory usecase.DevServerRelay — records every
// call so tests can assert exact (devServerID, repoPath, worktreeID)/
// (connectionID, method, paramsJSON) arguments (ScanNested/
// SetupExistingFolder's test plans both need this).
type fakeDevServerRelay struct {
	createConnectionErr error
	connectionID        string

	relayErr    error
	relayResult []byte

	createConnectionCalls []fakeCreateConnectionCall
	relayCalls            []fakeRelayCall
}

type fakeCreateConnectionCall struct {
	DevServerID string
	RepoPath    string
	WorktreeID  string
}

type fakeRelayCall struct {
	ConnectionID string
	Method       string
	ParamsJSON   []byte
}

func (f *fakeDevServerRelay) CreateConnection(ctx context.Context, devServerID, repoPath, worktreeID string) (string, error) {
	f.createConnectionCalls = append(f.createConnectionCalls, fakeCreateConnectionCall{DevServerID: devServerID, RepoPath: repoPath, WorktreeID: worktreeID})
	if f.createConnectionErr != nil {
		return "", f.createConnectionErr
	}
	connID := f.connectionID
	if connID == "" {
		connID = "conn-1"
	}
	return connID, nil
}

func (f *fakeDevServerRelay) Relay(ctx context.Context, connectionID, method string, paramsJSON []byte) ([]byte, error) {
	f.relayCalls = append(f.relayCalls, fakeRelayCall{ConnectionID: connectionID, Method: method, ParamsJSON: paramsJSON})
	if f.relayErr != nil {
		return nil, f.relayErr
	}
	return f.relayResult, nil
}

// fakeHostSetupRepository is an in-memory usecase.HostSetupRepository.
type fakeHostSetupRepository struct {
	setups map[string]domain.HostSetup

	createErr    error
	getErr       error
	listErr      error
	updateErr    error
	deleteErr    error
	setStatusErr error
	completeErr  error

	// createCalled/projectsCreateCalled-style flags let tests assert a
	// mutation never ran — see TestCreateHostSetup_ValidatesDevServerID.
	createCalled bool
}

func newFakeHostSetupRepository() *fakeHostSetupRepository {
	return &fakeHostSetupRepository{setups: map[string]domain.HostSetup{}}
}

func (f *fakeHostSetupRepository) Create(ctx context.Context, s domain.HostSetup) (domain.HostSetup, error) {
	f.createCalled = true
	if f.createErr != nil {
		return domain.HostSetup{}, f.createErr
	}
	f.setups[s.ID] = s
	return s, nil
}

func (f *fakeHostSetupRepository) Get(ctx context.Context, tenantID, id string) (domain.HostSetup, error) {
	if f.getErr != nil {
		return domain.HostSetup{}, f.getErr
	}
	s, ok := f.setups[id]
	if !ok || s.TenantID != tenantID {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	return s, nil
}

func (f *fakeHostSetupRepository) List(ctx context.Context, tenantID string) ([]domain.HostSetup, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.HostSetup
	for _, s := range f.setups {
		if s.TenantID == tenantID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeHostSetupRepository) Update(ctx context.Context, tenantID, id string, patch domain.HostSetupPatch) (domain.HostSetup, error) {
	if f.updateErr != nil {
		return domain.HostSetup{}, f.updateErr
	}
	s, ok := f.setups[id]
	if !ok || s.TenantID != tenantID {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if patch.FolderPath != "" {
		s.FolderPath = patch.FolderPath
	}
	if patch.DisplayName != "" {
		s.DisplayName = patch.DisplayName
	}
	f.setups[id] = s
	return s, nil
}

func (f *fakeHostSetupRepository) Delete(ctx context.Context, tenantID, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	s, ok := f.setups[id]
	if !ok || s.TenantID != tenantID {
		return domain.ErrHostSetupNotFound
	}
	delete(f.setups, id)
	return nil
}

func (f *fakeHostSetupRepository) SetStatus(ctx context.Context, tenantID, id string, status domain.HostSetupStatus) error {
	if f.setStatusErr != nil {
		return f.setStatusErr
	}
	s, ok := f.setups[id]
	if !ok || s.TenantID != tenantID {
		return domain.ErrHostSetupNotFound
	}
	s.Status = status
	f.setups[id] = s
	return nil
}

func (f *fakeHostSetupRepository) Complete(ctx context.Context, tenantID, id, projectID string) (domain.HostSetup, error) {
	if f.completeErr != nil {
		return domain.HostSetup{}, f.completeErr
	}
	s, ok := f.setups[id]
	if !ok || s.TenantID != tenantID {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	s.Status = domain.HostSetupCompleted
	s.ProjectID = projectID
	f.setups[id] = s
	return s, nil
}

// fakeDevServerLister is an in-memory usecase.DevServerLister.
type fakeDevServerLister struct {
	exists bool
	err    error
}

func (f *fakeDevServerLister) Exists(ctx context.Context, tenantID, devServerID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.exists, nil
}

// fakeTerminalStatusResolver is an in-memory usecase.TerminalStatusResolver
// — sessionsByDevServer/errByDevServer let a test seed sessions (or force a
// failure) per dev_server_id; statusByPtyID/statusErrByPtyID do the same
// for GetAgentStatus. callsByDevServer/getAgentStatusCalls record call
// counts so tests can assert de-duplication (one ListSessionsForDevServer
// call per distinct dev_server_id, per TASK-MB-04-04's Verify section).
type fakeTerminalStatusResolver struct {
	mu sync.Mutex

	sessionsByDevServer map[string][]*infrafleetv1.TerminalSession
	errByDevServer      map[string]error
	callsByDevServer    map[string]int

	statusByPtyID       map[string]*infrafleetv1.GetTerminalAgentStatusResponse
	statusErrByPtyID    map[string]error
	getAgentStatusCalls []string
}

func newFakeTerminalStatusResolver() *fakeTerminalStatusResolver {
	return &fakeTerminalStatusResolver{
		sessionsByDevServer: map[string][]*infrafleetv1.TerminalSession{},
		errByDevServer:      map[string]error{},
		callsByDevServer:    map[string]int{},
		statusByPtyID:       map[string]*infrafleetv1.GetTerminalAgentStatusResponse{},
		statusErrByPtyID:    map[string]error{},
	}
}

func (f *fakeTerminalStatusResolver) ListSessionsForDevServer(ctx context.Context, devServerID string) ([]*infrafleetv1.TerminalSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callsByDevServer[devServerID]++
	if err, ok := f.errByDevServer[devServerID]; ok {
		return nil, err
	}
	return f.sessionsByDevServer[devServerID], nil
}

func (f *fakeTerminalStatusResolver) GetAgentStatus(ctx context.Context, ptyID string) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getAgentStatusCalls = append(f.getAgentStatusCalls, ptyID)
	if err, ok := f.statusErrByPtyID[ptyID]; ok {
		return nil, err
	}
	if status, ok := f.statusByPtyID[ptyID]; ok {
		return status, nil
	}
	return &infrafleetv1.GetTerminalAgentStatusResponse{}, nil
}

func (f *fakeTerminalStatusResolver) callCount(devServerID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callsByDevServer[devServerID]
}

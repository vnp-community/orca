package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

func seedMobileProject(t *testing.T, projects *fakeProjectRepository, id, tenantID, devServerID string) domain.Project {
	t.Helper()
	p := domain.Project{ID: id, TenantID: tenantID, Name: id, DevServerID: devServerID}
	if _, err := projects.Create(context.Background(), p); err != nil {
		t.Fatalf("seeding project: %v", err)
	}
	return p
}

func seedMobileWorktree(t *testing.T, worktrees *fakeWorktreeRepository, id, projectID, repoID, path, branch string) domain.Worktree {
	t.Helper()
	wt, err := domain.NewWorktree(id, projectID, repoID, path, branch)
	if err != nil {
		t.Fatalf("building worktree: %v", err)
	}
	if _, err := worktrees.RecordWorktreeCreated(context.Background(), wt); err != nil {
		t.Fatalf("seeding worktree: %v", err)
	}
	return wt
}

func TestGetMobileWorktreeStatus_RequiresTenantContext(t *testing.T) {
	uc := NewGetMobileWorktreeStatus(newFakeWorktreeRepository(), newFakeProjectRepository(), newFakeTerminalStatusResolver())
	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestGetMobileWorktreeStatus_NoBoundDevServer_OmitsRuntimeFieldsNotSkipped:
// a worktree whose project has no dev_server_id still appears in the list —
// just with empty runtime fields, not dropped.
func TestGetMobileWorktreeStatus_NoBoundDevServer_OmitsRuntimeFieldsNotSkipped(t *testing.T) {
	projects := newFakeProjectRepository()
	worktrees := newFakeWorktreeRepository()
	seedMobileProject(t, projects, "proj-1", "tenant-1", "")
	seedMobileWorktree(t, worktrees, "wt-1", "proj-1", "repo-1", "/home/wt-1", "feature-a")

	uc := NewGetMobileWorktreeStatus(worktrees, projects, newFakeTerminalStatusResolver())
	result, err := uc.Execute(withTenant(context.Background(), "tenant-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Worktrees) != 1 {
		t.Fatalf("expected the worktree to still appear in the list, got %d entries", len(result.Worktrees))
	}
	got := result.Worktrees[0]
	if got.ID != "wt-1" || got.Name != "feature-a" {
		t.Errorf("expected identity fields preserved, got %+v", got)
	}
	if got.Status != "" || got.Agent != "" || got.DurationMs != 0 || got.LastOutput != "" {
		t.Errorf("expected empty runtime fields with no bound dev server, got %+v", got)
	}
}

// TestGetMobileWorktreeStatus_NoMatchingSession_Idle: a worktree whose Path
// matches no live terminal session reports Status "idle".
func TestGetMobileWorktreeStatus_NoMatchingSession_Idle(t *testing.T) {
	projects := newFakeProjectRepository()
	worktrees := newFakeWorktreeRepository()
	seedMobileProject(t, projects, "proj-1", "tenant-1", "ds-1")
	seedMobileWorktree(t, worktrees, "wt-1", "proj-1", "repo-1", "/home/wt-1", "feature-a")

	terminals := newFakeTerminalStatusResolver()
	uc := NewGetMobileWorktreeStatus(worktrees, projects, terminals)
	result, err := uc.Execute(withTenant(context.Background(), "tenant-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result.Worktrees))
	}
	if result.Worktrees[0].Status != "idle" {
		t.Errorf("expected Status %q, got %q", "idle", result.Worktrees[0].Status)
	}
}

// TestGetMobileWorktreeStatus_MatchedSession_ComposesAgentStatus: a
// worktree whose Path matches a live session's Cwd composes Agent/Status/
// Duration/LastOutput from the session plus one GetAgentStatus call.
func TestGetMobileWorktreeStatus_MatchedSession_ComposesAgentStatus(t *testing.T) {
	projects := newFakeProjectRepository()
	worktrees := newFakeWorktreeRepository()
	seedMobileProject(t, projects, "proj-1", "tenant-1", "ds-1")
	seedMobileWorktree(t, worktrees, "wt-1", "proj-1", "repo-1", "/home/wt-1", "feature-a")

	terminals := newFakeTerminalStatusResolver()
	terminals.sessionsByDevServer["ds-1"] = []*infrafleetv1.TerminalSession{
		{PtyId: "pty-1", Cwd: "/home/wt-1", CreatedAtUnixMs: 1000, LastOutputPreview: "hello"},
	}
	terminals.statusByPtyID["pty-1"] = &infrafleetv1.GetTerminalAgentStatusResponse{
		AgentRunning: true, AgentKind: "claude-code", ReadyForInput: true,
	}

	uc := NewGetMobileWorktreeStatus(worktrees, projects, terminals)
	result, err := uc.Execute(withTenant(context.Background(), "tenant-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result.Worktrees))
	}
	got := result.Worktrees[0]
	if got.Status != "waiting" {
		t.Errorf("expected Status %q for a ready-for-input running agent, got %q", "waiting", got.Status)
	}
	if got.Agent != "claude-code" {
		t.Errorf("expected Agent %q, got %q", "claude-code", got.Agent)
	}
	if got.LastOutput != "hello" {
		t.Errorf("expected LastOutput %q, got %q", "hello", got.LastOutput)
	}
	if got.DurationMs <= 0 {
		t.Errorf("expected a positive DurationMs, got %d", got.DurationMs)
	}
	if calls := terminals.getAgentStatusCalls; len(calls) != 1 || calls[0] != "pty-1" {
		t.Errorf("expected exactly one GetAgentStatus call for pty-1, got %+v", calls)
	}
}

// TestGetMobileWorktreeStatus_ListSessionsError_DegradesToUnknown:
// ListSessionsForDevServer erroring for one dev server degrades that dev
// server's worktrees to "unknown", other dev servers unaffected.
func TestGetMobileWorktreeStatus_ListSessionsError_DegradesToUnknown(t *testing.T) {
	projects := newFakeProjectRepository()
	worktrees := newFakeWorktreeRepository()
	seedMobileProject(t, projects, "proj-bad", "tenant-1", "ds-bad")
	seedMobileWorktree(t, worktrees, "wt-bad", "proj-bad", "repo-1", "/home/wt-bad", "feature-bad")
	seedMobileProject(t, projects, "proj-ok", "tenant-1", "ds-ok")
	seedMobileWorktree(t, worktrees, "wt-ok", "proj-ok", "repo-1", "/home/wt-ok", "feature-ok")

	terminals := newFakeTerminalStatusResolver()
	terminals.errByDevServer["ds-bad"] = context.DeadlineExceeded

	uc := NewGetMobileWorktreeStatus(worktrees, projects, terminals)
	result, err := uc.Execute(withTenant(context.Background(), "tenant-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(result.Worktrees))
	}
	byID := map[string]MobileWorktreeStatus{}
	for _, wt := range result.Worktrees {
		byID[wt.ID] = wt
	}
	if byID["wt-bad"].Status != "unknown" {
		t.Errorf("expected wt-bad Status %q, got %q", "unknown", byID["wt-bad"].Status)
	}
	if byID["wt-ok"].Status != "idle" {
		t.Errorf("expected wt-ok (unaffected dev server, no session) Status %q, got %q", "idle", byID["wt-ok"].Status)
	}
}

// TestGetMobileWorktreeStatus_SharedDevServer_ListSessionsCalledOnce: N
// worktrees sharing one dev_server_id must trigger exactly one
// ListSessionsForDevServer call, not one per worktree.
func TestGetMobileWorktreeStatus_SharedDevServer_ListSessionsCalledOnce(t *testing.T) {
	projects := newFakeProjectRepository()
	worktrees := newFakeWorktreeRepository()
	seedMobileProject(t, projects, "proj-1", "tenant-1", "ds-shared")
	seedMobileWorktree(t, worktrees, "wt-1", "proj-1", "repo-1", "/home/wt-1", "feature-a")
	seedMobileWorktree(t, worktrees, "wt-2", "proj-1", "repo-1", "/home/wt-2", "feature-b")
	seedMobileWorktree(t, worktrees, "wt-3", "proj-1", "repo-1", "/home/wt-3", "feature-c")

	terminals := newFakeTerminalStatusResolver()
	uc := NewGetMobileWorktreeStatus(worktrees, projects, terminals)
	result, err := uc.Execute(withTenant(context.Background(), "tenant-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Worktrees) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(result.Worktrees))
	}
	if got := terminals.callCount("ds-shared"); got != 1 {
		t.Errorf("expected exactly 1 ListSessionsForDevServer call for the shared dev server, got %d", got)
	}
}

// TestGetMobileWorktreeStatus_InactiveWorktree_Excluded: a soft-removed
// (Active=false) worktree is excluded from the response entirely — BL-MB-04
// only reports live worktrees.
func TestGetMobileWorktreeStatus_InactiveWorktree_Excluded(t *testing.T) {
	projects := newFakeProjectRepository()
	worktrees := newFakeWorktreeRepository()
	seedMobileProject(t, projects, "proj-1", "tenant-1", "")
	seedMobileWorktree(t, worktrees, "wt-1", "proj-1", "repo-1", "/home/wt-1", "feature-a")
	if _, err := worktrees.SetWorktreeActivation(context.Background(), "wt-1", false); err != nil {
		t.Fatalf("deactivating worktree: %v", err)
	}

	uc := NewGetMobileWorktreeStatus(worktrees, projects, newFakeTerminalStatusResolver())
	result, err := uc.Execute(withTenant(context.Background(), "tenant-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Worktrees) != 0 {
		t.Fatalf("expected an inactive worktree to be excluded, got %+v", result.Worktrees)
	}
}

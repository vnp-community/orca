package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CreateWorktreeFromIssueInput struct {
	ProjectID, RepoID, BaseRef string
	IssueRef                   domain.IssueRef
	SkipStatusUpdate           bool
	SkipAgentStart             bool
}

type WorktreeFromIssueResult struct {
	Worktree             domain.WorktreeResult
	BranchName           string
	AgentSessionID       string
	AgentStartError      string
	StatusUpdateEnqueued bool
}

// CreateWorktreeFromIssue is CreateWorktree's saga with two steps prepended
// (fetch issue, derive branch) and two appended (spawn agent, thread the
// issue link through Lineage for project-service to publish — see
// SOL-PI-03; this service owns no database and cannot itself enqueue an
// outbox event).
type CreateWorktreeFromIssue struct {
	issues   IssueSourceClient
	createWT *CreateWorktree
	agents   AgentSpawner
	projects ProjectClient
}

func NewCreateWorktreeFromIssue(issues IssueSourceClient, createWT *CreateWorktree, agents AgentSpawner, projects ProjectClient) *CreateWorktreeFromIssue {
	return &CreateWorktreeFromIssue{issues: issues, createWT: createWT, agents: agents, projects: projects}
}

func (uc *CreateWorktreeFromIssue) Execute(ctx context.Context, in CreateWorktreeFromIssueInput) (WorktreeFromIssueResult, error) {
	issue, err := uc.issues.GetIssue(ctx, in.IssueRef)
	if err != nil {
		return WorktreeFromIssueResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_FROM_ISSUE_ISSUE_NOT_FOUND", "issue not found", err)
	}

	branch := generateBranchName(issue.Title, issue.Labels, issue.ExternalRef) // BR-PI-04

	lineage := domain.WorktreeLineageCapture{
		Origin: "issue", CaptureSource: "createWorktreeFromIssue",
		LinkedIssueProvider: issue.Provider, LinkedIssueRef: issue.ExternalRef,
	}
	if in.SkipStatusUpdate {
		lineage.LinkedIssueProvider, lineage.LinkedIssueRef = "", ""
	} else if enabled, err := uc.projects.IsIssueStatusSyncEnabled(ctx, in.ProjectID); err != nil || !enabled {
		// BR-PI-06: don't even record the link if sync is off for this project.
		lineage.LinkedIssueProvider, lineage.LinkedIssueRef = "", ""
	}

	wtResult, err := uc.createWT.Execute(ctx, CreateWorktreeInput{
		ProjectID: in.ProjectID, RepoID: in.RepoID, Branch: branch, BaseRef: in.BaseRef,
		Lineage: lineage,
	})
	if err != nil {
		return WorktreeFromIssueResult{}, err // CreateWorktree's own compensation already ran
	}

	result := WorktreeFromIssueResult{
		Worktree: wtResult, BranchName: branch,
		StatusUpdateEnqueued: lineage.LinkedIssueRef != "",
	}

	if !in.SkipAgentStart {
		prompt := buildAgentPrompt(issue.Title, issue.Description, issue.AcceptanceCriteria, issue.Comments) // BR-PI-05
		sessionID, err := uc.agents.SpawnAndInject(ctx, wtResult.WorktreeID, wtResult.Path, prompt)
		if err != nil {
			// Non-fatal: the worktree exists; agent spawn is a separate
			// BL-AG-01 concern with its own failure modes (e.g. Dev Server
			// not connected) — never roll back a successful worktree for it.
			result.AgentStartError = err.Error()
		} else {
			result.AgentSessionID = sessionID
		}
	}
	return result, nil
}

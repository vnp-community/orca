# TASK-PI-02-07: `worktree.createFromIssue` WS channel

**From Solution:** SOL-PI-02
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go`
**Depends on:** TASK-PI-02-05
**Status:** `[ ]` TODO

---

## Context

Exposes the new `CreateWorktreeFromIssue` RPC over the WS-compat surface,
next to the existing `worktree.create` registration (`channels_worktree.go:36-74`),
translating the request's `oneof` shape (SCM vs. tracker issue) from the
decoded args struct.

## Changes to make

```go
r.Register("worktree.createFromIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type createFromIssueArgs struct {
        ProjectID string `json:"projectId"`
        RepoID    string `json:"repoId"`
        BaseRef   string `json:"baseRef"`
        Provider  string `json:"provider"`   // "github"|"gitlab"|"jira"|"linear"
        Repo      string `json:"repo"`       // scm only
        Number    int32  `json:"number"`     // scm only
        IssueRef  string `json:"issueRef"`   // tracker only
        SkipAgentStart   bool `json:"skipAgentStart"`
        SkipStatusUpdate bool `json:"skipStatusUpdate"`
    }
    in, err := decodeArg[createFromIssueArgs](args, 0)
    if err != nil {
        return nil, err
    }

    req := &gitgatewayv1.CreateWorktreeFromIssueRequest{
        ProjectId: in.ProjectID, RepoId: in.RepoID, BaseRef: in.BaseRef,
        SkipAgentStart: in.SkipAgentStart, SkipStatusUpdate: in.SkipStatusUpdate,
    }
    switch in.Provider {
    case "github", "gitlab":
        if in.Repo == "" || in.Number == 0 {
            return nil, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_FROM_ISSUE_MISSING_SCM_FIELDS", "repo and number are required for github/gitlab", nil)
        }
        req.IssueSource = &gitgatewayv1.CreateWorktreeFromIssueRequest_ScmIssue{
            ScmIssue: &gitgatewayv1.ScmIssueRef{Provider: in.Provider, Repo: in.Repo, Number: in.Number},
        }
    case "jira", "linear":
        if in.IssueRef == "" {
            return nil, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_FROM_ISSUE_MISSING_TRACKER_REF", "issueRef is required for jira/linear", nil)
        }
        req.IssueSource = &gitgatewayv1.CreateWorktreeFromIssueRequest_TrackerIssue{
            TrackerIssue: &gitgatewayv1.TrackerIssueRef{Provider: in.Provider, IssueRef: in.IssueRef},
        }
    default:
        return nil, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_FROM_ISSUE_UNKNOWN_PROVIDER", "provider must be one of github/gitlab/jira/linear", nil)
    }

    ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
    resp, err := gitClient.CreateWorktreeFromIssue(ctx, req)
    if err != nil {
        return nil, err
    }
    return resp, nil
})
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
```

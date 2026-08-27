# TASK-PI-01-07: Wire issue filters, `force_refresh`, and the two new RPCs into `api-gateway`

**From Solution:** SOL-PI-01
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go`
**Depends on:** TASK-PI-01-01, TASK-PI-01-03, TASK-PI-01-05, TASK-PI-01-06
**Status:** `[ ]` TODO

---

## Context

The REST/WS edges for `ListIssues` currently only forward
`tenant_id`/`provider`/`repo` (`scm_routes.go`'s `ListIssues` handler,
confirmed at lines ~41-45). This task adds the new query params/channel args
and two new endpoints, with no new gateway business logic — pure translation,
per the existing pattern this file and `channels_scm.go` already use.

## Changes to make

### 1. `scm_routes.go` — extend `GET /v1/scm/issues`

```go
resp, err := client.ListIssues(ctx, &scmintegrationv1.ListIssuesRequest{
    TenantId: identity.TenantID,
    Provider: parseSCMProvider(q.Get("provider")),
    Repo:     q.Get("repo"),
    Filter: &scmintegrationv1.IssueFilter{
        State:     q.Get("state"),
        Assignee:  q.Get("assignee"),
        Labels:    q["label"], // repeatable query param
        Milestone: q.Get("milestone"),
    },
    ForceRefresh: q.Get("refresh") == "true",
})
```

Add a new route, following this file's existing handler shape:

```go
// GET /v1/scm/issues/{number}/comments
func (h *Handler) ListIssueComments(w http.ResponseWriter, r *http.Request) {
    identity := identityFromContext(r.Context())
    slug := fmt.Sprintf("%s#%s", chi.URLParam(r, "repo"), chi.URLParam(r, "number"))
    resp, err := h.scmClient.ListIssueCommentsBySlug(r.Context(), &scmintegrationv1.ListIssueCommentsBySlugRequest{
        TenantId: identity.TenantID, ItemSlug: slug,
    })
    if err != nil {
        writeGRPCError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, resp)
}
```

Mount it in this package's router-registration function next to the
existing `/v1/scm/issues` route.

### 2. `channels_scm.go` — extend `github.issues`, add `github.issueComments`

```go
r.Register("github.issues", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type issuesArgs struct {
        Repo      string   `json:"repo"`
        State     string   `json:"state"`
        Assignee  string   `json:"assignee"`
        Labels    []string `json:"labels"`
        Milestone string   `json:"milestone"`
        Refresh   bool     `json:"refresh"`
    }
    in, err := decodeArg[issuesArgs](args, 0)
    if err != nil {
        return nil, err
    }
    rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
    defer cancel()
    resp, err := client.ListIssues(attachSCMIdentity(rpcCtx, id), &scmintegrationv1.ListIssuesRequest{
        TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB, Repo: in.Repo,
        Filter: &scmintegrationv1.IssueFilter{State: in.State, Assignee: in.Assignee, Labels: in.Labels, Milestone: in.Milestone},
        ForceRefresh: in.Refresh,
    })
    if err != nil {
        return nil, err
    }
    return resp, nil
})

// github.issueComments — same registration pattern as github.rateLimit.
r.Register("github.issueComments", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type commentsArgs struct {
        ItemSlug string `json:"itemSlug"`
    }
    in, err := decodeArg[commentsArgs](args, 0)
    if err != nil {
        return nil, err
    }
    rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
    defer cancel()
    resp, err := client.ListIssueCommentsBySlug(attachSCMIdentity(rpcCtx, id),
        &scmintegrationv1.ListIssueCommentsBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
    if err != nil {
        return nil, err
    }
    return resp, nil
})
```

Apply the same registration for `gitlab.issues`/`gitlab.issueComments` if
those channels exist as a separate pair (mirror whatever `gitlab.rateLimit`
does relative to `github.rateLimit`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
```

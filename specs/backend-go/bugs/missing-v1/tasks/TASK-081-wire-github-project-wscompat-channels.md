# TASK-081: Wire `github.project.*` (GitHub Projects v2) wscompat channels

**From Solution:** SOL-012 (Design — `wscompat` channel wiring, shape 3)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_github.go`
**Depends on:** TASK-079, TASK-080
**Status:** `[x]` DONE — implemented in worktree `agent-aac2382028c6ce920` (branch `worktree-agent-aac2382028c6ce920`), **committed** as `ce750c490`. `go build`/`go vet`/`gofmt -l` clean, `buf generate`/`buf breaking` clean (additive-only). Pending merge to main + one-line RegisterRealChannels/main.go wiring.

---

## Context

Appends the 16 `github.project.*` channels to `channels_github.go`
(TASK-080). These RPCs have no `provider` field on their request messages
(TASK-077 — GitHub-only by construction), so unlike every other handler in
this file, none of these pass a `Provider:` field.

---

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels_github.go`

### Step 1: Append the 16 handlers inside `registerGitHubChannels`

Add, just before the closing `}` of `registerGitHubChannels` (after
`github.revokeAuth`'s registration from TASK-080):

```go
	// github.project.* — GitHub Projects v2. No provider field on any
	// request message here (TASK-077): this whole sub-surface is
	// GitHub-only by construction, unlike every handler above.
	r.Register("github.project.listAccessible", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListAccessibleProjects(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ListAccessibleProjectsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.resolveRef", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			Owner  string `json:"owner"`
			Number int32  `json:"number"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ResolveProjectRef(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ResolveProjectRefRequest{TenantId: id.TenantID, Owner: in.Owner, Number: in.Number})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listViews", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type viewsArgs struct {
			ProjectSlug string `json:"projectSlug"`
		}
		in, err := decodeArg[viewsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListProjectViews(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ListProjectViewsRequest{TenantId: id.TenantID, ProjectSlug: in.ProjectSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.viewTable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type tableArgs struct {
			ProjectSlug string `json:"projectSlug"`
			ViewID      string `json:"viewId"`
			PageToken   string `json:"pageToken"`
			PageSize    int32  `json:"pageSize"`
		}
		in, err := decodeArg[tableArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ViewProjectTable(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ViewProjectTableRequest{
				TenantId: id.TenantID, ProjectSlug: in.ProjectSlug, ViewId: in.ViewID, PageToken: in.PageToken, PageSize: in.PageSize,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateItemField", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateFieldArgs struct {
			ProjectSlug string `json:"projectSlug"`
			ItemID      string `json:"itemId"`
			FieldID     string `json:"fieldId"`
			Kind        string `json:"kind"`
			Value       string `json:"value"`
		}
		in, err := decodeArg[updateFieldArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateProjectItemField(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.UpdateProjectItemFieldRequest{
				TenantId: id.TenantID, ProjectSlug: in.ProjectSlug, ItemId: in.ItemID,
				Field: &scmintegrationv1.ProjectFieldValue{FieldId: in.FieldID, Kind: in.Kind, Value: in.Value},
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.clearItemField", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type clearFieldArgs struct {
			ProjectSlug string `json:"projectSlug"`
			ItemID      string `json:"itemId"`
			FieldID     string `json:"fieldId"`
		}
		in, err := decodeArg[clearFieldArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ClearProjectItemField(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ClearProjectItemFieldRequest{TenantId: id.TenantID, ProjectSlug: in.ProjectSlug, ItemId: in.ItemID, FieldId: in.FieldID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.workItemDetailsBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetWorkItemDetailsBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.GetWorkItemDetailsBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateIssueBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ItemSlug     string   `json:"itemSlug"`
			Title        *string  `json:"title"`
			Body         *string  `json:"body"`
			State        *string  `json:"state"`
			AddLabels    []string `json:"addLabels"`
			RemoveLabels []string `json:"removeLabels"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &scmintegrationv1.UpdateIssueBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, AddLabels: in.AddLabels, RemoveLabels: in.RemoveLabels}
		if in.Title != nil {
			req.Title = in.Title
		}
		if in.Body != nil {
			req.Body = in.Body
		}
		if in.State != nil {
			req.State = in.State
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateIssueBySlug(attachGitHubIdentity(rpcCtx, id), req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updatePullRequestBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ItemSlug string  `json:"itemSlug"`
			Title    *string `json:"title"`
			Body     *string `json:"body"`
			State    *string `json:"state"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &scmintegrationv1.UpdatePullRequestBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug}
		if in.Title != nil {
			req.Title = in.Title
		}
		if in.Body != nil {
			req.Body = in.Body
		}
		if in.State != nil {
			req.State = in.State
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdatePullRequestBySlug(attachGitHubIdentity(rpcCtx, id), req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateIssueTypeBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type typeArgs struct {
			ItemSlug  string `json:"itemSlug"`
			IssueType string `json:"issueType"`
		}
		in, err := decodeArg[typeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateIssueTypeBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.UpdateIssueTypeBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, IssueType: in.IssueType})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listIssueTypesBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListIssueTypesBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ListIssueTypesBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listAssignableUsersBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListAssignableUsersBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ListAssignableUsersBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listLabelsBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListLabelsBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.ListLabelsBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.addIssueCommentBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentArgs struct {
			ItemSlug string `json:"itemSlug"`
			Body     string `json:"body"`
		}
		in, err := decodeArg[commentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.AddIssueCommentBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.AddIssueCommentBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, Body: in.Body})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateIssueCommentBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentArgs struct {
			ItemSlug  string `json:"itemSlug"`
			CommentID string `json:"commentId"`
			Body      string `json:"body"`
		}
		in, err := decodeArg[commentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateIssueCommentBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.UpdateIssueCommentBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, CommentId: in.CommentID, Body: in.Body})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.deleteIssueCommentBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentArgs struct {
			ItemSlug  string `json:"itemSlug"`
			CommentID string `json:"commentId"`
		}
		in, err := decodeArg[commentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err = client.DeleteIssueCommentBySlug(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.DeleteIssueCommentBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, CommentId: in.CommentID})
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
```

This is 15 handlers, not 16 — `github.project.resolveRef`'s item-read
counterpart (SOL-012's signature table 17th row, "item read for
resolveRef/viewTable") has no separate RPC or channel; it's covered by
`resolveRef`/`viewTable` themselves (see TASK-077's note).

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/... && go vet ./internal/adapter/wscompat/...
```

Expected: clean build. Every `github.project.*` method in
`specs/frontend/api/rpc-catalog.md` now resolves to a real channel instead
of `notImplementedHandler`.

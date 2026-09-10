# TASK-CR-05-08: Wire `draft`/`linkedIssueNumber` into `hostedReview.create` and add `hostedReview.suggestReviewers`

**From Solution:** SOL-CR-05
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go`
**Depends on:** TASK-CR-05-06, TASK-CR-05-07
**Status:** `[x]` DONE — hostedReview.create forwards draft/linkedIssueNumber (*int32, unset stays nil — no live frontend call site sends it yet) and now returns the whole CreatePullRequestResponse; hostedReview.suggestReviewers wired. All TestHostedReview* cases pass.

---

## Context

`hostedReview.create`'s current args (`channels_scm.go:741-767`) don't
carry `draft`/`linkedIssueNumber`, so BR-CR-20/BR-CR-19 are unreachable
from the frontend even once the backend supports them. This task threads
both through and registers the new `hostedReview.suggestReviewers`
channel. `changedFiles` is supplied by the client, not fetched here — the
AI-description flow (`generate_pull_request_fields.go`'s frontend caller)
already gathers the changed-file list client-side via `git.diff`/
`git.status` before calling `git.generatePullRequestFields`, so this reuses
data the frontend flow already has at that point, adding zero new
`git-gateway-service` calls.

## Changes to make

Update `hostedReview.create`'s handler:

```go
	r.Register("hostedReview.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Provider          string `json:"provider"`
			Repo              string `json:"repo"`
			Title             string `json:"title"`
			Body              string `json:"body"`
			HeadBranch        string `json:"headBranch"`
			BaseBranch        string `json:"baseBranch"`
			RequestID         string `json:"requestId"`
			Draft             bool   `json:"draft"`             // NEW
			LinkedIssueNumber int32  `json:"linkedIssueNumber"` // NEW
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.CreatePullRequest(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.CreatePullRequestRequest{
				TenantId: id.TenantID, Provider: parseWSProvider(in.Provider),
				Repo: in.Repo, Title: in.Title, Body: in.Body,
				HeadBranch: in.HeadBranch, BaseBranch: in.BaseBranch, RequestId: in.RequestID,
				Draft: in.Draft, LinkedIssueNumber: proto.Int32(in.LinkedIssueNumber),
			})
		if err != nil {
			return nil, err
		}
		return resp, nil // NEW — was resp.GetPullRequest(); now returns the whole response so
		                  // linked_issue_update_error (BR-CR-19) reaches the client too
	})
```

`LinkedIssueNumber` on the proto is `optional int32` (TASK-CR-05-01) — use
`proto.Int32(in.LinkedIssueNumber)` unconditionally only if `0` is an
acceptable "no linked issue" sentinel for this field's `optional` wire
semantics; if the frontend never intends to omit it, wrap with a
`*int32`-typed arg (`LinkedIssueNumber *int32`) instead so a genuinely
absent value round-trips as unset rather than `0`. Confirm against the
actual frontend call site before choosing.

Add `"google.golang.org/protobuf/proto"` to this file's imports if not
already present.

Register the new channel, near the other `hostedReview.*` registrations in
`registerHostedReviewChannels`:

```go
	r.Register("hostedReview.suggestReviewers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type suggestArgs struct {
			Provider     string   `json:"provider"`
			Repo         string   `json:"repo"`
			BaseRef      string   `json:"baseRef"`
			ChangedFiles []string `json:"changedFiles"`
		}
		in, err := decodeArg[suggestArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.SuggestPullRequestReviewers(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.SuggestPullRequestReviewersRequest{
				TenantId: id.TenantID, Provider: parseWSProvider(in.Provider),
				Repo: in.Repo, BaseRef: in.BaseRef, ChangedFiles: in.ChangedFiles,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./...
go vet ./...
go test ./internal/adapter/wscompat/... -run TestHostedReview -v
```

Add cases to `channels_scm_test.go`: `hostedReview.create` forwards
`draft`/`linkedIssueNumber` into the RPC request and returns
`linked_issue_update_error` when the fake client sets it;
`hostedReview.suggestReviewers` wired and forwards `changedFiles`.

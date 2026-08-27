# TASK-PI-04-06: `hostedReview.submit` WS channel

**From Solution:** SOL-PI-04
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go`
**Depends on:** TASK-PI-04-02
**Status:** `[ ]` TODO

---

## Context

Wraps the same annotation-aggregation composition as `SubmitPullRequestReview`
(TASK-PI-04-05) for the WS-compat surface, matching `hostedReview.create`'s
existing registration pattern (`channels_scm.go:741-767`). BR-PI-12 needs no
special-case code here — this channel and the BL-CR-03 agent-feedback
channel are already independent calls the frontend can issue in parallel;
no new orchestration.

## Changes to make

```go
r.Register("hostedReview.submit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type submitArgs struct {
        RepoID     string `json:"repoId"`
        Provider   string `json:"provider"`
        PRNumber   int32  `json:"prNumber"`
        ReviewType string `json:"reviewType"`
        Summary    string `json:"summary"`
    }
    in, err := decodeArg[submitArgs](args, 0)
    if err != nil {
        return nil, err
    }

    // Aggregation read — same drain-all-pages + BR-PI-10 fail-fast as
    // pr_review_routes.go's SubmitPullRequestReview (TASK-PI-04-05).
    var allAnnotations []*annotationv1.Annotation
    pageToken := ""
    for {
        resp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
            RepoId: in.RepoID, PageToken: pageToken,
        })
        if err != nil {
            return nil, err
        }
        allAnnotations = append(allAnnotations, resp.GetAnnotations()...)
        if pageToken = resp.GetNextPageToken(); pageToken == "" {
            break
        }
    }
    if len(allAnnotations) == 0 {
        return nil, apperrors.New(apperrors.KindInvalidArgument, "PR_REVIEW_NO_COMMENTS", "annotate at least one line before submitting a review", nil)
    }

    comments := make([]*scmintegrationv1.ReviewComment, 0, len(allAnnotations))
    for _, a := range allAnnotations {
        comments = append(comments, &scmintegrationv1.ReviewComment{
            Path: a.GetAnchor().GetFilePath(), Line: a.GetAnchor().GetLine(), Body: a.GetContent(),
        })
    }

    rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
    defer cancel()
    resp, err := client.SubmitReview(attachSCMIdentity(rpcCtx, id), &scmintegrationv1.SubmitReviewRequest{
        TenantId: id.TenantID, Provider: parseWSProvider(in.Provider), Repo: in.RepoID,
        PrNumber: in.PRNumber, ReviewType: parseReviewType(in.ReviewType),
        SummaryBody: in.Summary, Comments: comments,
    })
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

# SOL-PI-04: `SubmitReview` RPC — annotation → GitHub PR review composition

**Resolves:** [BUG-PI-04](../BUG-PI-04-submit-pr-review-not-implemented.md)
**Service:** `scm-integration-service` (new `SubmitReview` RPC + GitHub Reviews API client), `api-gateway` (aggregation-read of `annotation-service` + call to `scm-integration-service`, per the multi-service-view prescription — see below)
**Affected files (proposed):**
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto` (`SubmitReview` RPC — already sketched in the TDD, not yet in the implemented proto; see rationale)
- `backend-go/services/scm-integration-service/internal/usecase/submit_review.go` (new)
- `backend-go/services/scm-integration-service/internal/usecase/ports.go` (extend `ScmProvider` with `SubmitReview`)
- `backend-go/services/scm-integration-service/internal/adapter/github/client.go` (`POST /repos/:repo/pulls/:pr/reviews`)
- `backend-go/services/scm-integration-service/internal/adapter/gitlab/client.go` (GitLab MR discussions equivalent — see provider-capability note)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/pr_review_routes.go` (new — aggregates `annotation-service` + `scm-integration-service`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go` (new `hostedReview.submit` channel)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### The RPC already exists in the TDD's own §3 sketch — this closes an implementation gap, not a design gap

`scm-integration-service.md`'s API surface section already lists
`rpc SubmitReview(SubmitReviewRequest) returns (Review);` grouped under
"Reviewers & reviews" (`scm-integration-service.md:83`), and §4's domain
model already defines the `Review` value object this RPC returns —
"reviewer identity, state (approved / changes-requested / commented),
submitted-at, comments" (`scm-integration-service.md:121-123`) — which maps
1:1 onto BL-PI-04's three review types (Comment/Approve/Request Changes)
and its per-line comment list. BUG-PI-04's finding that the *implemented*
`scmintegration.proto` has no such RPC (`scmintegration.proto` lines 12-89
per BUG-PI-04:23) describes drift between the TDD and the current proto,
the same situation SOL-PI-01 found for `ListComments`/`*BySlug` comments —
the fix is to implement what the design doc already specified, not design
something new. This solution's only real design work is the
`AnnotationService → ScmIntegrationService` composition BUG-PI-04 correctly
identifies as genuinely missing (§"What's missing", third bullet) — no RPC
sketch anywhere covers turning `Annotation` rows into a GitHub review
payload, because `annotation.proto` and `scmintegration.proto` are
deliberately independent services with no cross-reference
(`annotation.proto:1-58`, confirmed zero SCM-facing fields).

### Where the composition (Annotation → review payload) happens: `api-gateway`, not either service

Neither `AnnotationService` nor `ScmIntegrationService` should import the
other's domain model — that would violate
`02-microservices-decomposition.md`'s design principle 1 ("cross-service
references are 'logical FKs'... validated by calling the owning service's
API" — `02-microservices-decomposition.md:14-18`) if either service
started reaching into the other's data to build its own request. This is
exactly the shape `05-data-architecture.md`'s "Read models / query needs
across service boundaries" section prescribes: "Where the frontend needs
data assembled from multiple services in one view... `api-gateway`
performs the aggregation (parallel gRPC calls, merge in the edge layer)
rather than any service reaching into another's database"
(`05-data-architecture.md:116-119`). Submitting a review is the write-side
mirror of that same read-aggregation shape — `api-gateway` already calls
`AnnotationService.ListAnnotations` for the diff view (per
`annotation-service`'s existing wiring) and now additionally maps that
list into `scmintegration.proto`'s new `SubmitReviewRequest.comments`
before calling `SubmitReview`. This keeps both services' domain models
untouched and matches `08-inter-service-communication.md`'s "API Gateway
responsibilities" #4, "[r]outes... response aggregation"
(`08-inter-service-communication.md:55`) — the closest existing
responsibility bucket, generalized from read-aggregation to this
write-composition case since no other layer in the system is positioned to
see both services' data in one request.

**Flagged as a genuine extension**: `08-inter-service-communication.md`'s
own aggregation examples are all reads; this is this solution's one
deliberate generalization — composing a *write* request at the gateway
from one service's read (`ListAnnotations`) and forwarding into another
service's write (`SubmitReview`), rather than merging two reads for
display. The alternative (a synchronous saga call from
`scm-integration-service` into `annotation-service`) was considered and
rejected: it would add an inbound dependency edge annotation-service has
no reason to accept (its own doc has zero outbound callers listed beyond
`api-gateway`), and — per `02-microservices-decomposition.md`'s dependency
graph — no `scm --> annot` edge exists today
(`02-microservices-decomposition.md:110-166`) and adding one only to serve
this one composite call is a larger footprint than doing the merge where
the system already does every other cross-service merge.

### BR-PI-10/11/12 — validated at the gateway composition step and re-validated in the usecase

BR-PI-10 ("phải có ít nhất 1 comment để submit review" —
`docs/logic/project-integration/BL-PI-04-submit-pr-review.md:41`) and
BR-PI-11 (default "Request Changes" when comments exist,
`BL-PI-04:42`) are business rules about the *request shape*, not about a
provider's API — enforced twice, per the same "belt-and-braces" posture
SOL-PI-03 uses for BR-PI-07: once at `api-gateway`'s composition step
(fail fast, cheap, no network call to `scm-integration-service` for an
input that's already invalid) and again inside `submit_review.go`'s
`Execute` (so a future second caller of `SubmitReview` — a CLI, an
automation trigger — can't bypass the rule by skipping the gateway path).
BR-PI-12 ("GitHub review và agent feedback có thể submit cùng lúc",
`BL-PI-04:43`) requires no new orchestration: it just means this solution
must not make `SubmitReview` and BL-CR-03's agent-feedback path mutually
exclusive at the gateway route level — both are independent calls
`api-gateway` can issue in the same request handler (parallel, via
`errgroup`, matching the existing `worktree.detectedList` aggregation
pattern at `channels_worktree.go`'s doc comment, "parallel gRPC calls via
errgroup, merged here" — cited in SOL-PI-01/02's sibling solutions as the
established idiom).

### Provider capability gap: GitLab has no identical "Reviews API"

`scm-integration-service.md` §4 already has the escape hatch this needs:
"[a] provider that doesn't support an operation... returns typed
`ErrCapabilityUnsupported`" (`scm-integration-service.md:139-140`).
GitLab's closest equivalent to a GitHub PR review (one atomic
Comment/Approve/Request-Changes submission with per-line comments) is a
combination of per-line "discussions" plus a separate approve/unapprove
call — not one endpoint. This solution's GitLab adapter composes that
sequence internally (create discussions, then approve or note a
"changes requested" summary comment) rather than exposing the seam to
`usecase/`, but documents this as an intentional per-provider difference,
consistent with §6's "no shared base class" package-layout note
(`scm-integration-service.md:166-167`) — each provider adapter owns its
own translation of the provider-agnostic `SubmitReview` call.

---

## Design — proto (`scmintegration.proto`)

```protobuf
// Already named in scm-integration-service.md §3 (line 83) — implementing
// the RPC that design doc specified, not inventing a new one.
rpc SubmitReview(SubmitReviewRequest) returns (Review);

enum ReviewType {
  REVIEW_TYPE_UNSPECIFIED = 0;
  REVIEW_TYPE_COMMENT = 1;
  REVIEW_TYPE_APPROVE = 2;
  REVIEW_TYPE_REQUEST_CHANGES = 3;
}

message ReviewComment {
  string path = 1;       // file path, matches Annotation.Anchor.FilePath (annotation.proto:20)
  int32 line = 2;         // matches Annotation.Anchor.Line (annotation.proto:21)
  string body = 3;        // matches Annotation.Content (annotation.proto:29)
}

message SubmitReviewRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 pr_number = 4;
  ReviewType review_type = 5;         // REVIEW_TYPE_UNSPECIFIED triggers BR-PI-11's default-to-REQUEST_CHANGES
  string summary_body = 6;            // top-level review comment, optional
  repeated ReviewComment comments = 7; // BR-PI-10: must be non-empty
}

message Review {
  string id = 1;
  string reviewer_id = 2;
  ReviewType state = 3;
  string submitted_at = 4;
  repeated ReviewComment comments = 5;
  string url = 6;
}
```

`ReviewComment`'s `path`/`line`/`body` fields deliberately mirror
`annotation.proto`'s `Anchor`/`Annotation.content` field names
(`annotation.proto:20-21,29`) — the `api-gateway` mapping step is then a
pure 1:1 field copy, not a translation, which is exactly the kind of
low-risk composition this cross-service call should be.

---

## Design — `usecase/` layer (`scm-integration-service`)

```go
// internal/usecase/submit_review.go
type SubmitReviewParams struct {
    TenantID   string
    Provider   domain.ScmProvider
    Repo       string
    PRNumber   int32
    ReviewType domain.ReviewType
    Summary    string
    Comments   []domain.ReviewComment
}

type SubmitReview struct {
    credentials CredentialResolver
    providers   ProviderRegistry
}

func (uc *SubmitReview) Execute(ctx context.Context, in SubmitReviewParams) (domain.Review, error) {
    if len(in.Comments) == 0 { // BR-PI-10, re-validated here per rationale above
        return domain.Review{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_REVIEW_EMPTY_COMMENTS", "at least one comment is required to submit a review", nil)
    }
    reviewType := in.ReviewType
    if reviewType == domain.ReviewTypeUnspecified {
        reviewType = domain.ReviewTypeRequestChanges // BR-PI-11
    }

    cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
    if err != nil {
        return domain.Review{}, err
    }
    provider, err := uc.providers.For(in.Provider)
    if err != nil {
        return domain.Review{}, err
    }
    review, err := provider.SubmitReview(ctx, cred, in.Repo, in.PRNumber, domain.ReviewInput{
        Type: reviewType, Summary: in.Summary, Comments: in.Comments,
    })
    if err != nil {
        return domain.Review{}, translateProviderError(err) // ErrCapabilityUnsupported etc., §4's existing pattern
    }
    return review, nil
}
```

### `ScmProvider` port extension (`ports.go`)

```go
type ScmProvider interface {
    // ... existing methods ...
    SubmitReview(ctx context.Context, cred Credential, repo string, prNumber int32, in domain.ReviewInput) (domain.Review, error)
}
```

### GitHub adapter (`github/client.go`)

```go
func (c *Client) SubmitReview(ctx context.Context, cred Credential, repo string, prNumber int32, in domain.ReviewInput) (domain.Review, error) {
    payload := githubReviewPayload{
        Body:  in.Summary,
        Event: reviewTypeToGitHubEvent(in.Type), // COMMENT | APPROVE | REQUEST_CHANGES
        Comments: mapComments(in.Comments),        // {path, line, body}[]
    }
    // POST /repos/{repo}/pulls/{prNumber}/reviews — the endpoint BUG-PI-04
    // confirms is never called anywhere in this adapter today.
    return c.doReview(ctx, cred, repo, prNumber, payload)
}
```

### GitLab adapter (`gitlab/client.go`) — composed sequence, per the capability note above

```go
func (c *Client) SubmitReview(ctx context.Context, cred Credential, repo string, mrIID int32, in domain.ReviewInput) (domain.Review, error) {
    for _, comment := range in.Comments {
        if err := c.createDiscussion(ctx, cred, repo, mrIID, comment); err != nil {
            return domain.Review{}, err
        }
    }
    switch in.Type {
    case domain.ReviewTypeApprove:
        return c.approveMR(ctx, cred, repo, mrIID, in.Summary)
    case domain.ReviewTypeRequestChanges, domain.ReviewTypeComment:
        return c.noteMR(ctx, cred, repo, mrIID, in.Summary, in.Type) // GitLab has no native "request changes" state — recorded as a summary note, documented divergence
    }
    return domain.Review{}, domain.ErrCapabilityUnsupported
}
```

---

## Design — wiring (`api-gateway`)

```go
// httpgateway/pr_review_routes.go — new
// POST /v1/scm/pull-requests/{prNumber}/reviews
func (h *Handler) SubmitPullRequestReview(w http.ResponseWriter, r *http.Request) {
    var req submitReviewBody // {repoId, provider, reviewType, summary}
    // ... decode ...

    // Aggregation read, same shape as every other multi-service view per
    // 05-data-architecture.md:116-119 — annotation-service is the source
    // of truth for the comments being submitted.
    annotations, err := h.annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
        RepoId: req.RepoID, FilePath: "", // all files for this PR's diff — pagination handled by the existing ListAnnotations contract
    })
    if err != nil {
        httpError(w, err)
        return
    }
    if len(annotations.GetAnnotations()) == 0 { // BR-PI-10, checked here first — fail fast before any SCM call
        httpError(w, apperrors.New(apperrors.KindInvalidArgument, "PR_REVIEW_NO_COMMENTS", "annotate at least one line before submitting a review", nil))
        return
    }

    comments := make([]*scmintegrationv1.ReviewComment, 0, len(annotations.GetAnnotations()))
    for _, a := range annotations.GetAnnotations() {
        comments = append(comments, &scmintegrationv1.ReviewComment{
            Path: a.GetAnchor().GetFilePath(), Line: a.GetAnchor().GetLine(), Body: a.GetContent(),
        })
    }

    resp, err := h.scmClient.SubmitReview(ctx, &scmintegrationv1.SubmitReviewRequest{
        TenantId: id.TenantID, Provider: req.Provider, Repo: req.RepoID,
        PrNumber: req.PRNumber, ReviewType: req.ReviewType, // UNSPECIFIED if caller omitted it — BR-PI-11 resolved server-side too
        Summary: req.Summary, Comments: comments,
    })
    // ...
}
```

`hostedReview.submit` WS channel wraps the same composition for the
WebSocket-compat surface, matching `hostedReview.create`'s existing
registration pattern (`channels_scm.go:741-765`).

BR-PI-12 needs no special-case code: `api-gateway` already can, and per
this design should, issue `SubmitPullRequestReview` and the BL-CR-03
agent-feedback call from the same client-triggered action in parallel
(`errgroup`, mirroring `worktree.detectedList`'s aggregation idiom) —
noted here as a wiring instruction for whoever implements the client-side
"submit both" action, not a new backend RPC.

---

## Test plan

- `submit_review_test.go`: empty `Comments` returns `SCM_REVIEW_EMPTY_COMMENTS`
  before any provider call (BR-PI-10); `ReviewTypeUnspecified` resolves to
  `ReviewTypeRequestChanges` (BR-PI-11) — assert the fake provider receives
  the resolved type, not the unspecified one.
- `github/client_test.go`: `SubmitReview` builds the exact GitHub payload
  shape (`event`, `comments[].path/line/body`) against a recorded HTTP
  fixture for each of the three review types.
- `gitlab/client_test.go`: comment-then-approve sequence order asserted;
  a failure on the second discussion call does not silently continue to
  approve (partial-failure behavior explicitly tested, not assumed).
- `pr_review_routes_test.go`: zero-annotation case returns 400 without
  ever calling the fake `scmClient`; multi-page `ListAnnotations` results
  (if paginated) are fully drained before composing the review — regression
  guard against silently submitting a partial comment set.
- Contract test: `ReviewComment.path/line/body` field names/order stay
  aligned with `Anchor.file_path/line` + `Annotation.content` — a renamed
  field on either proto without the other breaks this test loudly instead
  of silently mismapping.
- `wscompat`: `hostedReview.submit` channel test using a fake
  `ScmIntegrationServiceClient`, same harness `hostedReview.create`
  already uses.

## Agent (`agent/`) impact

**None.** `SubmitReview` is a direct HTTPS call from `scm-integration-
service` to GitHub/GitLab's own REST API, using the same per-tenant OAuth
credential path every other `scm-integration-service` RPC already uses
(`scm-integration-service.md` §9) — no Dev Server Agent involvement, same
as `CreatePullRequest`/`RequestPullRequestReviewers` today.

## References

- `specs/backend-go/tdd/services/scm-integration-service.md:76-90` (§3, `SubmitReview`/`Review`/reviewer-and-reviews RPC group already sketched), `:110-123` (§4 domain model, `Review`/`PullRequest` shapes), `:139-142` (`ErrCapabilityUnsupported` degrade pattern reused for GitLab), `:166-167` (§6, "no shared base class" per-provider adapter posture)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:114-119` ("Read models / query needs across service boundaries" — the aggregation-at-the-gateway pattern this solution generalizes to a write)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:47-70` (API Gateway responsibilities, #4 response aggregation)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:14-18` (design principle 1, logical-FK/no-cross-service-data-reach rule ruling out either service composing the other's data directly)
- `docs/logic/project-integration/BL-PI-04-submit-pr-review.md:21-43` — main flow and BR-PI-10/11/12 verbatim
- `backend-go/proto/orca/annotation/v1/annotation.proto:18-34` — `Anchor`/`Annotation` field shapes `ReviewComment` mirrors
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:741-765` — `hostedReview.create`'s existing registration pattern, reused for `hostedReview.submit`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go` (doc comment) — `worktree.detectedList`'s parallel-`errgroup` aggregation idiom, the precedent for BR-PI-12's "submit both" wiring
- `specs/backend-go/bugs/logic-v1/BUG-PI-04-submit-pr-review-not-implemented.md` — problem statement and all "what's missing" findings this solution addresses

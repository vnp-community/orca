# BUG-PI-04: No RPC submits annotated diff comments as a GitHub PR Review

**Business Logic:** [BL-PI-04](../../../../docs/logic/project-integration/BL-PI-04-submit-pr-review.md) — Submit PR Review Comments lên GitHub
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** After Maya annotates a diff in Orca, there is no "Submit as GitHub Review" action backed by any backend-go RPC — her line comments stay local `Annotation` rows in `annotation-service`'s own database and never reach GitHub as a real PR review, regardless of review type (Comment/Approve/Request Changes).

---

## Spec summary

BL-PI-04 lets a user annotate a diff (BL-CR-02) and submit those comments as a GitHub PR review: pick a review type (Comment/Approve/Request Changes), map each `DiffComment` to a GitHub line-level review comment, and `POST /repos/:repo/pulls/:pr/reviews`. Business rules require at least one comment to submit (BR-PI-10), default review type "Request Changes" when comments exist (BR-PI-11), and the ability to submit the GitHub review and agent feedback simultaneously (BR-PI-12).

## What backend-go has

- Diff annotations themselves are real: `AnnotationService` (`backend-go/proto/orca/annotation/v1/annotation.proto:10-15`) has working `CreateAnnotation`/`ListAnnotations`/`UpdateAnnotation`/`DeleteAnnotation` RPCs, each anchored to `repo_id`/`file_path`/`line`/`ref` (`annotation.proto:19-24`), with real usecases (`backend-go/services/annotation-service/internal/usecase/create_annotation.go`, `list_annotations.go`, `update_annotation.go`, `delete_annotation.go`).
- PR-creation itself works: `CreatePullRequest`/`hostedReview.create` (`backend-go/proto/orca/scmintegration/v1/scmintegration.proto:14`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:741-765`) opens a real PR via the provider's API.
- Reviewer *management* (a different concept — who is requested to review, not review content) exists: `RequestPullRequestReviewers`/`RemovePullRequestReviewers` (`scmintegration.proto:34-35`), each with a real usecase and GitHub client call.

## What's missing

- **No "submit review" RPC exists anywhere.** `scmintegration.proto`'s full RPC list (lines 12-89) has no `SubmitPullRequestReview`/`CreateReview`/`CreatePullRequestReview` method — a repo-wide grep for `SubmitReview`, `CreateReview`, `PullRequestReview` across all of `backend-go/` returns hits only in unrelated files (provider adapters' generic client scaffolding, `RequestPullRequestReviewers`'s own test names) — none is a review-submission endpoint.
- **No `POST /repos/:repo/pulls/:pr/reviews` call exists** in the GitHub adapter (`backend-go/services/scm-integration-service/internal/adapter/github/client.go`) — that client implements issue listing, PR creation/merge, reviewer request/removal, and label/assignee updates, but never GitHub's Reviews API.
- **No mapping from `Annotation`/diff comments to a review-comment payload.** `AnnotationService` and `ScmIntegrationService` are two separate services with zero cross-references — nothing in either service's code reads `AnnotationService.ListAnnotations` output and turns it into a GitHub review-comment array (`path`/`line`/`body` per comment, `event: "COMMENT"|"APPROVE"|"REQUEST_CHANGES"`).
- **No review-type selection or default-to-"Request Changes" logic (BR-PI-10/11)** exists anywhere, since the submission path itself doesn't exist.
- **No "submit review + agent feedback simultaneously" composition (BR-PI-12)** — moot without the review-submission half existing.
- Do not confuse this with `hostedReview.create` (`channels_scm.go:741`) — that channel is `CreatePullRequest` (opening a brand-new PR from a branch), a completely different operation from submitting a review with comments onto an *existing* PR.

## See also

- None found — this gap is not covered by `missing-v1`/`api-v1`'s prior findings, which focus on unwired `github.*`/`git.*` WS channels rather than this specific cross-service (annotation → SCM) composition.

## References

- `backend-go/proto/orca/annotation/v1/annotation.proto:1-58` — full `AnnotationService` surface (no GitHub-submission concept)
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:12-89` — full `ScmIntegrationService` RPC list (no review-submission RPC)
- `backend-go/services/scm-integration-service/internal/adapter/github/client.go` — real GitHub REST client, no Reviews API usage
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:741-765` — `hostedReview.create` (PR creation, not review submission)
- `backend-go/services/annotation-service/internal/usecase/list_annotations.go` — annotation read path with no SCM cross-reference
- `docs/logic/project-integration/BL-PI-04-submit-pr-review.md`

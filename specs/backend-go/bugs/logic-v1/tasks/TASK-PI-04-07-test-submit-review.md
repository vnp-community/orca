# TASK-PI-04-07: Tests for `SubmitReview` usecase, both provider adapters, and gateway composition

**From Solution:** SOL-PI-04
**Priority:** P1
**Service:** `scm-integration-service` + `api-gateway`
**File:** `services/scm-integration-service/internal/usecase/submit_review_test.go` (new), `internal/adapter/github/client_test.go`, `internal/adapter/gitlab/client_test.go`, `services/api-gateway/internal/adapter/httpgateway/pr_review_routes_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_scm_test.go`
**Depends on:** TASK-PI-04-02, TASK-PI-04-03, TASK-PI-04-04, TASK-PI-04-05, TASK-PI-04-06
**Status:** `[ ]` TODO

---

## Tests to add

### `submit_review_test.go`

- Empty `Comments` returns `SCM_REVIEW_EMPTY_COMMENTS` before any provider
  call (BR-PI-10).
- `ReviewTypeUnspecified` resolves to `ReviewTypeRequestChanges` (BR-PI-11)
  — assert the fake provider receives the resolved type, not the
  unspecified one.

### `github/client_test.go`

`SubmitReview` builds the exact GitHub payload shape (`event`,
`comments[].path/line/body`) against a recorded HTTP fixture, for each of
the three review types.

### `gitlab/client_test.go`

- Comment-then-approve call order asserted.
- A failure on the second discussion call does not silently continue to
  approve — assert `approveMR`/`noteMR` are never called after a
  `createDiscussion` error.

### `pr_review_routes_test.go`

- Zero-annotation case returns 400 without ever calling the fake `scmClient`.
- Multi-page `ListAnnotations` results are fully drained before composing
  the review — regression guard against silently submitting a partial
  comment set (assert every page's annotations appear in the final
  `SubmitReview` call).

### Contract test

`ReviewComment.path/line/body` field names/order stay aligned with
`Anchor.file_path/line` + `Annotation.content` — a renamed field on either
proto without the other breaks this test loudly instead of silently
mismapping.

### `channels_scm_test.go`

`hostedReview.submit` channel test using a fake `ScmIntegrationServiceClient`
+ fake `AnnotationServiceClient`, same harness `hostedReview.create` already
uses.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/scm-integration-service/internal/usecase/... -run TestSubmitReview -v
go test ./services/scm-integration-service/internal/adapter/github/... -run TestClient_SubmitReview -v
go test ./services/scm-integration-service/internal/adapter/gitlab/... -run TestClient_SubmitReview -v
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestPrReviewRoutes -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestChannelsScm -v
go build ./... && go vet ./...
```

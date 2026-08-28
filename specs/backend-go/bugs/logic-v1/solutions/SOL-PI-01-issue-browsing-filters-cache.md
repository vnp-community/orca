# SOL-PI-01: Issue filters, 5-minute cache, backoff, and comment listing for `ListIssues`

**Resolves:** [BUG-PI-01](../BUG-PI-01-import-issues-partial.md)
**Service:** `scm-integration-service` (+ `api-gateway/wscompat` REST/WS wiring only — no new gateway logic)
**Affected files (proposed):**
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
- `backend-go/services/scm-integration-service/internal/usecase/list_issues.go`
- `backend-go/services/scm-integration-service/internal/usecase/ports.go`
- `backend-go/services/scm-integration-service/internal/usecase/list_issue_comments.go` (new)
- `backend-go/services/scm-integration-service/internal/usecase/get_linked_pull_requests_for_issue.go` (new)
- `backend-go/services/scm-integration-service/internal/adapter/postgres/issue_list_cache.go` (new)
- `backend-go/services/scm-integration-service/internal/adapter/postgres/migrations/000X_issue_list_cache.up.sql` (new)
- `backend-go/services/scm-integration-service/internal/adapter/github/client.go`, `.../gitlab/client.go` (filter params, `ListIssueComments`, backoff wrapper)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go` (new query params + `ListIssueComments` route)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go` (filter args on `github.issues`, new `github.issueComments` channel)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`scm-integration-service.md` §5 already names the exact pattern this bug is
missing: `rate_limit_cache` is described as "a hot local read to avoid a
round trip purely to check quota... [n]ot a source of truth"
(`scm-integration-service.md:154`). BR-PI-01's 5-minute issue cache is the
same shape — a short-lived, invalidatable local read in front of a live
provider call — not a second table type to invent. This solution adds
`issue_list_cache` as a sibling row in that same table, following the same
posture §5 states explicitly: "No table has an `issue_id`/`pr_id` column
meant to be queried as 'the current list of PRs'" (`scm-integration-service.md:157-160`)
— the cache is keyed by request shape (tenant/provider/repo/filter), not
treated as a queryable issue mirror, and every entry has a hard 5-minute
TTL matching BR-PI-01's literal words, not §5's "future cross-repo
search/cache" scope creep it explicitly warns against.

The missing `IssueFilter` population is a plumbing bug, not a design gap:
`usecase/ports.go`'s `IssueFilter.State` field already exists
(cited in BUG-PI-01 at `ports.go:33-35`) and `scm-integration-service.md`'s
own `ScmProvider` port sketch takes `filter IssueFilter` as a parameter
(`scm-integration-service.md:132`) — the port contract was already
filter-aware; `list_issues.go:53`'s `IssueFilter{}` literal simply never
reads the request. This solution wires the existing shape through, it does
not invent a new one.

Exponential backoff (BR-PI-03's other half) is grounded in §8's existing
non-functional posture: "proactively [throttling] instead of handling 429s
reactively" and "[s]econdary-rate-limit backoff... a separate backoff
trigger" (`scm-integration-service.md:216-219,228-230`). §8 already commits
to backoff as a design property of this service; it was simply never
implemented for the read path. This solution adds a jittered-exponential
retry wrapper around the two calls BR-PI-03 names (issue list, and any
burst the cache miss triggers), reusing the per-provider circuit breaker §8
already specifies rather than adding a second failure-handling mechanism.

**Genuine extension beyond the TDD, flagged explicitly**: `ListIssueComments`
and a linked-PR lookup are not in `scm-integration-service.md`'s §3 RPC
sketch at all — that sketch has `ListComments`/`CreateComment`/
`UpdateComment`/`DeleteComment` as one generic comment surface spanning
"issue + PR/MR" (`scm-integration-service.md:76-80`), which already covers
step 6's "view comments" gap load-bearing-ly: `ListComments` is the missing
RPC, not a brand-new one. BUG-PI-01's finding that only
`AddIssueCommentBySlug`/`UpdateIssueCommentBySlug`/`DeleteIssueCommentBySlug`
exist (no `List`) describes the *implemented* proto (`scmintegration.proto`),
which has drifted from `scm-integration-service.md`'s own §3 sketch — the
fix is to complete the RPC group the TDD already specified, named
`ListIssueCommentsBySlug` to match the existing `*BySlug` family's calling
convention (`GetWorkItemDetailsBySlug`, `AddIssueCommentBySlug`, etc. —
`scmintegration.proto` lines 58,65-67 per BUG-PI-01), not `ListComments`
verbatim. A dedicated "linked PRs for issue" RPC genuinely has no TDD
precedent (`GetPullRequestForBranch` resolves by branch, not issue,
BUG-PI-01:30) — flagged below as this solution's one true addition.

GraphQL support for filtering (BR-PI-02) is **not** needed to close this
bug: GitHub's REST `GET /repos/{repo}/issues` endpoint (already the call
`github/client.go:82-112` makes) natively accepts `state`, `assignee`,
`labels`, and `milestone` query parameters — no GraphQL round-trip
required for filtering. BR-PI-02's "support both REST and GraphQL" is
already satisfied elsewhere in the proto (Projects v2 RPCs,
`scmintegration.proto:52-67`, per BUG-PI-01:28); this solution does not
add a GraphQL path to `ListIssues` because REST already covers every
filter field the spec lists. This is a deliberate scope narrowing versus
BR-PI-02's literal text — call it out in review.

---

## Design — proto (`scmintegration.proto`)

```protobuf
message IssueFilter {
  string state = 1;       // "open" | "closed" | "all" — default "open"
  string assignee = 2;    // GitHub login / GitLab username; "" = unfiltered
  repeated string labels = 3;
  string milestone = 4;   // milestone title or number-as-string
}

message ListIssuesRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  IssueFilter filter = 4;      // NEW — was previously absent (scmintegration.proto:112-116)
  bool force_refresh = 5;      // NEW — bypasses the 5-minute cache, BR-PI-01's "refresh on demand"
}

message ListIssuesResponse {
  repeated Issue issues = 1;
  bool from_cache = 2;         // NEW — surfaces cache-hit state so the UI can show staleness, not required by BR-PI-01 but cheap and testable
  int64 cached_at_unix_ms = 3; // NEW
}

// NEW RPC — completes the *BySlug comment family (scmintegration.proto:65-67)
rpc ListIssueCommentsBySlug(ListIssueCommentsBySlugRequest) returns (ListIssueCommentsBySlugResponse);

message ListIssueCommentsBySlugRequest {
  string tenant_id = 1;
  string slug = 2;             // same slug shape GetWorkItemDetailsBySlug/AddIssueCommentBySlug take
}
message ListIssueCommentsBySlugResponse {
  repeated IssueComment comments = 1;
}

// NEW RPC — genuine addition, no TDD precedent (see rationale above)
rpc GetLinkedPullRequestsForIssue(GetLinkedPullRequestsForIssueRequest) returns (GetLinkedPullRequestsForIssueResponse);

message GetLinkedPullRequestsForIssueRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 issue_number = 4;
}
message GetLinkedPullRequestsForIssueResponse {
  repeated PullRequest pull_requests = 1;   // GitHub: parsed from issue timeline "cross-referenced" events / closing-keyword search; GitLab: related MRs endpoint
}
```

`GetLinkedPullRequestsForIssue` degrades per-provider like `GetBoardView`
already does (`scm-integration-service.md:139-142`, `ErrCapabilityUnsupported`)
for any of the 5 providers whose API has no cheap "linked PRs" query —
returns an empty list with a typed capability-unsupported reason rather
than failing the whole issue-detail view.

---

## Design — `usecase/` layer

### Filter plumbing (the actual bug)

```go
// internal/usecase/list_issues.go — fixed
func (uc *ListIssues) Execute(ctx context.Context, in ListIssuesInput) (ListIssuesOutput, error) {
    cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
    if err != nil {
        return ListIssuesOutput{}, err
    }

    cacheKey := issueCacheKey(in.TenantID, in.Provider, in.Repo, in.Filter)
    if !in.ForceRefresh {
        if cached, ok, err := uc.cache.Get(ctx, cacheKey); err == nil && ok {
            return ListIssuesOutput{Issues: cached.Issues, FromCache: true, CachedAt: cached.CachedAt}, nil
        }
        // cache-read errors are logged, never fail the request — cache is
        // an optimization, per scm-integration-service.md §5's "not a
        // source of truth" framing
    }

    issues, err := uc.backoff.Do(ctx, in.Provider, func(ctx context.Context) ([]domain.Issue, error) {
        return uc.provider.ListIssues(ctx, cred, in.Repo, in.Filter) // in.Filter, not IssueFilter{} — the fix
    })
    if err != nil {
        return ListIssuesOutput{}, err
    }

    now := time.Now().UTC()
    if err := uc.cache.Put(ctx, cacheKey, issues, now, 5*time.Minute); err != nil {
        // log, don't fail — same non-blocking posture as the cache read
    }
    return ListIssuesOutput{Issues: issues, FromCache: false, CachedAt: now}, nil
}
```

### `ports.go` additions

```go
type IssueListCache interface {
    Get(ctx context.Context, key IssueCacheKey) (CachedIssueList, bool, error)
    Put(ctx context.Context, key IssueCacheKey, issues []domain.Issue, cachedAt time.Time, ttl time.Duration) error
}

// BackoffExecutor wraps a provider call with jittered exponential retry,
// keyed per (provider, tenant) — same key shape as §8's circuit breaker so
// both mechanisms trip independently per provider, never globally.
type BackoffExecutor interface {
    Do(ctx context.Context, provider domain.ScmProvider, fn func(context.Context) ([]domain.Issue, error)) ([]domain.Issue, error)
}
```

`IssueFilter` gains no new fields beyond what `ports.go:33-35` already
declares per BUG-PI-01 — `State`, plus `Assignee`/`Labels`/`Milestone` to
be added there in the same change (currently absent, since nothing ever
populated them).

---

## Design — data model (`issue_list_cache`, Postgres, `scm-integration-service`'s own DB)

```sql
CREATE TABLE issue_list_cache (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    provider        TEXT NOT NULL,
    repo            TEXT NOT NULL,
    filter_hash     TEXT NOT NULL,      -- sha256 of the normalized IssueFilter, so distinct filter combos don't collide
    issues_json     JSONB NOT NULL,
    cached_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,  -- cached_at + 5 minutes, indexed for cheap expiry sweep
    UNIQUE (tenant_id, provider, repo, filter_hash)
);
CREATE INDEX idx_issue_list_cache_expires ON issue_list_cache (expires_at);
```

Sibling of `rate_limit_cache` (`scm-integration-service.md:154`), same
database, same "operational bookkeeping, not a copy of provider data"
framing (§5) — a 5-minute TTL row is explicitly not the kind of durable
"current list of PRs" mirror §5 rules out.

---

## Design — wiring (REST/WS)

- `scm_routes.go`: `GET /v1/scm/issues` gains `state`, `assignee`, `label`
  (repeatable), `milestone`, `refresh` query params, mapped into
  `IssueFilter`/`force_refresh`. New `GET /v1/scm/issues/{number}/comments`
  route to `ListIssueCommentsBySlug`.
- `channels_scm.go`: `github.issues` WS channel gains the same filter
  fields in its decoded args struct; new `github.issueComments` channel
  wired the same way `github.rateLimit` already is
  (`channels_scm.go:58-67` cited in BUG-PI-01).

---

## Test plan

- `list_issues_test.go`: fake `IssueTrackerProvider` records the `IssueFilter`
  it was called with — regression guard against the exact bug this solution
  fixes (asserting a non-empty filter reaches the provider call, not `IssueFilter{}`).
- Cache hit/miss/expiry: fake `IssueListCache` — a cached entry younger than
  5 minutes short-circuits the provider call; an expired one does not;
  `force_refresh=true` always bypasses regardless of freshness.
- Cache-read/write failure does not fail `Execute` — assert the provider
  call still happens and a result is still returned when `IssueListCache`
  returns an error.
- Backoff: fake provider returns a transient error N times then succeeds;
  assert `BackoffExecutor` retries with increasing (jittered) delay and
  the call ultimately succeeds; a non-transient error (4xx) is not retried.
- `ListIssueCommentsBySlug`: real GitHub/GitLab adapter test hitting a
  recorded HTTP fixture, mapped to `IssueComment` the same shape
  `AddIssueCommentBySlug`'s existing tests use.
- `GetLinkedPullRequestsForIssue`: one provider (GitHub) returns real
  results from a fixture; a provider without support returns
  `ErrCapabilityUnsupported`, and the usecase maps that to an empty list,
  not an RPC error — assert the response is still `OK` status.
- `wscompat`/`httpgateway` tests: filter query params correctly populate
  `IssueFilter`; `refresh=true` maps to `force_refresh`.

## References

- `specs/backend-go/tdd/services/scm-integration-service.md:76-80` (§3, generic `ListComments` RPC group this fix completes), `:132` (`ScmProvider.ListIssues(..., filter IssueFilter)` — filter-aware port contract already specified), `:139-142` (`ErrCapabilityUnsupported` degrade pattern reused for linked-PR lookup), `:154-160` (§5, `rate_limit_cache`'s "hot local read... not a source of truth" pattern this solution's `issue_list_cache` follows), `:216-230` (§8, backoff/circuit-breaking posture already committed to, not yet implemented for this path)
- `specs/backend-go/bugs/logic-v1/BUG-PI-01-import-issues-partial.md:17-30` (findings this solution addresses), `:36` (`ports.go:33-35` unused `IssueFilter.State`)
- `backend-go/services/scm-integration-service/internal/usecase/list_issues.go:16-58` — the hardcoded `IssueFilter{}` bug
- `backend-go/services/scm-integration-service/internal/usecase/get_rate_limit_status.go:12-16,43-51` — cache pattern precedent
- `backend-go/services/scm-integration-service/internal/adapter/postgres/rate_limit_cache.go` — sibling table's adapter shape to mirror
- `backend-go/services/scm-integration-service/internal/adapter/github/client.go:82-112` — existing REST call, filter params to add
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:58-67` — existing channel-registration pattern
- `docs/logic/project-integration/BL-PI-01-import-issues.md:39-41` — BR-PI-01/02/03 verbatim

# SOL-CR-02: Add diff-side, multi-line range, code snapshot, and sent-to-agent state to `Annotation`

**Resolves:** [BUG-CR-02](../BUG-CR-02-annotate-diff-partial.md)
**Service:** `annotation-service` (+ `api-gateway` REST/WS wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/annotation/v1/annotation.proto`
- `backend-go/services/annotation-service/internal/domain/annotation.go`
- `backend-go/services/annotation-service/internal/usecase/create_annotation.go`
- `backend-go/services/annotation-service/internal/usecase/delete_annotation.go`
- `backend-go/services/annotation-service/internal/usecase/mark_annotations_sent.go` (new)
- `backend-go/services/annotation-service/internal/usecase/ports.go`
- `backend-go/services/annotation-service/internal/adapter/postgres/` (repository + new migration)
- `backend-go/services/annotation-service/migrations/0002_annotation_side_range_sent.sql` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (`registerAnnotationChannels`)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/annotation_routes.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`annotation-service.md` §4 already anticipates this class of addition: "No
resolved/unresolved thread state... If review-thread resolution becomes a
requirement later, it's an additive `ResolvedAt`/`ResolvedBy` column, not a
redesign" (`annotation-service.md:87-90`). The same reasoning applies here —
`side`, a line range, a code snapshot, and a sent-to-agent flag are additive
columns on an already-correct single-table CRUD shape, not a redesign of the
service.

The current implementation (`domain/annotation.go:40-45`) already diverges
from the TDD's own `Anchor{FilePath, LineNumber, Ref}` + `Annotation.ProjectID`
shape (`annotation-service.md:79-85`) by using `Anchor.RepoID` instead of a
top-level `ProjectID`. That divergence predates this bug and is out of this
solution's scope — this solution extends the field set the current code
already ships (`RepoID`-keyed `Anchor`), not a `RepoID`→`ProjectID` rename,
to keep the change reviewable and traceable to BUG-CR-02's four concrete
gaps.

Two of the four gaps map directly onto the TDD:

- **`side`, multi-line range, `original_code`** are pure additive fields on
  `Anchor`/`Annotation` — no architectural decision needed beyond field
  design, matching §4's own "additive column" precedent.
- **Author-only mutation via OPA** already exists for `UpdateAnnotation`/
  `DeleteAnnotation` (`annotation-service.md:161-167`,
  `delete_annotation.go:52-57`) — the "confirm before deleting an
  already-sent comment" (BR-CR-08) gap is closed by *exposing* the missing
  `sent_to_agent` state through that same existing gate, not a new
  authorization mechanism.

One gap is a genuine, flagged extension beyond the TDD:

- **`worktree_id` scoping.** `annotation-service.md` §2/§4 scopes an
  annotation to `(repository, file path, line number, commit-or-ref)` via
  `ProjectID`/`Anchor`, with no mention of worktree identity — the TDD's
  domain was written for the general "comment on a file in a project" case,
  not specifically for BL-CR-01/03's per-worktree AI-diff review session.
  BUG-CR-02 itself identifies the concrete failure mode: two worktrees on
  the same repo, both mid-review, share `Anchor.RepoID` and can share
  `Ref` too (an uncommitted worktree's `ref` is typically its base branch,
  not a unique per-worktree value) — nothing distinguishes their comment
  sets server-side. This solution adds `Anchor.WorktreeID` (optional,
  additive) as a scope addition to the TDD, the same way `ports.go`'s
  `SCMClient` doc comment in `git-gateway-service` flagged a new outbound
  edge as "a new dependency edge... git-gateway-service.md §7's current
  dependency list... doesn't yet document, flagged here as a scope
  addition" (`git-gateway-service/internal/usecase/ports.go:347-352`) —
  same citation discipline, applied to a new *field* instead of a new
  *service edge*. No new service dependency is introduced:
  `annotation-service.md` §7 already allows a `project-service` call to
  validate `project_id`; validating `worktree_id` the same way (when
  present) is the same call shape, not a new dependency.

## Design — proto (`annotation.proto`)

```protobuf
enum Side {
  SIDE_UNSPECIFIED = 0; // non-diff comment (plain file/line note) — BR-CR-05 only applies to diff review
  SIDE_OLD = 1;
  SIDE_NEW = 2;
}

message Anchor {
  string repo_id = 1;
  string file_path = 2;
  int32 line = 3;
  string ref = 4;
  string worktree_id = 5;  // NEW — optional; scope addition, see rationale above
  int32 end_line = 6;      // NEW — 0 or == line means single-line; must be >= line (BR-CR-06)
  Side side = 7;            // NEW — BR-CR-05
}

message Annotation {
  string id = 1;
  string tenant_id = 2;
  string author_id = 3;
  Anchor anchor = 4;
  string content = 5;
  bool resolved = 6;
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
  string original_code = 9;             // NEW — BL-CR-02 DiffComment.originalCode
  bool sent_to_agent = 10;              // NEW — distinct from `resolved` (BR-CR-08)
  google.protobuf.Timestamp sent_at = 11; // NEW — nil until MarkAnnotationsSent
}

message CreateAnnotationRequest {
  Anchor anchor = 1;
  string content = 2;
  string request_id = 3;
  string original_code = 4; // NEW
}

message ListAnnotationsRequest {
  string repo_id = 1;
  string file_path = 2;
  string page_token = 3;
  int32 page_size = 4;
  string worktree_id = 5;   // NEW — optional filter, alternative to repo_id+file_path
  optional bool sent_to_agent = 6; // NEW — lets a caller ask for only-unsent (SOL-CR-03's send flow)
}

message DeleteAnnotationRequest {
  string id = 1;
  bool confirmed = 2; // NEW — BR-CR-08; see usecase section
}

// NEW RPC — bulk transition to "sent", used by SOL-CR-03's send-to-agent flow.
rpc MarkAnnotationsSent(MarkAnnotationsSentRequest) returns (MarkAnnotationsSentResponse);
message MarkAnnotationsSentRequest { repeated string ids = 1; }
message MarkAnnotationsSentResponse { repeated Annotation annotations = 1; }
```

`worktree_id` is deliberately optional on both `Anchor` and
`ListAnnotationsRequest` — existing callers that only know `repo_id`
continue to work unchanged; a caller that has a `worktree_id` (BL-CR-02/03's
actual UI context) gets tighter scoping without a breaking change to the
`repo_id`-based query path `annotation_routes.go:86-114` already serves.

## Design — domain (`domain/annotation.go`)

```go
type Side int32

const (
    SideUnspecified Side = iota
    SideOld
    SideNew
)

type Anchor struct {
    RepoID     string
    WorktreeID string // optional; empty = not worktree-scoped (existing callers)
    FilePath   string
    Line       int32
    EndLine    int32 // 0 treated as == Line; BR-CR-06
    Side       Side
    Ref        string
}

// NewAnchor's existing invariants (non-empty RepoID/FilePath, Line >= 0)
// are unchanged; add:
var ErrEndLineBeforeLine = errors.New("domain: end_line must not be before line")

// inside NewAnchor, after the existing Line < 0 check:
if endLine != 0 && endLine < line {
    return Anchor{}, ErrEndLineBeforeLine
}
```

`Side` has no validation beyond being one of the three defined values —
`SideUnspecified` is a legitimate state for a non-diff comment (a plain
file/line note outside diff-review context, which the TDD's general
`annotation-service` bounded context still supports per §1's "note anchored
to a specific file+line in a diff **or PR**"), so `NewAnchor` does not
reject it.

```go
type Annotation struct {
    ID           string
    TenantID     string
    AuthorID     string
    Anchor       Anchor
    Content      string
    OriginalCode string     // NEW
    Resolved     bool
    SentToAgent  bool       // NEW
    SentAt       *time.Time // NEW
    RequestID    string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// MarkSent returns a copy of a with SentToAgent=true, SentAt=&at — pure,
// like the rest of domain/, called from mark_annotations_sent.go.
func (a Annotation) MarkSent(at time.Time) Annotation {
    a.SentToAgent = true
    a.SentAt = &at
    return a
}
```

`NewAnnotation`'s signature grows `originalCode string` — passed through
unvalidated (empty is legitimate: a comment on a deleted/binary line has no
meaningful original-code snapshot).

## Design — usecase

- `create_annotation.go`: `CreateAnnotationInput` gains `WorktreeID`,
  `EndLine`, `Side`, `OriginalCode`, threaded into `domain.NewAnchor`/
  `domain.NewAnnotation`. `WorktreeID` presence is not itself validated here
  beyond the domain's shape check — production-readiness follow-up work can
  add a `project-service` existence check the same way `ProjectID`
  validation is scoped as optional/§7-level, not a hard requirement of this
  fix.
- `delete_annotation.go`: after the existing `uc.repo.GetAnnotation` +
  author-OPA check (`delete_annotation.go:42-58`, unchanged), add:

  ```go
  if existing.SentToAgent && !in.Confirmed {
      return apperrors.New(apperrors.KindFailedPrecondition,
          "ANNOTATION_ALREADY_SENT",
          "this comment was already sent to the agent — confirm to delete anyway",
          nil)
  }
  ```

  This is what actually closes BR-CR-08 server-side, per the bug's own
  framing ("a client cannot enforce... without inventing that state
  itself") — the state now exists (`SentToAgent`) *and* the enforcement
  point exists, so a client's delete button doesn't have to independently
  track which comments were sent; it can optimistically call delete and
  handle `ANNOTATION_ALREADY_SENT` by showing the confirm dialog, then
  retry with `confirmed=true`.
- `mark_annotations_sent.go` (new): tenant-scoped bulk update, mirroring
  `create_annotation.go`'s tenant/actor extraction pattern
  (`create_annotation.go:39-46`). Takes `ids []string`, loads each via
  `Repository.GetAnnotation`, calls `.MarkSent(now)`, persists via a new
  `Repository.MarkSent(ctx, tenantID, ids []string, sentAt time.Time) ([]domain.Annotation, error)`
  method (one `UPDATE ... WHERE id = ANY($1) AND tenant_id = $2` statement,
  not N round trips). Any id not found for the tenant is skipped, not a hard
  failure — the send-to-agent flow (SOL-CR-03) calls this after PTY
  injection already succeeded, so a partial id mismatch (e.g. a
  concurrently-deleted annotation) should not turn a successful send into
  an error response.
- `ports.go` `Repository` interface gains `MarkSent(...)` alongside the
  existing five methods.

## Design — data model (`migrations/0002_annotation_side_range_sent.sql`)

```sql
ALTER TABLE annotations
    ADD COLUMN worktree_id   TEXT,
    ADD COLUMN end_line      INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN side          SMALLINT NOT NULL DEFAULT 0, -- 0=unspecified,1=old,2=new
    ADD COLUMN original_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN sent_to_agent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN sent_at       TIMESTAMPTZ;

CREATE INDEX idx_annotations_worktree ON annotations (project_id_repo_id, worktree_id)
    WHERE worktree_id IS NOT NULL;
```

All new columns are `NOT NULL DEFAULT`-backed (except the nullable
`worktree_id`/`sent_at`, which are optional by design) so the migration is a
pure additive `ALTER TABLE`, no backfill needed — existing rows get
`side=SIDE_UNSPECIFIED`, `end_line=0` (single-line), `sent_to_agent=false`,
consistent with "this data didn't exist for old rows" rather than a
fabricated value.

## Design — wiring (`wscompat`/REST)

`registerAnnotationChannels` (`channels.go:138-220`): thread the new
`annotationAnchorArg` fields (`workTreeId`, `endLine`, `side`) and
`originalCode` through `annotation.create`; add `worktreeId`/`sentToAgent`
to `annotation.list`'s args; add `confirmed` to `annotation.delete`'s args
and request; register a new `annotation.markSent` channel calling
`MarkAnnotationsSent` (used exclusively by SOL-CR-03's send-to-agent flow,
not exposed as a standalone user action). `annotation_routes.go` gets the
mirrored REST fields on `POST/PUT/DELETE /v1/annotations{,/{id}}` and a new
`POST /v1/annotations/mark-sent`.

## Test plan

- `domain/annotation_test.go`: `NewAnchor` rejects `EndLine < Line`;
  `SideUnspecified` accepted; `Annotation.MarkSent` sets both fields and
  leaves the rest of the struct unchanged (copy semantics).
- `usecase/create_annotation_test.go`: new fields round-trip through to the
  fake repository's persisted `domain.Annotation`.
- `usecase/delete_annotation_test.go`: deleting a `SentToAgent=true`
  annotation with `Confirmed=false` returns `ANNOTATION_ALREADY_SENT`
  without calling `Repository.DeleteAnnotation`; `Confirmed=true` proceeds;
  deleting a not-yet-sent annotation with `Confirmed=false` still succeeds
  (regression guard — the new check must not require confirmation
  universally).
- `usecase/mark_annotations_sent_test.go` (new): marks all ids present,
  silently skips ids the fake repo doesn't have, returns the updated set.
- `adapter/postgres/annotation_repository_test.go`: `MarkSent` updates
  exactly the given ids in one query; migration applies cleanly against a
  `testcontainers-go` Postgres with existing seeded rows (defaults verified).
- `wscompat/channels_test.go`: `annotation.delete` forwards `confirmed`;
  `annotation.markSent` wired and forwards `ids`.

## References

- `specs/backend-go/tdd/services/annotation-service.md:79-93` (§4 domain
  model, additive-column precedent), `:161-167` (§9 author-only OPA gate
  this solution reuses), `:44-55` (§3 API surface this solution extends
  with `MarkAnnotationsSent`)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md:61-72`
  (domain invariants belong in `domain/`, typed errors not raw strings)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:347-352`
  (the "flagged scope addition" citation pattern this solution follows for
  `worktree_id`)
- `backend-go/services/annotation-service/internal/domain/annotation.go:1-113`
- `backend-go/services/annotation-service/internal/usecase/create_annotation.go:1-82`,
  `delete_annotation.go:1-67`, `ports.go:1-48`
- `backend-go/proto/orca/annotation/v1/annotation.proto:1-72`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:129-220`
- `docs/logic/code-review/BL-CR-02-annotate-diff.md:36-58`

# TASK-WF-005-02: Template sharing schema widening + `UpdateVisibility`/`GenerateShareLink`/`GetTemplateByShareToken`

**From Solution:** BE-SOL-005
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/template.go` (new fields), `backend-go/services/workflow-service/migrations/0007_template_sharing_and_library.up.sql`/`.down.sql` (new), `backend-go/proto/orca/workflow/v1/workflow.proto` (widened `WorkflowTemplate`, new RPCs + `TemplateShareView`), `backend-go/services/workflow-service/internal/usecase/update_visibility.go`, `generate_share_link.go`, `get_template_by_share_token.go` (new), `backend-go/services/workflow-service/internal/adapter/grpc/server.go`
**Depends on:** None
**Status:** `[ ]` TODO

---

## ⚠ Security review required before merge

Per BE-SOL-005's own note, matching the same standard as
`specs/backend-go/crs/v4/task-graph/solutions/BE-SOL-003-task-access-control-team-scope-and-sharing.md`'s
task share-link: `GetTemplateByShareToken` bypasses ordinary tenant-scoped
access **by design** (it must work for an unauthenticated caller who only
has a share link). This is a genuine security-sensitive surface, not a
routine RPC addition — **this task requires a security review pass before
merge**, specifically confirming:

1. `GetTemplateByShareToken`'s response message excludes every field not
   explicitly meant for public exposure (see `TemplateShareView` below).
2. `share_token` values are unguessable (cryptographically random, not a
   sequential ID or derivable from the template's own ID).
3. A revoked/regenerated share link actually stops resolving (no
   caching layer or CDN holds a stale token→template mapping past
   revocation).
4. Rate limiting or abuse protection on the unauthenticated
   `GetTemplateByShareToken` path — an open lookup RPC is a natural
   enumeration target.

Do not merge this task without an explicit security sign-off, even if all
functional tests pass.

## Context

`WorkflowTemplate` (`internal/domain/template.go:63-77`) confirmed to
have exactly 7 fields — `ID, TenantID, Name, DAGJSON, Scope,
ParentTemplateID, Version` — no `owner_id`, `description`, `tags`,
`visibility`, `share_token`, `rating_sum`/`rating_count`, `usage_count`.
Migration history confirmed: `0001_init` through `0006_template_version`
exist (`ls backend-go/services/workflow-service/migrations/`), so `0007`
is genuinely the next-free number as of this writing — re-run that `ls`
immediately before naming this task's migration file, since concurrent
work on TASK-WF-005-01/-03 in this same series does not touch migrations,
but other unrelated work might have landed a `0007` in the meantime.

Confirmed also: `workflow-service.md`'s own TDD does NOT include any of
these fields — this is flagged (per this series' own convention) as a
genuine net-new design, not a "finish what the TDD sketched" change.

## Changes to make

**1. `0007_template_sharing_and_library.up.sql`** (re-verify `0007` is
free before naming the file):

```sql
ALTER TABLE workflow.templates
  ADD COLUMN owner_id      TEXT,
  ADD COLUMN description   TEXT,
  ADD COLUMN tags          TEXT[] DEFAULT '{}',
  ADD COLUMN visibility    TEXT NOT NULL DEFAULT 'private', -- private|team|company|public
  ADD COLUMN share_token   TEXT,
  ADD COLUMN rating_sum    INT NOT NULL DEFAULT 0,
  ADD COLUMN rating_count  INT NOT NULL DEFAULT 0,
  ADD COLUMN usage_count   INT NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX idx_workflow_templates_share_token
  ON workflow.templates (share_token) WHERE share_token IS NOT NULL;
```

`0007_template_sharing_and_library.down.sql`:

```sql
DROP INDEX IF EXISTS idx_workflow_templates_share_token;
ALTER TABLE workflow.templates
  DROP COLUMN IF EXISTS usage_count,
  DROP COLUMN IF EXISTS rating_count,
  DROP COLUMN IF EXISTS rating_sum,
  DROP COLUMN IF EXISTS share_token,
  DROP COLUMN IF EXISTS visibility,
  DROP COLUMN IF EXISTS tags,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS owner_id;
```

**2. `internal/domain/template.go`** — widen `WorkflowTemplate`:

```go
type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityTeam    Visibility = "team"
	VisibilityCompany Visibility = "company"
	VisibilityPublic  Visibility = "public"
)

type WorkflowTemplate struct {
	// ...existing fields unchanged...
	OwnerID      string
	Description  string
	Tags         []string
	Visibility   Visibility
	ShareToken   string // empty until GenerateShareLink is called
	RatingSum    int32
	RatingCount  int32
	UsageCount   int32
}

// TemplateShareView is the narrow, unauthenticated-safe projection
// GetTemplateByShareToken returns — NEVER the full WorkflowTemplate. See
// this task's security-review note: any future WorkflowTemplate field
// addition must be deliberately opted into this view, not inherited by
// default, to avoid silently exposing a new field publicly.
type TemplateShareView struct {
	Name        string
	Description string
	DAGJSON     string
	Tags        []string
}
```

**3. `workflow.proto`** — widen `WorkflowTemplate` message with the same
8 fields, and add:

```protobuf
service WorkflowService {
  ...
  rpc UpdateVisibility(UpdateVisibilityRequest) returns (google.protobuf.Empty);
  rpc GenerateShareLink(GenerateShareLinkRequest) returns (GenerateShareLinkResponse);
  rpc GetTemplateByShareToken(GetTemplateByShareTokenRequest) returns (GetTemplateByShareTokenResponse); // unauthenticated — see security note
}

message UpdateVisibilityRequest { string template_id = 1; string visibility = 2; }
message GenerateShareLinkRequest { string template_id = 1; }
message GenerateShareLinkResponse { string share_token = 1; }
message GetTemplateByShareTokenRequest { string share_token = 1; }
message GetTemplateByShareTokenResponse { TemplateShareView template = 1; }

message TemplateShareView {
  string name = 1;
  string description = 2;
  string dag_json = 3;
  repeated string tags = 4;
}
```

Add `import "google/protobuf/empty.proto";` if not already present in
`workflow.proto` (confirm at implementation time).

**4. Usecases** (new files, each following `UpdateTemplate`'s existing
tenant-scoping pattern):

- `update_visibility.go`: tenant-scoped, validates the caller owns/can
  edit the template (reuse whatever ownership check `UpdateTemplate`
  already performs — `UpdateTemplate` itself does not currently check
  ownership beyond tenant match, confirm at implementation time whether
  `OwnerID` needs its own explicit check here), sets `Visibility`.
- `generate_share_link.go`: tenant-scoped, generates a cryptographically
  random token (e.g. `crypto/rand`-backed, NOT `uuid.NewString()` alone
  unless the team confirms UUIDv4's randomness is an acceptable
  share-token source — flag this choice explicitly for the security
  review), persists it to `share_token`, returns it.
- `get_template_by_share_token.go`: **deliberately NOT tenant-scoped** —
  looks up by `share_token` alone (this is the whole point of a share
  link), returns only `TemplateShareView`, never the full
  `domain.WorkflowTemplate`. This usecase must be the one place in this
  codebase's `workflow-service` that intentionally skips
  `tenant.RequireTenantID` — document this loudly in its doc comment so
  a future refactor doesn't "fix" it by adding tenant scoping back in and
  silently breaking every existing share link.

**5. `internal/adapter/grpc/server.go`** — add the 3 handlers, following
`GetExecution`'s pattern for the two tenant-scoped RPCs, and note
`GetTemplateByShareToken`'s handler must not call any
tenant-context-requiring helper.

**6. Repository (`internal/adapter/postgres/repository.go`)** — add
`UpdateVisibility`, `SetShareToken`, `GetByShareToken` (unauthenticated
lookup, no tenant filter in its WHERE clause — the one deliberate
exception in this repository).

## Test plan

- `UpdateVisibility` persists correctly; a non-owner (different tenant)
  cannot update another tenant's template's visibility.
- `GenerateShareLink` produces a token, `GetTemplateByShareToken` resolves
  it to a `TemplateShareView` — **regression test that asserts the
  response excludes every `WorkflowTemplate` field NOT on
  `TemplateShareView`** (per BE-SOL-005's own test-plan item: "fails
  loudly on a future `WorkflowTemplate` field addition forgotten in the
  share view").
- A share token for a template in tenant A resolves successfully when
  called with NO tenant context at all (proving the unauthenticated path
  actually works, not just "happens to still have a tenant somewhere").
- Security review checklist (see header) signed off.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/postgres/... -run TestRepository -v
go test ./services/workflow-service/internal/usecase/... -run "TestUpdateVisibility|TestGenerateShareLink|TestGetTemplateByShareToken" -v
```

Expected: clean build; migration applies/reverses cleanly; the
narrow-projection regression test passes; unauthenticated share-token
resolution works without a tenant context.

# SOL-WF-01: Add owner/description/tags, Clone mode, field-level inheritance deepMerge, and usage-gated version bumps to `workflow-service` templates

**Resolves:** [BUG-WF-01](../BUG-WF-01-workflow-template-sharing-fields-missing.md)
**Service:** `workflow-service` (schema/domain/usecase/proto) + `api-gateway` (REST wiring)
**Affected files (proposed):**
- `backend-go/services/workflow-service/migrations/0007_template_authoring_fields.{up,down}.sql`
- `backend-go/proto/orca/workflow/v1/workflow.proto`
- `backend-go/services/workflow-service/internal/domain/template.go`
- `backend-go/services/workflow-service/internal/domain/dag.go` (step-merge helpers)
- `backend-go/services/workflow-service/internal/usecase/create_template.go`, `update_template.go`, `resolve_template.go`
- `backend-go/services/workflow-service/internal/usecase/clone_template.go` (new)
- `backend-go/services/workflow-service/internal/usecase/ports.go` (extend `TemplateRepository`)
- `backend-go/services/workflow-service/internal/adapter/postgres/` (extend template repo + sqlc queries)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go`
- Corresponding `_test.go` files for every usecase and repo method touched
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

- `workflow-service.md` §4 names `WorkflowTemplate`'s fields as `ID`,
  `TenantID`, `Name`, `Version`, `ParentTemplateID *ID`, `Scope`, **`OwnerID`**,
  `Definition` — `OwnerID` is already part of the target domain model, just
  never added to backend-go's actual `domain.WorkflowTemplate`
  (`backend-go/services/workflow-service/internal/domain/template.go:63-77`,
  confirmed missing by BUG-WF-01's own diff against
  `backend-go/proto/orca/workflow/v1/workflow.proto:62-70`). §5's schema
  sketch also already has `description TEXT` and `owner_id UUID NOT NULL`
  columns (`workflow-service.md:150-162`) that backend-go's actual
  migrations never added (`0001_init.up.sql` → `0003_...` → `0006_...`,
  per BUG-WF-01's citation) — this solution closes that drift, it does not
  invent new columns beyond what §5 already specifies. **`tags`** is not in
  §5's sketch; adding it is a genuine extension, flagged below.
- §6 explains why template inheritance is walked with a single depth-capped
  recursive query rather than `ent`-style traversal — the deepMerge policy
  this solution adds still walks the same `ResolveChain` result
  (`resolve_template.go:64`), it only changes what happens to that result
  after the chain is fetched. No new recursive-SQL surface is introduced.
- §10 "Migration notes" explicitly requires closing TS Gap 4 "by
  construction" and treats `ExecuteAdHocStep` as unconditional public
  surface from day one — neither bears on this bug directly, but the same
  migration-notes discipline (get resumability/version semantics right the
  *first* time, not port TS's bugs) is the standard this solution's
  version-bump-policy fix is held to: TS's own spec (cited by BUG-WF-01)
  says bump only on a breaking change to a template with active usage;
  backend-go's `update_template.go:38,61` currently bumps unconditionally,
  which is the divergence being corrected here.
- `05-data-architecture.md`'s migration conventions (`golang-migrate`,
  sequential numeric prefix, mandatory working `down`, expand/contract for
  any `NOT NULL` addition) govern the new `0007_...` migration below —
  `owner_id UUID NOT NULL` cannot be added directly to a table with
  existing rows without a backfill step, addressed in the migration design.
- `02-microservices-decomposition.md` design principle 3 ("a service owns
  exactly the data it's the system of record for") is why Clone mode's
  resolution step (turning a parent chain into one flat, disconnected DAG)
  happens inside `workflow-service` itself via `ResolveTemplate`'s own
  usecase, not by asking a client to walk the chain and re-POST a copy —
  `workflow-service` is already the system of record for `ResolveChain`
  (`workflow-service.md` §6), so `CloneTemplate` is a thin usecase that
  reuses it rather than re-deriving inheritance logic in a second place.

### Flagged as genuine extensions beyond the TDD

- **`tags`** (schema, proto, domain) — not in `workflow-service.md` §5's
  column list. BUG-WF-01 requires it (`name/description/tags/scope` at
  create time) and BL-WF-03's library search needs it for tag filtering
  (SOL-WF-03 reuses this column) — added as `TEXT[]` with a GIN index,
  the standard Postgres shape for tag-array filtering, since §5 is silent
  on the type.
- **`overrides`/`inject_steps`/`remove_steps`** (schema, proto, domain) —
  §4 only says inheritance resolution happens via `ResolveTemplate`
  (§3's RPC comment: "walks parent chain, depth<=5") but does not specify
  a field-level merge grammar. BUG-WF-01's spec summary is explicit that
  Inherit mode needs `overrides`/`inject_steps`/`remove_steps` resolved via
  a `deepMerge`-shaped algorithm; this solution designs that algorithm
  (below) as the concrete shape for the gap §4 leaves open, matching how
  `resolve_template.go:35-46`'s own doc comment already frames its current
  "nearest non-empty ancestor wins" policy as "a deliberate, documented
  choice ... not an assumption" — this solution documents a *replacement*
  choice, still without contradicting §4's text (§4 never says the
  resolution must be all-or-nothing).
- **`usage_count`** (schema) — needed structurally by this bug's
  version-bump-on-breaking-change policy ("active usage" is what gates a
  bump). SOL-WF-03 also needs this exact column (trending sort,
  usage-count-on-execute) — added here rather than duplicated in both
  solutions, with SOL-WF-03 designed to reuse it.
- **`cloned_from_template_id`** (schema) — a nullable provenance pointer,
  distinct from `parent_template_id` (Clone deliberately has no live
  parent link — that's what makes it Clone, not Inherit). Not in §5;
  needed so a cloned template's origin isn't silently lost.

---

## Design — schema (migration `0007_template_authoring_fields`)

```sql
-- 0007_template_authoring_fields.up.sql
ALTER TABLE templates
  ADD COLUMN description TEXT,
  ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN owner_id UUID,                      -- backfilled below, then NOT NULL in a follow-up migration
  ADD COLUMN usage_count INT NOT NULL DEFAULT 0,
  ADD COLUMN overrides JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN inject_steps JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN remove_steps JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN cloned_from_template_id UUID REFERENCES templates(id) ON DELETE SET NULL;

-- Backfill: no owner history exists for templates created before this
-- migration (owner_id was never captured) — the only safe backfill value
-- is each row's own tenant, treated as an implicit "system"-owned
-- template. Real ownership for pre-existing rows must be reconciled by a
-- data-fix script per tenant (out of scope for this migration itself),
-- per 05-data-architecture.md's expand/contract discipline: this
-- migration only expands the shape, a later 0008 migration makes
-- owner_id NOT NULL once every row has a real value.
UPDATE templates SET owner_id = tenant_id WHERE owner_id IS NULL;

CREATE INDEX idx_templates_tags ON templates USING GIN (tags);
CREATE INDEX idx_templates_owner ON templates(tenant_id, owner_id);
```

```sql
-- 0007_template_authoring_fields.down.sql
DROP INDEX IF EXISTS idx_templates_owner;
DROP INDEX IF EXISTS idx_templates_tags;
ALTER TABLE templates
  DROP COLUMN cloned_from_template_id,
  DROP COLUMN remove_steps,
  DROP COLUMN inject_steps,
  DROP COLUMN overrides,
  DROP COLUMN usage_count,
  DROP COLUMN owner_id,
  DROP COLUMN tags,
  DROP COLUMN description;
```

`owner_id` is added nullable here (not `NOT NULL` in one shot) precisely
per `05-data-architecture.md`'s destructive/near-destructive-migration
rule — a follow-up `0008_template_owner_not_null.up.sql` tightens the
constraint only after every tenant's backfill is verified, one release
later. `CreateTemplate`'s usecase (below) always sets `owner_id` for new
rows regardless of the column's current nullability, so no new gap opens
in the meantime.

---

## Design — domain (`domain.WorkflowTemplate`)

```go
// internal/domain/template.go (extended)
type WorkflowTemplate struct {
    ID       string
    TenantID string
    Name     string
    DAGJSON  string
    Scope    Scope
    ParentTemplateID string
    Version  int32

    OwnerID     string   // required — the authoring user, workflow-service.md §4
    Description string   // optional
    Tags        []string // optional, GIN-indexed for SOL-WF-03's search

    // Inherit-mode merge instructions, applied against the resolved parent
    // chain by resolveEffectiveTemplate (see resolve_template.go below).
    // Meaningless (ignored) when ParentTemplateID is empty.
    OverridesJSON    string // map[stepId]json.RawMessage — shallow per-field merge onto that step's Config
    InjectStepsJSON  string // []domain.Step — appended after remove_steps is applied
    RemoveStepsJSON  string // []string — step ids to drop from the parent's resolved steps

    UsageCount int32 // incremented by workflow-service.Execute (SOL-WF-03), read by the version-bump policy below

    // ClonedFromTemplateID is a provenance-only pointer (Clone mode
    // deliberately has no live ParentTemplateID) — never walked by
    // ResolveChain, never affects resolution.
    ClonedFromTemplateID string
}
```

`NewWorkflowTemplate` gains an `ownerID string` parameter (new required
positional arg — a real signature change, all four call sites in
`create_template.go`/`update_template.go` update together) and a new
sentinel `ErrTemplateEmptyOwner`, mirroring `ErrTemplateEmptyTenant`'s
existing check shape (`template.go:85-87`). `OverridesJSON`/
`InjectStepsJSON`/`RemoveStepsJSON` are validated for *parseability* only
at construction (each must be valid JSON of its expected shape, or empty)
— semantic validation (do `inject_steps` reference valid dependsOn ids,
etc.) happens at `ResolveTemplate` time, once the full chain is known,
matching how `dag.Validate()` already only runs against a template's own
`DAGJSON` at construction and cross-template invariants (parent cycles)
are checked later in the usecase layer (`update_template.go:43-59`).

---

## Design — Inherit-mode deepMerge (`resolve_template.go`)

Replaces `resolveEffectiveTemplate`'s current "nearest non-empty ancestor
wins" (`resolve_template.go:88-107`) with a genuine field-level merge that
**generalizes** rather than breaks the old behavior — a chain with no
`overrides`/`inject_steps`/`remove_steps` anywhere (the only case that
existed before this change) resolves identically to before.

```go
// internal/usecase/resolve_template.go (resolveEffectiveTemplate, replaced)
//
// Walks chain root-first (chain[0] = topmost ancestor, per ResolveChain's
// existing contract, resolve_template.go:24-27) and folds each level's
// own steps/overrides/inject_steps/remove_steps onto an accumulator:
//
//   acc := chain[0].Steps  (topmost ancestor's own definition, may be empty)
//   for level := chain[1:] {
//       if level has its OWN non-empty dag_json steps:
//           acc = level's own steps   // "own steps fully replace" — same
//                                     // rule the old policy already had,
//                                     // now scoped per-level instead of
//                                     // whole-chain
//       acc = removeSteps(acc, level.RemoveStepsJSON)
//       acc = applyOverrides(acc, level.OverridesJSON)   // shallow per-field JSON merge, keyed by step id
//       acc = append(acc, level.InjectStepsJSON...)      // new steps, own dependsOn as authored
//   }
//   return acc, as the effective template's DAGJSON (validated via
//   DAGDefinition.Validate() before being returned — a bad inject/override
//   must fail ResolveTemplate cleanly, not silently produce a broken DAG)
```

`applyOverrides(steps, overridesJSON)` unmarshals `overridesJSON` as
`map[string]json.RawMessage` (step id → partial JSON object) and, for each
step whose id has an entry, merges that object's top-level keys into the
step's own `Config` (`json.RawMessage` → unmarshal to
`map[string]json.RawMessage`, overwrite matching keys, re-marshal) — a
one-level-deep merge, matching the bug's "field-level overrides" framing,
not a recursive deep-merge into nested config structures (nested-key
override is a documented non-goal, callers needing that override a whole
sub-object key instead).

`removeSteps` drops any step whose id appears in `RemoveStepsJSON`, and
also strips that id from every remaining step's `DependsOn` (a removed
step's dependents lose that edge rather than becoming permanently
unsatisfiable — same "shape stays valid" posture `DAGDefinition.Validate`
already enforces elsewhere, `dag.go:77-99`).

This function's output is validated exactly like a directly-authored
template's `DAGJSON` (`DAGDefinition.Validate()`, `dag.go:77-99` — unique
ids, no dangling `dependsOn`, no self-reference), and `ResolveTemplate`
returns a `KindInvalidArgument`/`WORKFLOW_INVALID_TEMPLATE` error if the
merged result fails validation, mirroring `resolve_template.go:80-83`'s
existing error-mapping shape for a parse failure.

---

## Design — Clone mode (`clone_template.go`, new usecase)

```go
// internal/usecase/clone_template.go
type CloneTemplateInput struct {
    SourceTemplateID       string
    Name, Description      string
    Tags                   []string
}

type CloneTemplate struct {
    resolve *ResolveTemplate     // reused as-is — see rationale above
    repo    TemplateRepository
}

func (uc *CloneTemplate) Execute(ctx context.Context, in CloneTemplateInput) (domain.WorkflowTemplate, error) {
    // 1. Resolve the SOURCE template's effective (post-inheritance) DAG —
    //    Clone snapshots what the source ACTUALLY runs today, not just its
    //    own dag_json (which, for an Inherit-mode source, may be empty or
    //    override-only and meaningless standalone).
    resolved, err := uc.resolve.Execute(ctx, ResolveTemplateInput{TemplateID: in.SourceTemplateID})
    // ... error mapping, same shape as every other usecase here ...

    // 2. Build a brand-new ROOT template (ParentTemplateID empty — Clone
    //    is explicitly a disconnected copy, workflow-service.md §4's
    //    "Constructor rejects a template naming itself as its own parent"
    //    doesn't apply here since there's no parent at all) with the
    //    resolved DAG baked in verbatim and ClonedFromTemplateID set for
    //    provenance only.
    tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, in.Name, resolved.Template.DAGJSON,
        resolved.Template.Scope, "" /* no parent */, ownerID)
    tmpl.Description = in.Description
    tmpl.Tags = in.Tags
    tmpl.ClonedFromTemplateID = in.SourceTemplateID

    return tmpl, uc.repo.CreateTemplate(ctx, tmpl)
}
```

`CreateTemplate`'s existing usecase is intentionally **not** reused
directly for this — Clone's defining property (snapshot the *resolved*
DAG, not the source's raw `dag_json`) is different enough from plain
authoring that folding it into `CreateTemplateInput` with an
`isClone bool` flag would make that one usecase do two things. A separate
usecase, sharing `ResolveTemplate` and `TemplateRepository`, keeps each
usecase doing exactly one job — matching this codebase's existing
one-usecase-per-RPC granularity (`create_template.go`, `update_template.go`,
`resolve_template.go` are each already single-purpose).

---

## Design — version-bump-on-breaking-change policy (`update_template.go`)

Current behavior bumps `templates.version` unconditionally on every
`UpdateTemplate` call (`update_template.go:38,61`). Replaced with:

```go
// internal/usecase/update_template.go (Execute, extended)
current, _ := uc.templates.GetTemplate(ctx, tenantID, in.ID)   // already fetched at line 31 — reused, not a second call
next, err := domain.NewWorkflowTemplate(...)

breaking := isBreakingChange(current, next)
bumpVersion := current.UsageCount > 0 && breaking
// Metadata-only edits (description/tags/scope) and non-breaking DAG
// edits (adding a step, adding a new optional dependsOn) never bump —
// only a breaking DAG change to an ACTIVELY-USED template does, per
// BUG-WF-01's spec summary ("automatic minor-version bumps when a
// template with active usage receives a breaking change").

updated, err := uc.templates.Update(ctx, next, in.ExpectedVersion, bumpVersion)
```

`isBreakingChange(old, new domain.WorkflowTemplate) bool` parses both
`DAGJSON`s and reports true if: any step id present in `old` is absent
from `new` (a removed step — anything downstream referencing its output
via `{{outputs.stepId...}}`, see SOL-WF-02, silently breaks), or any step
id present in both has a **different `Type`** (an `agent` step becoming a
`webhook` step under the same id is not a compatible edit). Adding new
steps, adding new `dependsOn` edges among already-compatible steps, and
any `Config`-only change are treated as non-breaking — this is a
conservative first cut (a `Config` change *can* be breaking in practice,
e.g. a `webhook` URL that other steps' interpolated outputs assume a shape
from) but keeping the detector to structural id/type changes only avoids
false-positive bumps on every prompt-wording edit, and is flagged here as
a policy choice reviewable independently of this solution's schema/proto
changes.

`TemplateRepository.Update`'s signature gains a `bumpVersion bool`
parameter; the `adapter/postgres` implementation's conditional `UPDATE`
either includes `version = version + 1` or omits it based on that flag,
still gated by the existing `WHERE version = $expected_version`
optimistic-concurrency check (`update_template.go`'s
`ErrTemplateVersionConflict` handling, lines 63-64, is unchanged).

---

## Design — proto (`workflow.proto`)

```protobuf
message WorkflowTemplate {
  string id = 1;
  string tenant_id = 2;
  string name = 3;
  string dag_json = 4;
  string scope = 5;
  string parent_template_id = 6;
  int32 version = 7;

  string owner_id = 8;
  string description = 9;
  repeated string tags = 10;
  string overrides_json = 11;
  string inject_steps_json = 12;
  string remove_steps_json = 13;
  int32 usage_count = 14;                 // read here, incremented by SOL-WF-03's Execute change
  string cloned_from_template_id = 15;
}

message CreateTemplateRequest {
  // ... existing fields unchanged ...
  string description = 6;
  repeated string tags = 7;
  string overrides_json = 8;
  string inject_steps_json = 9;
  string remove_steps_json = 10;
}

// CloneTemplate is the disconnected-copy creation path BUG-WF-01 finds
// entirely absent — a distinct RPC, not a flag on CreateTemplateRequest,
// since Clone snapshots a RESOLVED template (server-computed) rather than
// accepting caller-supplied dag_json like every other creation path.
rpc CloneTemplate(CloneTemplateRequest) returns (CloneTemplateResponse);

message CloneTemplateRequest {
  string source_template_id = 1;
  string name = 2;
  string description = 3;
  repeated string tags = 4;
}
message CloneTemplateResponse {
  WorkflowTemplate template = 1;
}
```

`UpdateTemplateRequest` gains the same `description`/`tags`/`overrides_json`/
`inject_steps_json`/`remove_steps_json` fields as `CreateTemplateRequest`
(still field-mask-free, per the existing doc comment at
`workflow.proto:195-198` — every field always sent).

---

## Design — wiring (REST)

`workflow_routes.go` (`backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go`):

- `createTemplateRequestBody` (line 44-49) gains `Description`, `Tags`,
  `OverridesJSON`, `InjectStepsJSON`, `RemoveStepsJSON`, threaded into
  `CreateTemplateRequest` at `handleCreateTemplate` (lines 62-68).
- New route: `sub.Post("/templates/{id}/clone", handleCloneTemplate(client))`
  inside `mountWorkflowRoutes` (line 26-37), following the existing
  `chi.URLParam(r, "id")` pattern already used by
  `handleGetExecution`/`handlePauseExecution` (lines 163-176, 178-191).

`wscompat`'s `workflow.*` namespace has zero channel registrations today
(confirmed by BUG-WF-01's own "See also" cross-reference to BUG-030) — this
solution's new RPCs (`CloneTemplate`, extended `CreateTemplate`/
`UpdateTemplate`) need equivalent `wscompat` wiring too, but that whole
namespace's registration is BUG-030's tracked gap, not re-designed here;
whoever picks up BUG-030 should include these new fields/RPC in that pass.

---

## Test plan

- `domain/template_test.go` — `NewWorkflowTemplate` rejects empty
  `ownerID` (new `ErrTemplateEmptyOwner`); accepts a template with
  `OverridesJSON`/`InjectStepsJSON`/`RemoveStepsJSON` set only when they
  parse as valid JSON of their expected shape; rejects malformed JSON in
  any of the three with a clear error.
- `usecase/resolve_template_test.go`:
  - A 3-level chain (company → team → personal) where only the middle
    level defines `overrides` for a step id present in the top-level
    ancestor's steps — asserts the merged `Config` reflects the override,
    the un-overridden fields survive from the ancestor.
  - `remove_steps` on the leaf drops a step AND strips it from any
    remaining step's `dependsOn` — assert the resulting DAG still passes
    `Validate()`.
  - `inject_steps` appends a new step with its own `dependsOn` referencing
    an inherited step id — assert it resolves correctly.
  - Regression: a chain with no overrides/inject/remove anywhere at any
    level resolves identically to the pre-change "nearest non-empty
    ancestor wins" behavior — pin this with a table test reusing the old
    test fixtures verbatim.
  - A merge that produces a cyclic or dangling-dependency DAG returns
    `WORKFLOW_INVALID_TEMPLATE`, not a corrupted resolved template.
- `usecase/clone_template_test.go` — cloning an Inherit-mode source (empty
  own `dag_json`, steps come entirely from its parent) produces a new root
  template whose `DAGJSON` matches the SOURCE's *resolved* steps, with
  `ParentTemplateID` empty and `ClonedFromTemplateID` set to the source id;
  a second `UpdateTemplate` on the clone never touches the original.
- `usecase/update_template_test.go`:
  - `UsageCount == 0` + a breaking DAG edit (step removed) → version does
    NOT bump.
  - `UsageCount > 0` + a breaking DAG edit → version bumps.
  - `UsageCount > 0` + a non-breaking edit (new step added, or
    description-only change) → version does NOT bump.
  - `UsageCount > 0` + a step's `Type` changed under the same id → treated
    as breaking, version bumps.
- `adapter/postgres/template_repository_test.go` — `Update` with
  `bumpVersion=false` leaves `version` unchanged while still updating
  every other column; `bumpVersion=true` increments it, both still
  respecting `expected_version`'s optimistic-concurrency `WHERE` clause.
  Migration `up`→`down`→`up` round-trip test per
  `05-data-architecture.md`'s CI convention, including the `owner_id`
  backfill `UPDATE` running cleanly against pre-existing rows.
- `httpgateway/workflow_routes_test.go` — `POST /v1/workflows/templates/{id}/clone`
  happy path + 404 on unknown `source_template_id`.

**Needs `agent/` (Dev Server Agent) changes:** No. This bug is entirely
schema/domain/usecase/proto surface inside `workflow-service` and its REST
wiring — no execution-plane interaction.

## References

- `backend-go/specs/backend-go/tdd/services/workflow-service.md:109-114` (§4 domain model, `OwnerID` already named), `:150-162` (§5 schema sketch, `description`/`owner_id` columns already named)
- `backend-go/specs/backend-go/tdd/architecture/05-data-architecture.md:58-73` (migration conventions, expand/contract for `NOT NULL`)
- `backend-go/specs/backend-go/tdd/architecture/02-microservices-decomposition.md:28-32` (design principle 3, system-of-record ownership underpinning why Clone resolves in-service)
- `backend-go/services/workflow-service/internal/domain/template.go:63-113` — current `WorkflowTemplate`/`NewWorkflowTemplate`
- `backend-go/services/workflow-service/internal/usecase/resolve_template.go:88-107` — `resolveEffectiveTemplate`, the policy replaced above
- `backend-go/services/workflow-service/internal/usecase/update_template.go:26-71` — current unconditional version bump
- `backend-go/services/workflow-service/internal/usecase/create_template.go:43-72` — current single creation path
- `backend-go/services/workflow-service/migrations/0001_init.up.sql`, `0003_template_parent_chain.up.sql`, `0006_template_version.up.sql` — full column history this solution's `0007_...` extends
- `backend-go/proto/orca/workflow/v1/workflow.proto:62-70,72-82,193-208` — `WorkflowTemplate`/`CreateTemplateRequest`/`UpdateTemplateRequest`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:25-49` — REST route table and `createTemplateRequestBody` extended above
- `specs/backend-go/bugs/logic-v1/BUG-WF-01-workflow-template-sharing-fields-missing.md` — problem statement
- `specs/backend-go/bugs/logic-v1/BUG-WF-03-workflow-sharing-not-implemented.md` — sibling bug this solution's `usage_count`/`tags` columns are designed to be reused by (see SOL-WF-03)

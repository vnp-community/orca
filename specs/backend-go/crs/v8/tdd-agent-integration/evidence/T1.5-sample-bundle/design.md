## Context

ms-system-setting already has an AI model-source CRUD built for FR-SYS-07 (`internal/domain/ai_config.go`,
`internal/usecase/ai/ai.go`, `internal/handler/ai/ai.go`, `internal/repository/mysql/ai_config.go`, table
`ai_model_source`), masked API keys through `internal/secret` (AES-GCM via `shared/pkg/secretbox`), and a flat
`ai_conversation` table (`id, user_id, source_id, content, create_time`) written by the still-mounted legacy routes
`/system/ai-conversation/add|list`. ENT-17 ("AiModelSource và AiConversation") already marks the conversation side
"hội thoại (hoãn)" — deferred. ms-case-management (`internal/usecase/ai/ai.go`) separately answers `501` for
`/functional/case/ai/chat` and `/functional/case/ai/transform` (FR-SYS-07's API list names both).

Centralized SSRF protection exists as `shared/pkg/netguard` (ADR-15, NFR-SE-05) with four profiles
(`UserTarget`, `AdminTarget`, `PoolNode`, `FixedHost`); admin-configured external destinations (LDAP, SMTP, Jira) use
`AdminTarget`, which this change reuses for AI provider calls (the provider URL is admin/organization configured, not
user-typed). Org/project scoping elsewhere in this service (`internal/middleware/task_scope.go`) resolves the acting
organization/project from header (`ORGANIZATION`/`PROJECT`) or the session's last-used org/project
(`domain.SessionUser.LastOrganizationID/LastProjectID`), checked through `shared/pkg/taskcenter.OrgAccess/ProjectAccess`;
this change follows the same resolution idiom rather than inventing a new one.

## Goals / Non-Goals

**Goals:**
- A reusable, provider-agnostic OpenAI-compatible client (`shared/pkg/aiclient`) other services can import later.
- A real `AIService` in ms-system-setting: resolve which model a chat uses, store/list conversation turns, enforce
  `AI_ENABLED` and per-organization quota before ever calling a provider.
- Keep the existing model-source CRUD, its masking and its legacy conversation routes unchanged.

**Non-Goals:**
- Claude/Bedrock support (stays `ErrProviderUnsupported`).
- AI test-case generation (CR-H02) and hiding the AI nav button (BL-J02).
- Changing `/functional/case/ai/chat` or `/functional/case/ai/transform` in ms-case-management (open question below).
- A UI for the new routes (out of scope of this backend change).

## Decisions

1. **`AI_ENABLED` is an application-level gate inside `AIService`, not a route-mount decision** (Approved Solution
   Option B). Every `/ai/conversation/*` handler calls into `AIService`, which checks `AI_ENABLED` first and returns
   a 503 `httpx` error (same shape as `secret.Seal`'s "not configured" 503) before touching the database or a
   provider. **This contradicts the project's default deferred-feature decision (ADR-12/BRL-64): a deferred feature
   is normally hidden entirely — not mounted, 404 + `X-Route-Missing: 1`, no UI, no MCP tool.** It is accepted here
   because (a) FR-SYS-07's acceptance text already documents a standing AI-specific exception to that rule (ADR-12's
   own consequence column: "Có ngoại lệ 501 đang tồn tại cho AI (OI-02)"), and (b) the Approved Solution explicitly
   chose 503-when-off over hiding the routes. Rejected alternative: mount/unmount routes from `AI_ENABLED` like
   `deferred_features.yaml` does for other deferred work — rejected because toggling `AI_ENABLED` is meant to be a
   runtime operational switch (flip and the routes react immediately), not a build-time deferred-feature flag.

2. **`ai_conversation` becomes a header, additively.** Rather than drop columns (which would break the already-mounted
   `/system/ai-conversation/add|list`), the migration only *adds* nullable columns to `ai_conversation`
   (`organization_id`, `project_id`, `model_source_id`, `title`, `update_time`) and adds a new table
   `ai_conversation_message` (`id, conversation_id, role, content, create_time`) with `conversation_id` foreign-keyed
   to `ai_conversation.id`. The legacy routes keep writing/reading `ai_conversation.content` directly and are
   untouched; the new `/ai/conversation/*` routes always go through `AIService` and `ai_conversation_message`.

3. **Scope resolution reuses the `task_scope.go` idiom, not a new middleware stack.** `AIService.ResolveModel(ctx,
   orgID, projectID)` takes ids the handler already resolves the way `RequireOrgScope`/`RequireProjectScope` do
   (header `ORGANIZATION`/`PROJECT`, else `SessionUser.LastOrganizationID/LastProjectID`); it looks up
   `ai_model_scope_config` for `project_id = projectID`, else `organization_id = orgID`, else the row with both null
   (system default), and dereferences to `ai_model_source`. Rejected alternative: a dedicated `RequireAIScope` gin
   middleware — rejected as unnecessary indirection for one service-level call.

4. **The organization usage counter is a new, simple MySQL table**, not a reuse of an existing quota mechanism: the
   knowledge graph has no quota/usage entity for AI (`kg_search` for hạn mức/quota/chi phí found none tied to
   `ai_model_source`/`ENT-17`). `ai_usage_counter(organization_id, request_count, cost_limit, updated_at)` is
   incremented inside the same transaction as `AppendTurn`; `AIService.AppendTurn` checks
   `request_count < cost_limit` (when `cost_limit` is set) before calling `aiclient` and before writing the turn, so
   an exhausted organization never reaches the provider (AC-AI-04). The exact unit (requests vs. tokens vs. a cost
   figure) is an open question below.

5. **`aiclient` uses `netguard.AdminTarget`** (not `UserTarget`): the provider's `api_url` is entered by an admin when
   declaring the model source (`ai_model_source.api_url`), the same trust level as the LDAP/SMTP/Jira destinations
   that already use `AdminTarget`.

6. **Retry follows the Jira client's `Retry-After` parsing precedent**
   (`services/infrastructure/integration/internal/client/jira/client.go: parseRetryAfter`): delta-seconds or an HTTP
   date, at most 2 retries, giving up and returning the provider's error after the second failed attempt (AC-AI-06).

## Risks / Trade-offs

- [Risk] `AI_ENABLED=off` returning 503 on mounted routes is a visible deviation from ADR-12/BRL-64's "hidden when
  deferred" rule → Mitigation: scoped narrowly to the AI exception the SRS/SAD already document (OI-02); no other
  deferred feature changes behavior.
- [Risk] Additive-only migration keeps two conversation shapes (`ai_conversation.content` for legacy rows,
  `ai_conversation_message` for new ones) live at once → Mitigation: the DTOs and handlers for each stay separate;
  `AIService` never reads `ai_conversation.content`.
- [Risk] A quota keyed only by organization could starve one project's legitimate traffic because of another
  project's usage in the same organization → Mitigation: accepted for this change (the Approved Solution specifies
  an organization-keyed counter); flagged as an open question for a future project-level sub-limit.

## Migration Plan

- `V037__ai_conversation_foundation.sql` in `backend/services/platform/system-setting/migrations/`: `ALTER TABLE
  ai_conversation ADD COLUMN` (nullable `organization_id`, `project_id`, `model_source_id`, `title`, `update_time`);
  `CREATE TABLE ai_conversation_message`; `CREATE TABLE ai_model_scope_config`; `CREATE TABLE ai_usage_counter`. All
  additive; no backfill needed (existing rows keep `organization_id`/`project_id`/`model_source_id` null and are
  never read by the new service).
- Rollback: drop the three new tables and the four added columns; the legacy routes are unaffected either way since
  they never reference the new columns/tables.

## Open Questions

- **`ResolveModel` precedence (project → organization → system default)** is what the Approved Solution specifies,
  but the knowledge graph has no requirement describing how it should interact with a user's own **personal**
  `ai_model_source` (`permission_type = PERSONAL`, already selectable today via `/ai/config/source/name/list`): does
  a personal source ever outrank the project/organization scope config, or is scope config system/org/project only
  and personal sources stay a separate, parallel choice the user makes per chat? This change implements the latter
  (personal sources untouched, `ResolveModel` only walks project → organization → system) but it needs confirmation.
- **Quota unit and reset period**: the CR says "hết hạn mức AI" (AI quota exhausted) but not whether `ai_usage_counter`
  counts requests, tokens, or a cost figure, nor whether/how it resets (never, daily, monthly). This change
  implements a request counter with a static `cost_limit` (no automatic reset) as the simplest reading of AC-AI-04;
  needs confirmation before a real cost-based limit is expected.
- **`/functional/case/ai/chat` and `/functional/case/ai/transform` (ms-case-management)** are named in FR-SYS-07's own
  API column and currently answer the same kind of 501 this CR fixes in ms-system-setting, but the Approved Solution
  text only names `ms-system-setting`'s `AIService` and `/ai/conversation/*`. This change leaves ms-case-management's
  501s as they are (`Transform` is explicitly CR-H02's test-case generation; `Chat` is undecided) — needs
  confirmation on whether `Chat` should call the new `aiclient`/`AIService` now or wait.
- **`AI_ENABLED` default value** is not stated by the CR; this change defaults it to `false` (fail closed, consistent
  with `secret.Seal`'s fail-closed-without-key pattern) unless told otherwise.

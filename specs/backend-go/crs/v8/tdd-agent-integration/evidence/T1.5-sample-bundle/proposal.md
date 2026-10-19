## Why

FR-SYS-07 (SRS §5.4, URD §7.4) requires the system to let an administrator declare AI model sources, but its own
acceptance text still mandates that chat and text-transform answer a truthful 501 because no AI backend is connected
(tracked as open item OI-02). BL-H01 (REQ-900) replaces that stub with a working AI foundation — a real OpenAI-compatible
client, a conversation service, and per-organization/project model configuration — so CR-H02 (AI test-case generation)
and the AI nav button (BL-J02) have something real to call instead of a canned 501.

## What Changes

- Add `shared/pkg/aiclient`: an OpenAI-compatible HTTP client (OpenAI, DeepSeek, ZhiPu) built on `shared/pkg/netguard`
  (`AdminTarget` profile, ADR-15), 60 s timeout, at most 2 retries honoring `Retry-After`, Prometheus metrics, and a
  key that never reaches logs, spans or metric labels (NFR-SE-04). Claude/Bedrock keep answering
  `ErrProviderUnsupported` (out of scope, per the CR).
- Add `ai_model_scope_config` (ms-system-setting): an organization/project-nullable record pointing at `ai_model_source`;
  `AIService.ResolveModel` picks project → organization → system default.
- Extend the existing `ai_conversation` row (ENT-17) into a conversation **header** and add `ai_conversation_message`
  for turns; add `AIService.ListMessages` / `AppendTurn`.
- Add an organization-keyed `ai_usage_counter`, incremented by `AppendTurn`, enforcing a per-organization quota/cost
  limit: a request against an exhausted organization is refused with a clear message and never reaches the provider
  (AC-AI-04).
- Add routes `/ai/conversation/{add,chat,update,list,delete,chat/list}` in ms-system-setting, gated by `AI_ENABLED`
  **inside `AIService`** (the routes stay mounted; off answers 503 on every call) — **BREAKING with the project's
  default deferred-feature pattern**: ADR-12/BRL-64 normally hide a deferred feature entirely (404 +
  `X-Route-Missing: 1`, no mount). This CR keeps the routes mounted and uses 503 instead, matching the Approved
  Solution (Option B). This is acceptable only because FR-SYS-07's own acceptance text already carves out an AI-specific
  exception to that rule (OI-02); see design.md.
- The existing `/system/ai-config/*`, `/personal/model/*` (model source CRUD) and legacy `/system/ai-conversation/add|list`
  routes are **not** touched by this change; they keep serving the flat `ai_conversation` rows they already write.
- A new API key stored on `ai_model_scope_config` (if a scope overrides the source's own key) is sealed/masked the
  same way as `ai_model_source.app_key` today (`internal/secret`, AES-GCM) — NFR-SE-04, AC-AI-03.

## Capabilities

### New Capabilities
- `ai-foundation`: the OpenAI-compatible `aiclient` package, `AIService` (resolve model, list/append conversation
  turns), the `AI_ENABLED` gate, organization/project model-source scope resolution, and the per-organization usage
  quota.

### Modified Capabilities
_None._ This is the project's first OpenSpec change (no `openspec/specs/` exists yet); FR-SYS-07's "answers 501"
acceptance line is superseded by this change's spec, but there is no prior OpenSpec capability file to diff against.

## Impact

- `backend/shared/pkg/aiclient` (new package).
- `backend/services/platform/system-setting`: `internal/domain/ai_config.go`, `internal/usecase/ai/`,
  `internal/handler/ai/`, `internal/repository/mysql/ai_config.go`, `internal/handler/router.go`,
  `internal/config/config.go`, `migrations/` (new `V037` migration).
- Not touched: `backend/services/qa-core/case-management` (`/functional/case/ai/chat`, `/functional/case/ai/transform`
  stay `501`; see design.md open question), `backend/services/infrastructure/integration-hub` (protected, unrelated).
- Docs: `docs/SRS.md` (FR-SYS-07, OI-02) and `docs/tdd/TDD-TMS.md` (FR-SYS-07) need their "trả 501" wording updated to
  describe the real behavior once this ships (tasked in PH-4; SAD is not touched, per project rules).

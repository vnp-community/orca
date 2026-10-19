# REQ-1234 — BL-H01 nền AI (T-F3-18)

## Why

`FR-SYS-07` today says the AI chat and transform actions "phải trả 501 trung thực" while the real model
sources are already configurable, so the product stores provider keys that nothing ever uses, and the SPA
drawer (`ui/tms/src/api/modules/ai.ts`) calls six `/ai/conversation/*` routes the gateway answers with
`404 + X-Route-Missing: 1` (`backend/services/gateway/internal/routing/default_table.go:159`,
`gone("UF-P1a-03", …)`). `ENT-17` itself records the conversation side as `hội thoại (hoãn)`. The team has
approved building the real platform (SOL-P3c-E §3.2, T-F3-18), so the honest-but-dead path becomes a working
one: a shared OpenAI-compatible client, persisted conversations, per-organisation/project model binding with
encrypted keys, and a spend ceiling so a working AI feature cannot become an unbounded bill.

The seven acceptance criteria in plain words: a chat turn against a configured OpenAI-compatible source
answers and is persisted and listable (AC-1); an approved real LLM also answers (AC-2); an API key is never
visible in the UI, an API response or a log line (AC-3); an organisation over its quota gets a clear message
and **no** provider call happens (AC-4); with `AI_ENABLED` off every `/ai/conversation/*` route answers 503
(AC-5); a provider's transient error with `Retry-After` is retried at most twice and then surfaces as a real
error (AC-6); a provider address pointing into a disallowed internal range is blocked (AC-7).

## What Changes

- **New `backend/shared/pkg/aiclient`** — one OpenAI Chat Completions implementation serving the only three
  providers the SPA can create (`Open AI`, `DeepSeek`, `ZhiPu AI`; `ui/tms/src/enums/modelEnum.ts`): 60 s
  timeout, at most 2 retries on 429/5xx/timeout honouring `Retry-After`, key only in `Authorization` and
  masked everywhere else, egress through `netguard` profile `AdminTarget` (`ADR-15`, `NFR-SE-05`), and the
  metrics `ai_requests_total{provider,model,status}`, `ai_latency_seconds`, `ai_tokens_total{direction}`
  (`NFR-OB-02`). `Claude` / `AWS Bedrock` return `ErrProviderUnsupported` ⇒ 422, with one
  `AI-PROVIDER-CLAUDE-BEDROCK` row in `tools/conformance/deferred_features.yaml` (`ADR-12`, `NFR-CO-02`).
- **Conversation persistence** — `ai_conversation` gains `title/scene/organization_id/model_id/update_time`;
  new `ai_conversation_message(id, conversation_id, role, content, create_time, tokens_in, tokens_out,
  model_id)`. `ENT-17`'s deferred conversation attributes become real.
- **New service operations** in ms-system-setting: `ResolveModel`, `ListMessages`, `AppendTurn` and the chat
  orchestration, published as `tms.identity.v1.AIService` (`Chat` for other services, `ResolveModel` masked)
  behind the caller list in `backend/shared/proto/tms/identity/v1/acl.yaml`, so no other service ever holds a
  provider key.
- **Six routes** `POST /ai/conversation/{add,chat,update}` and `GET /ai/conversation/{list,delete/:id,chat/list/:id}`,
  idempotent on the client-generated `conversationId`, owner-scoped (another user's id ⇒ 404).
- **Per-organisation/project model binding** — `ai_model_source` gains nullable `organization_id` and
  `project_id`; resolution order personal → project → organisation → system; the key stays `enc:v1:`
  AES-256-GCM and is only ever returned masked (`BRL-46`, `INT-14`, `NFR-SE-04`).
- **Quota** — new `ai_org_budget` and append-only `ai_usage_ledger`; a synchronous SQL check runs *before*
  `ResolveModel`, so an exhausted organisation gets 429 with a plain Vietnamese message and the provider is
  never called. Ledger insert and counter update share the turn's transaction (`shared/pkg/uow`).
- **`AI_ENABLED` flag** (default off) gating the subtree with 503, and the gateway entry changes from
  `gone("UF-P1a-03", …)` to a system-setting subtree with a read timeout ≥ 90 s.
- **BREAKING for the frozen route contract**: these six paths change from `404 + X-Route-Missing: 1` to
  `200/422/429/502/503`. `NFR-CO-01` allows this only through a registered UI exception, so
  `backend/specs/crs/v4/UI-CONTRACT-REGISTER.md` and the four route-decision registers are updated in this
  change. The SPA is **not** edited.

## Capabilities

### New Capabilities
- `ai-client`: the shared outbound AI provider client — protocol, timeout, retry with `Retry-After`, SSRF
  profile, key masking, metrics, and the unsupported-provider contract.
- `ai-conversation`: conversation and message persistence, the six `/ai/conversation/*` routes, their
  idempotency and ownership rules, and the `AI_ENABLED` 503 gate.
- `ai-model-config`: declaring and resolving a model source per user, project, organisation or system, with
  the API key encrypted at rest and masked in every read path.
- `ai-quota`: per-organisation token and cost ceilings per calendar month, the pre-call check and the
  append-only usage ledger.

### Modified Capabilities
<!-- `openspec/specs/` holds no published specs yet, so there is no base spec to delta against.
     The revision of FR-SYS-07's 501 wording is carried as an ADDED requirement that keeps the id in its
     name, plus the document tasks T5.3 and T5.4. -->

## Impact

**Requirements and decisions** (knowledge graph snapshot `snap-000001`):

| Id | Class | Effect of this change |
|---|---|---|
| `FR-SYS-07` | Requirement | Text and acceptance revised: the chat path stops being a 501 and becomes the real path its own acceptance column already anticipates ("khi nối AI (T-F3-18) ca kiểm thử đổi sang đường thật"). |
| `ENT-17` | Entity | `hội thoại (hoãn)` becomes implemented; tables `ai_model_source`, `ai_conversation` gain columns and a new child table. Owner stays system-setting. |
| `BRL-64` | BusinessRule | **Contradicted as written.** It forbids a 501 route for a deferred feature; the AI chat is no longer deferred, so the rule no longer applies to it — only `Claude`/`Bedrock` keep a `deferred_features.yaml` row. Proposal: revise BRL-64's AI scope, see design.md. |
| `ADR-12` | Decision | Its own consequence column names `Có ngoại lệ 501 đang tồn tại cho AI (OI-02)`; this change removes that exception. The 503 behind `AI_ENABLED` is "a service configured off", not a deferred feature — the point needing the decision owner's ruling (see open questions). |
| `NFR-CO-01` | QualityTarget | Needs a registered UI exception for the six routes' new status codes. |
| `NFR-CO-02` | QualityTarget | Same as `BRL-64`: the AI chat leaves the deferred registry. |
| `NFR-SE-04`, `INT-14`, `BRL-46` | QualityTarget / Constraint / BusinessRule | Met unchanged: the AI key is `enc:v1:` AES-256-GCM, masked in API reads, and absent from logs, spans and metric labels (AC-3). |
| `NFR-SE-05`, `ADR-15` | QualityTarget / Decision | Met: `netguard` profile `AdminTarget` is the single egress decision point (AC-7). |
| `NFR-OB-02` | QualityTarget | Three new metrics to register in `deploy/observability/metrics-catalog.yaml`. |
| `ADR-06`, `NFR-MA-01`, `NFR-MA-02` | Decision / QualityTarget | Met: three new tables, all owned by system-setting in `backend/db/ownership.yaml` (and the conformance register), added by versioned additive migrations `V036`/`V037` with no `DROP`. |
| `BRL-02` | BusinessRule | Applied: the organisation a turn is charged to is checked against the caller's membership before any usage is counted. |
| `ADR-09` | Decision | **Tension.** It names ms-integration as the only credential holder and outbound caller. Read as naming Jira and Google, LLM egress runs from ms-system-setting per SOL-P3c-E §3.2. Recorded as an open question for the decision owner. |

**Code** — new: `backend/shared/pkg/aiclient/`, `backend/shared/proto/tms/identity/v1/ai.proto`,
`backend/services/platform/system-setting/migrations/V036`–`V037`,
`.../internal/handler/ai/conversation.go`, `.../internal/usecase/ai/quota.go`,
`.../internal/repository/mysql/ai_conversation.go`, `.../internal/repository/mysql/ai_quota.go`,
`.../internal/grpcsvc/ai.go`. Modified: `.../internal/usecase/ai/ai.go`, `.../internal/domain/ai_config.go`,
`.../internal/handler/router.go`, `.../internal/config/config.go`,
`backend/services/gateway/internal/routing/default_table.go`.

**Registers and gates** — `tools/conformance/table_owners.yaml`, `tools/conformance/deferred_features.yaml`,
`tools/decom/route_classes_dec.yaml`, `tools/decom/ui_route_decisions.yaml`,
`tests/golden/ui-neutral/route_classes.json`,
`backend/services/gateway/internal/router/testdata/route_decisions.approved.yaml`,
`deploy/{dev,dev-remote,production}/mysql-init/10-users.sql`, `deploy/observability/metrics-catalog.yaml`.

**Documents** — `docs/features/08-ai-advanced/F24_ai-integration.md`,
`backend/services/platform/system-setting/specs/tdd.md`,
`backend/services/platform/system-setting/docs/data-model.md`,
`backend/specs/crs/v4/UI-CONTRACT-REGISTER.md`, `docs/product/v1/PRD_TMS_v1.0.md` (the FR-SYS-07 row). The SAD
is not touched.

## Out of Scope

- `Claude` and `AWS Bedrock` providers — they keep returning `ErrProviderUnsupported`.
- AI test-case generation and transform (CR-H02 / BL-H03).
- Hiding the navbar AI button when AI is off (BL-J02): with `AI_ENABLED=false` the user sees the real 503.
- Streaming responses: `Stream` is defined but the SPA does not stream today.

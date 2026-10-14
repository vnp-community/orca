Orca's strict parser reads only the fenced region below; everything else in this file is for a human reader.

<!-- orca:begin plan request=REQ-900 schema=1 -->
## PH-1 aiclient: an OpenAI-compatible, SSRF-safe, retrying HTTP client
> Delivers `shared/pkg/aiclient` as a standalone, importable package: it can chat against an OpenAI-compatible
> provider, refuses an internal-network target, retries a transient error honoring `Retry-After`, and never leaks the
> API key. Done when its own test suite passes in isolation, before anything in ms-system-setting depends on it.
- [ ] T1.1 Build the aiclient core: Config, Client, ChatRequest/ChatResponse, provider registry [type=feature] [h=3] [ac=AC-AI-01,AC-AI-02]
  Serves: FR-SYS-07
  Code: create `backend/shared/pkg/aiclient/client.go::Client,Config,ChatRequest,ChatResponse,ErrProviderUnsupported`
  Verify: AC-AI-01/AC-AI-02 chat round trip via `cd backend/shared/pkg && go test ./aiclient/... -run TestClient_Chat`
  Done when: a `Client` built from a `Config` (base URL, sealed key, provider name) posts an OpenAI-compatible chat-completions request and returns the parsed reply, or `ErrProviderUnsupported` for claude/bedrock.
- [ ] T1.2 Wire aiclient's transport through netguard's AdminTarget profile [type=feature] [h=2.5] [ac=AC-AI-07] [depends=T1.1]
  Serves: NFR-SE-05
  Code: create `backend/shared/pkg/aiclient/transport.go::NewHTTPClient`
  Verify: AC-AI-07 provider address resolving to a blocked internal range is refused via `cd backend/shared/pkg && go test ./aiclient/... -run TestTransport_SSRF`
  Done when: every `Client` call goes through `netguard.NewClient(netguard.FromEnv(netguard.AdminTarget), 60*time.Second)`, and a target in a blocked internal/loopback/link-local/metadata range is refused before any network write.
- [ ] T1.3 Add retry honoring Retry-After [type=feature] [h=2.5] [ac=AC-AI-06] [depends=T1.1]
  Serves: FR-SYS-07
  Code: create `backend/shared/pkg/aiclient/retry.go::Do,parseRetryAfter`
  Verify: AC-AI-06 provider with Retry-After is retried twice then fails via `cd backend/shared/pkg && go test ./aiclient/... -run TestRetry_RetryAfter`
  Done when: a transient provider error carrying `Retry-After` is retried at most 2 times, waiting the duration the header names (delta-seconds or an HTTP date) between attempts, then the provider's error is returned unchanged.
- [ ] T1.4 Add metrics and redact the API key everywhere the package can emit text [type=feature] [h=2] [ac=AC-AI-03] [depends=T1.1]
  Serves: NFR-SE-04
  Code: modify `backend/shared/pkg/aiclient/client.go::Client` (add Prometheus metrics; redact the key from every returned error, log line and metric label)
  Verify: AC-AI-03 API key stays masked everywhere it could leak via `cd backend/shared/pkg && go test ./aiclient/... -run TestClient_KeyRedaction`
  Done when: no error, metric label or log line produced by the package ever contains the configured API key in clear.
- [ ] T1.5 Run the aiclient regression suite [type=task] [h=1.5] [ac=AC-AI-03,AC-AI-06,AC-AI-07] [labels=test:regression] [depends=T1.2,T1.3,T1.4]
  Serves: FR-SYS-07
  Code: create `backend/shared/pkg/aiclient/aiclient_test.go::TestRegression`
  Verify: AC-AI-03, AC-AI-06, AC-AI-07 via `cd backend/shared/pkg && go test ./aiclient/...`
  Done when: `go test ./aiclient/...` passes with the SSRF, retry and key-redaction cases included.

## PH-2 Conversation data model, usage counter and the AI_ENABLED flag
> Delivers the storage ms-system-setting needs: `ai_conversation` becomes a header, `ai_conversation_message` holds
> turns, `ai_model_scope_config` and `ai_usage_counter` exist, and `AI_ENABLED` is a real config flag. Done when the
> repository methods round-trip against a real database and the config default is verified.
- [ ] T2.1 Add migration V037: conversation header columns, messages, scope config, usage counter [type=feature] [h=2]
  Serves: ENT-17, FR-SYS-07
  Code: create `backend/services/platform/system-setting/migrations/V037__ai_conversation_foundation.sql`
  Verify: migration applies and rolls back cleanly via `cd backend/services/platform/system-setting && go test ./migrations/... -run TestMigrations`
  Done when: `ai_conversation` gains nullable `organization_id, project_id, model_source_id, title, update_time`; new tables `ai_conversation_message`, `ai_model_scope_config`, `ai_usage_counter` exist; nothing already stored in `ai_conversation` is altered or dropped.
- [ ] T2.2 Add the new domain types [type=feature] [h=2] [depends=T2.1]
  Serves: ENT-17
  Code: modify `backend/services/platform/system-setting/internal/domain/ai_config.go::AIConversation` (add the new header fields) and add `AIConversationMessage`, `AIModelScopeConfig`, `AIUsageCounter`
  Verify: `cd backend/services/platform/system-setting && go build ./...`
  Done when: the three new domain types map 1:1 to the V037 tables and `AIConversation` carries its new header fields.
- [ ] T2.3 Add the AI_ENABLED config flag [type=feature] [h=1.5]
  Serves: FR-SYS-07
  Code: modify `backend/services/platform/system-setting/internal/config/config.go::Config` (add `AI_ENABLED bool`, loaded as `getEnv("AI_ENABLED", "false") == "true"`, same shape as `NOTIFICATION_SSE_ENABLED`)
  Verify: `cd backend/services/platform/system-setting && go test ./internal/config/...`
  Done when: `Config.AI_ENABLED` defaults to `false` and reads `AI_ENABLED=true` from the environment.
- [ ] T2.4 Add the conversation repository [type=feature] [h=3] [depends=T2.1,T2.2]
  Serves: ENT-17
  Code: create `backend/services/platform/system-setting/internal/repository/mysql/ai_conversation.go::AIConversationRepository` (AppendMessage, ListMessages, GetScopeConfig, IncrementUsage, GetUsage)
  Verify: scope-config precedence via `cd backend/services/platform/system-setting && go test ./internal/repository/mysql/... -run TestAIConversationRepository`
  Done when: each method round-trips through the V037 tables, and `GetScopeConfig` returns the project row, else the organization row, else the system-default row (both ids null).
- [ ] T2.5 Run the data-model regression suite [type=task] [h=1.5] [labels=test:regression] [depends=T2.3,T2.4]
  Serves: FR-SYS-07
  Code: create `backend/services/platform/system-setting/internal/repository/mysql/ai_conversation_test.go`
  Verify: `cd backend/services/platform/system-setting && go test ./internal/repository/mysql/... ./internal/config/...`
  Done when: both suites pass, covering the new tables, the `AI_ENABLED` default and the scope-config precedence order.

## PH-3 AIService: resolve model, append/list turns, enforce quota and the gate
> Delivers `AIService`, the one place that resolves a model, calls `aiclient`, persists a turn, enforces the
> organization quota and the `AI_ENABLED` gate. Done when its own tests pass against the PH-1/PH-2 pieces, before any
> route exists.
- [ ] T3.1 Add AIService.ResolveModel [type=feature] [h=3]
  Serves: FR-SYS-07
  Code: create `backend/services/platform/system-setting/internal/usecase/ai/service.go::AIService,ResolveModel`
  Verify: project scope resolves ahead of organization and system default via `cd backend/services/platform/system-setting && go test ./internal/usecase/ai/... -run TestAIService_ResolveModel`
  Done when: `ResolveModel(ctx, orgID, projectID)` returns the `ai_model_source` of the project's scope-config row, else the organization's, else the system default, via `GetScopeConfig`.
- [ ] T3.2 Add AIService.AppendTurn with quota enforcement [type=feature] [h=3] [ac=AC-AI-04] [depends=T3.1]
  Serves: FR-SYS-07
  Code: modify `backend/services/platform/system-setting/internal/usecase/ai/service.go::AIService.AppendTurn`
  Verify: AC-AI-04 exhausted organization quota refuses the chat without calling the provider via `cd backend/services/platform/system-setting && go test ./internal/usecase/ai/... -run TestAIService_AppendTurn_Quota`
  Done when: `AppendTurn` reads the organization's `ai_usage_counter`, refuses with a clear quota-exhausted `httpx` error before calling `aiclient` once the limit is reached, and otherwise resolves the model, calls `aiclient`, stores the turn and increments the counter in one transaction.
- [ ] T3.3 Add AIService.ListMessages [type=feature] [h=2] [ac=AC-AI-01] [depends=T3.1]
  Serves: FR-SYS-07
  Code: modify `backend/services/platform/system-setting/internal/usecase/ai/service.go::AIService.ListMessages`
  Verify: AC-AI-01 listing half via `cd backend/services/platform/system-setting && go test ./internal/usecase/ai/... -run TestAIService_ListMessages`
  Done when: `ListMessages` returns a conversation's turns in order, including a turn just written by `AppendTurn`.
- [ ] T3.4 Gate AIService on AI_ENABLED [type=feature] [h=2] [ac=AC-AI-05] [depends=T3.2,T3.3]
  Serves: FR-SYS-07
  Code: modify `backend/services/platform/system-setting/internal/usecase/ai/service.go::AIService` (check `config.AI_ENABLED` first in every exported method)
  Verify: AC-AI-05 AI disabled answers 503 on every conversation route via `cd backend/services/platform/system-setting && go test ./internal/usecase/ai/... -run TestAIService_Disabled`
  Done when: every `AIService` method returns the same `503` `httpx` error when `AI_ENABLED` is false, before touching the database or `aiclient`.
- [ ] T3.5 Run the AIService regression suite [type=task] [h=2] [ac=AC-AI-02,AC-AI-06,AC-AI-07] [labels=test:regression] [depends=T3.4]
  Serves: FR-SYS-07
  Code: create `backend/services/platform/system-setting/internal/usecase/ai/service_test.go`
  Verify: AC-AI-02, AC-AI-06, AC-AI-07 driven through `AIService` via `cd backend/services/platform/system-setting && go test ./internal/usecase/ai/...`
  Done when: the suite passes, including a real-provider-success case, a Retry-After case and an SSRF-blocked case, all driven through `AIService` rather than `aiclient` directly.

## PH-4 Conversation routes, key masking and documents
> Delivers the six `/ai/conversation/*` routes, confirms the scope-config API key is masked the way `ai_model_source`
> already is, and updates the two documents whose text this change makes false. Done when the full acceptance list
> (AC-AI-01 through AC-AI-07) passes at the route level.
- [ ] T4.1 Add the conversation handler [type=feature] [h=3]
  Serves: FR-SYS-07
  Code: create `backend/services/platform/system-setting/internal/handler/ai/conversation.go::ConversationHandler` (Add, Chat, Update, List, Delete, ChatList)
  Verify: `cd backend/services/platform/system-setting && go build ./...`
  Done when: each handler binds its request, calls the matching `AIService` method and returns its `httpx` error (including the 503) unchanged.
- [ ] T4.2 Mount /ai/conversation/* [type=feature] [h=1.5] [ac=AC-AI-01,AC-AI-05] [depends=T4.1]
  Serves: FR-SYS-07
  Code: modify `backend/services/platform/system-setting/internal/handler/router.go` (mount `add`, `chat`, `update`, `list`, `delete`, `chat/list` under `/ai/conversation`)
  Verify: AC-AI-01, AC-AI-05 via `cd backend/services/platform/system-setting && go test ./internal/handler/... -run TestRouter_AIConversation`
  Done when: the six routes are reachable to any signed-in user, and are registered in `route_authz.json`/covered by `route_shape_test.go` the way every other mounted route is.
- [ ] T4.3 Mask the scope config's own API key [type=feature] [h=2] [ac=AC-AI-03] [depends=T4.1]
  Serves: NFR-SE-04
  Code: modify `backend/services/platform/system-setting/internal/domain/ai_config.go::AIModelScopeConfigDTO` (mask its API key field through `secret.Show`/`secret.Resolve`, the same way `AIModelSourceDTO` does)
  Verify: AC-AI-03 via `cd backend/services/platform/system-setting && go test ./internal/handler/ai/... -run TestConversationHandler_KeyMasking`
  Done when: a scope config with its own API key never returns it in clear from any `/ai/conversation/*` or scope-config response.
- [ ] T4.4 Update SRS.md and TDD-TMS.md for FR-SYS-07 [type=task] [h=1.5]
  Serves: FR-SYS-07
  Code: modify `docs/SRS.md` (FR-SYS-07 row, OI-02 row) and `docs/tdd/TDD-TMS.md` (FR-SYS-07 statement and acceptance rows)
  Verify: manual review — the "trả 501" wording no longer describes current behavior
  Done when: FR-SYS-07's statement/acceptance and OI-02's status in both documents describe the `AI_ENABLED`-gated real behavior instead of the 501 stub; SAD is not touched.
- [ ] T4.5 Run the full acceptance suite [type=task] [h=2] [ac=AC-AI-01,AC-AI-02,AC-AI-03,AC-AI-04,AC-AI-05,AC-AI-06,AC-AI-07] [labels=test:regression] [depends=T4.2,T4.3]
  Serves: FR-SYS-07
  Code: create `backend/services/platform/system-setting/internal/handler/ai/conversation_test.go`
  Verify: AC-AI-01 through AC-AI-07 end to end via `cd backend/services/platform/system-setting && go test ./...`
  Done when: every acceptance scenario of this change passes at the route level in one run.
<!-- orca:end plan -->

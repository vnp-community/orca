# Tasks — REQ-1234 BL-H01 nền AI

Five phases, each reviewable and releasable on its own. PH-1 lands an unused shared package. PH-2 lands
additive tables and the resolution/persistence service operations. PH-3 mounts the six routes with
`AI_ENABLED` off, so they answer 503 while the gateway still declares the subtree gone. PH-4 adds the quota.
PH-5 opens the gateway subtree, moves the registers and documents, and runs acceptance.

<!-- orca:begin plan request=REQ-1234 schema=1 -->
## PH-1 Shared `aiclient`: OpenAI-compatible provider client, guarded egress, bounded retry
> Delivers `backend/shared/pkg/aiclient` as a self-contained package with no caller yet: `Chat`/`Stream`, the single OpenAI Chat Completions implementation for `Open AI`/`DeepSeek`/`ZhiPu AI`, `ErrProviderUnsupported` for Claude and Bedrock, netguard `AdminTarget` egress, 60 s deadline, at most two `Retry-After`-aware retries, masked keys and the three AI metrics. Done when the package's unit tests pass and nothing else in the repo imports it yet.

- [ ] T1.1 Create the aiclient package with Client, Provider, ChatRequest/ChatResponse and the OpenAI Chat Completions call [type=feature] [h=5]
  Serves: FR-SYS-07, NFR-SE-05
  Code: create `backend/shared/pkg/aiclient/aiclient.go::Client`
  Verify: "A configured OpenAI-compatible provider answers a chat request" via `go test -C backend/shared/pkg ./aiclient/... -run TestChat`
  Done when: `Chat` posts to `<BaseURL>/chat/completions` with a Bearer key and returns the assistant content plus the provider-reported input and output token counts.

- [ ] T1.2 Return ErrProviderUnsupported for Claude and AWS Bedrock without any stub path [type=feature] [h=1.5] [depends=T1.1]
  Serves: FR-SYS-07, NFR-CO-02, ADR-12
  Code: create `backend/shared/pkg/aiclient/provider.go::New`
  Verify: "Claude is requested" via `go test -C backend/shared/pkg ./aiclient/... -run TestNewUnsupportedProvider`
  Done when: `New` returns `ErrProviderUnsupported` for `Claude` and `AWS Bedrock`, opens no connection, and the three SPA provider names all resolve to the one OpenAI implementation.

- [ ] T1.3 Build the HTTP client through netguard AdminTarget and the telemetry untrusted-target transport [type=feature] [h=3] [ac=AC-7] [depends=T1.1]
  Serves: NFR-SE-05, ADR-15
  Code: create `backend/shared/pkg/aiclient/transport.go::newGuardedClient`
  Verify: "AC-7 a provider address in a disallowed internal range is blocked" via `go test -C backend/shared/pkg ./aiclient/... -run TestGuardedTargetBlocked`
  Done when: the client is built with `netguard.NewClientFrom(netguard.FromEnv(netguard.AdminTarget), base, 60s)` wrapped by `telemetry.WrapTransport(..., WithUntrustedTarget())`, a loopback or metadata target is refused with netguard's own fixed text, and the test reaches a private target only via `netguard.AllowLoopbackForTests`.

- [ ] T1.4 Retry at most twice on 429, 5xx and timeout, honouring a capped and jittered Retry-After [type=feature] [h=4] [ac=AC-6] [depends=T1.1]
  Serves: FR-SYS-07
  Code: create `backend/shared/pkg/aiclient/retry.go::doWithRetry`
  Verify: "AC-6 two retries honouring `Retry-After` then a real error" via `go test -C backend/shared/pkg ./aiclient/... -run TestRetryAfter`
  Done when: a provider that always answers a transient error with `Retry-After` is called exactly three times and the provider's real error is returned, a single 429 is recovered by one retry, and a cancelled context stops the wait immediately.

- [ ] T1.5 Mask the provider key everywhere but the Authorization header and export the three AI metrics [type=feature] [h=3] [ac=AC-3] [depends=T1.1]
  Serves: NFR-SE-04, NFR-OB-02
  Code: create `backend/shared/pkg/aiclient/metrics.go::observe`
  Verify: "A provider error is logged" via `go test -C backend/shared/pkg ./aiclient/... -run TestKeyMasked`
  Done when: no log line, error string, span attribute or metric label carries the key, and `ai_requests_total{provider,model,status}`, `ai_latency_seconds` and `ai_tokens_total{direction}` are exported and added to `deploy/observability/metrics-catalog.yaml`.

- [ ] T1.6 Run the aiclient unit suite for the PH-1 scenarios [type=task] [h=1] [ac=AC-6,AC-7] [labels=test:regression] [depends=T1.5]
  Serves: FR-SYS-07, NFR-SE-05
  Code: create `backend/shared/pkg/aiclient/aiclient_test.go::TestAIClientSuite`
  Verify: every PH-1 scenario of `specs/ai-client/spec.md` via `go test -C backend/shared/pkg ./aiclient/... -count=1`
  Done when: the whole package suite passes with `-count=1` and the no-stub grep for a no-op AI client or an "AI service not configured" placeholder returns nothing.

## PH-2 Data model, model resolution and the internal AIService contract
> Delivers the additive migration for conversations, messages and the per-organisation/project binding of `ai_model_source`, the repository and domain types, `ResolveModel`/`ListMessages`/`AppendTurn`, and the `AIService` gRPC contract guarded by `svcauth`. Done when the migration applies forward on a copy of the dev schema, the service operations are unit-tested, and the extended SEC-07 secret test passes for the AI key.

- [ ] T2.1 Add the additive migration for ai_conversation columns, ai_conversation_message and the org/project binding [type=feature] [h=3]
  Serves: ENT-17, NFR-MA-02
  Code: create `backend/services/platform/system-setting/migrations/V036__ss_ai_conversation_v2.sql`
  Verify: "A chat turn stores both messages" via `go test -C backend/services/platform/system-setting ./migrations/... -run TestMigrationV036`
  Done when: `ai_conversation` has `title`, `scene`, `organization_id`, `model_id`, `update_time` and `idx_aiconv_user`, `ai_conversation_message` exists with its `idx_aimsg_conv`, `ai_model_source` has nullable `organization_id` and `project_id`, the legacy `content` column is untouched, and the file contains no `DROP`.

- [ ] T2.2 Register the new table in both ownership registers and add the grants [type=task] [h=1.5] [depends=T2.1]
  Serves: NFR-MA-01, NFR-MA-04
  Code: modify `tools/conformance/table_owners.yaml`
  Verify: "The legacy conversation route is untouched" via `bash tools/conformance/run.sh`
  Done when: `ai_conversation_message` is owned by system-setting in both `tools/conformance/table_owners.yaml` and `backend/db/ownership.yaml`, the three `deploy/{dev,dev-remote,production}/mysql-init/10-users.sql` files grant the system-setting user on it, and the conformance table-ownership check passes.

- [ ] T2.3 Extend the AI domain types and add the conversation and message repository [type=feature] [h=4] [depends=T2.1]
  Serves: ENT-17, FR-SYS-07
  Code: create `backend/services/platform/system-setting/internal/repository/mysql/ai_conversation.go::AIConversationRepo`
  Verify: "A chat turn stores both messages" via `go test -C backend/services/platform/system-setting ./internal/repository/... -run TestAIConversationRepo`
  Done when: `backend/services/platform/system-setting/internal/domain/ai_config.go` carries the new conversation and message fields, and the repository can upsert a conversation by client id, append a message, list a user's conversations newest-first and list a conversation's messages in ascending time order.

- [ ] T2.4 Implement ResolveModel with the personal to project to organisation to system order and TrimSpace owner types [type=feature] [h=5] [ac=AC-3] [depends=T2.3]
  Serves: FR-SYS-07, BRL-46, INT-14, NFR-SE-04
  Code: modify `backend/services/platform/system-setting/internal/usecase/ai/ai.go::AIConfigUsecase`
  Verify: "The narrowest binding wins" via `go test -C backend/services/platform/system-setting ./internal/usecase/ai/... -run TestResolveModel`
  Done when: only `status` enabled sources resolve, the order is personal then project then organisation then system, `ownerType` matches after `TrimSpace` so the SPA's `SYSTEM ` works, `adv_settings` entries with `enable=true` become the provider params, the key is opened with the existing `secret` helper and never logged, and no resolvable source yields the 422 "chưa cấu hình mô hình AI" error.

- [ ] T2.5 Implement ListMessages and AppendTurn on the AI usecase [type=feature] [h=3] [depends=T2.3]
  Serves: FR-SYS-07, ENT-17
  Code: modify `backend/services/platform/system-setting/internal/usecase/ai/ai.go::AIConfigUsecase`
  Verify: "A chat turn stores both messages" via `go test -C backend/services/platform/system-setting ./internal/usecase/ai/... -run TestAppendTurn`
  Done when: `ListMessages` returns a conversation's turns in ascending time order for its owner only, and `AppendTurn` writes the `user` and `assistant` rows with the token counts and model id and refreshes the conversation's title and update time.

- [ ] T2.6 Publish the AIService gRPC contract restricted by the identity acl.yaml caller list [type=feature] [h=4] [depends=T2.4,T2.5]
  Serves: FR-SYS-07, NFR-SE-04, ADR-09
  Code: create `backend/shared/proto/tms/identity/v1/ai.proto::AIService`
  Verify: "An unauthorised caller cannot resolve a model" via `go test -C backend/services/platform/system-setting ./internal/grpcsvc/... -run TestAIServiceAuth`
  Done when: `Chat`, `ResolveModel`, `ListMessages` and `AppendTurn` are served from a new `backend/services/platform/system-setting/internal/grpcsvc/ai.go`, a `tms.identity.v1.AIService` document in `backend/shared/proto/tms/identity/v1/acl.yaml` names the allowed callers for each method (parsed by `grpcx.ParseACL`, `backend/shared/pkg/grpcx/acl.go`), `ResolveModel` returns the provider metadata with the key masked, and a caller outside the list is rejected with no key in the response.

- [ ] T2.7 Extend the SEC-07 secret test to the AI model-source key and run it [type=task] [h=2] [ac=AC-3] [labels=test:regression] [depends=T2.6]
  Serves: NFR-SE-04, INT-14, BRL-46
  Code: modify `tests/integrated/api/security/test_sec07_secrets.py`
  Verify: "AC-3 the key is not in any API response" and "AC-3 the key is not in logs, spans or audit records" via `python -m pytest tests/integrated/api/security/test_sec07_secrets.py -k ai_model_source`
  Done when: the test saves an AI model source with a known key, reads it back through the system, organisation, project and personal routes, inspects the stored row, the service logs and the operation-log rows, and fails if the clear key appears anywhere.

## PH-3 The six `/ai/conversation/*` routes behind `AI_ENABLED`
> Delivers the routes in ms-system-setting with the flag defaulting to off, so this phase can be released while the gateway still answers the subtree as gone: internally the routes answer 503, and with the flag on they work against the local stack's mock provider. Done when the handler tests pass against `tests/local-stack/mock-ext` and the 503 gate is proven for all six paths.

- [ ] T3.1 Add the AI_ENABLED flag and the subtree middleware that answers 503 when it is off [type=feature] [h=3] [ac=AC-5]
  Serves: FR-SYS-07, NFR-CO-01
  Code: modify `backend/services/platform/system-setting/internal/config/config.go::Config`
  Verify: "AC-5 every conversation route answers 503 when AI is off" via `go test -C backend/services/platform/system-setting ./internal/middleware/... -run TestAIEnabledGate`
  Done when: `AI_ENABLED` is read with default false into `Config`, a new `backend/services/platform/system-setting/internal/middleware/aienabled.go::RequireAIEnabled` returns 503 with a message for the whole `/ai/conversation` group, and no handler reaches model resolution, the quota or a provider while it is off.

- [ ] T3.2 Implement add, update, list and delete with client-id idempotency and owner scoping [type=feature] [h=5] [ac=AC-1] [depends=T3.1]
  Serves: FR-SYS-07, ENT-17
  Code: create `backend/services/platform/system-setting/internal/handler/ai/conversation.go::ConversationHandler`
  Verify: "`add` and `chat` race on the same client id" and "Another user's conversation is not visible" via `go test -C backend/services/platform/system-setting ./internal/handler/ai/... -run TestConversationCRUD`
  Done when: `add` creates the conversation with the client's `conversationId` when it matches `^[A-Za-z0-9_-]{8,64}$`, returns that same id and calls no provider; `update` caps the title at 255; `list` returns `[]` and never `null`, newest first; and an unknown or another user's id answers 404 on each of the four routes.

- [ ] T3.3 Implement the chat turn: resolve, assemble history, call aiclient, persist, return a plain string [type=feature] [h=5] [ac=AC-1] [depends=T3.2]
  Serves: FR-SYS-07, NFR-SE-04
  Code: modify `backend/services/platform/system-setting/internal/handler/ai/conversation.go::ConversationHandler`
  Verify: "AC-1 a chat turn is answered, stored and listable" via `go test -C backend/services/platform/system-setting ./internal/handler/ai/... -run TestConversationChat`
  Done when: the handler resolves the model, sends at most the 20 most recent turns plus the prompt through `aiclient.Chat`, stores both messages via `AppendTurn`, returns the answer as a plain string in the envelope's `data`, upserts an unknown conversation id, and maps the failures to 422 for an unsupported provider or no configured model and 502 with the provider's masked message otherwise — never 501 and never a fabricated 200.

- [ ] T3.4 Implement chat/list and mount the six routes on the router [type=feature] [h=3] [ac=AC-1] [depends=T3.3]
  Serves: FR-SYS-07, ENT-17
  Code: modify `backend/services/platform/system-setting/internal/handler/router.go`
  Verify: "AC-1 the conversation appears in the conversation list" via `go test -C backend/services/platform/system-setting ./internal/handler/... -run TestAIConversationRoutes`
  Done when: `GET /ai/conversation/chat/list/:id` returns the turns as `{id,conversationId,type,timestamp,content}` ascending, all six paths are mounted behind authentication and `RequireAIEnabled`, the legacy `/system/ai-conversation/*` routes still answer as before, and the route-shape and authz-guard tests of the service pass.

- [ ] T3.5 Add the handler integration tests against the local stack mock provider and its failure control plane [type=task] [h=4] [ac=AC-1,AC-5] [labels=test:regression] [depends=T3.4]
  Serves: FR-SYS-07, NFR-CO-01
  Code: create `backend/services/platform/system-setting/internal/handler/ai/conversation_test.go::TestConversationAgainstMockExt`
  Verify: every scenario of `specs/ai-conversation/spec.md` except AC-2 via `go test -C backend/services/platform/system-setting ./internal/handler/ai/... -count=1`
  Done when: the suite drives `tests/local-stack/mock-ext` `/llm/v1/chat/completions` for the happy path and its `__fail` control plane for the 429-with-`Retry-After` and 5xx paths, asserts 503 on all six routes with the flag off, and passes with `-count=1`.

## PH-4 Per-organisation quota, cost ceiling and usage ledger
> Delivers the quota tables, the pre-call check that refuses an exhausted organisation with 429 before any provider call, and the ledger written in the turn's transaction. Releasable on its own: with no quota rows the platform default applies and the behaviour of PH-3 is unchanged. Done when the quota unit and integration tests pass, including the proof that no provider request leaves the service.

- [ ] T4.1 Add the quota and ledger migration and register the tables and grants [type=feature] [h=3]
  Serves: NFR-MA-01, NFR-MA-02
  Code: create `backend/services/platform/system-setting/migrations/V037__ss_ai_quota.sql`
  Verify: "A quota row and a default" via `go test -C backend/services/platform/system-setting ./migrations/... -run TestMigrationV037`
  Done when: `ai_org_budget(organization_id, period, token_limit, cost_limit_micro)` and the append-only `ai_usage_ledger` exist with no `DROP`, both are owned by system-setting in `tools/conformance/table_owners.yaml` and `backend/db/ownership.yaml`, and the three `deploy/{dev,dev-remote,production}/mysql-init/10-users.sql` files grant the system-setting user on them.

- [ ] T4.2 Add the static price table and the pre-call token estimator [type=feature] [h=3] [depends=T4.1]
  Serves: FR-SYS-07, ENT-17
  Code: create `backend/services/platform/system-setting/internal/usecase/ai/pricing.go::PriceFor`
  Verify: "AC-4 the cost ceiling refuses the turn as the token limit does" via `go test -C backend/services/platform/system-setting ./internal/usecase/ai/... -run TestPricing`
  Done when: a per-provider, per-model price table in micro-units is checked in, an unknown model falls back to the most expensive known price for that provider, and the estimator bounds a turn by the prompt length plus `AI_MAX_OUTPUT_TOKENS`.

- [ ] T4.3 Check the quota before model resolution and refuse an exhausted organisation with 429 [type=feature] [h=4] [ac=AC-4] [depends=T4.2]
  Serves: FR-SYS-07, ENT-17
  Code: create `backend/services/platform/system-setting/internal/usecase/ai/quota.go::QuotaCheck`
  Verify: "AC-4 an exhausted organisation gets a clear message and no provider call" via `go test -C backend/services/platform/system-setting ./internal/usecase/ai/... -run TestQuotaCheck`
  Done when: `QuotaCheck(orgID, estimate)` runs before `ResolveModel` on the chat path, reads the current calendar month's counter, returns 429 with a plain Vietnamese exhausted-quota message for a token or cost breach, decrypts no key, issues no provider request, and applies the platform default when the organisation has no quota row.

- [ ] T4.4 Write the ledger row and the period counter in the same transaction as the turn [type=feature] [h=4] [depends=T4.3]
  Serves: ENT-17, FR-SYS-07
  Code: modify `backend/services/platform/system-setting/internal/usecase/ai/ai.go::AIConfigUsecase`
  Verify: "A successful turn is stored and metered atomically" via `go test -C backend/services/platform/system-setting ./internal/usecase/ai/... -run TestAppendTurnLedger`
  Done when: `AppendTurn` opens one `shared/pkg/uow` transaction that inserts the two message rows, inserts the ledger row with the provider-reported tokens and the derived cost and updates the period counter, the ledger has no update or delete path, and a forced ledger failure leaves no message row and no counter change.

- [ ] T4.5 Validate the request organisation against the caller's membership before counting usage [type=feature] [h=3] [ac=AC-4] [depends=T4.3]
  Serves: BRL-02, ENT-17
  Code: modify `backend/services/platform/system-setting/internal/handler/ai/conversation.go::ConversationHandler`
  Verify: "A non-member cannot spend an organisation's quota" via `go test -C backend/services/platform/system-setting ./internal/handler/ai/... -run TestChatOrganisationMembership`
  Done when: the chat handler takes the organisation from the request's `organizationId`, rejects a caller who is not a member of it, and leaves that organisation's ledger and counter unchanged.

- [ ] T4.6 Run the quota suite including the no-provider-call proof [type=task] [h=2] [ac=AC-4] [labels=test:regression] [depends=T4.5]
  Serves: FR-SYS-07, ENT-17
  Code: create `backend/services/platform/system-setting/internal/usecase/ai/quota_test.go::TestQuotaSuite`
  Verify: every scenario of `specs/ai-quota/spec.md` via `go test -C backend/services/platform/system-setting ./internal/usecase/ai/... -count=1`
  Done when: the suite passes with `-count=1` and asserts against the mock provider's captured-request plane that an exhausted organisation produced zero provider requests.

## PH-5 Gateway exposure, registers, documents and acceptance
> Opens the subtree at the gateway, moves every register the frozen route contract depends on, revises the documents the change makes false, and runs acceptance on the real local stack. This is the phase that makes the feature reachable from the SPA, and the one whose rollback is `AI_ENABLED=false`. Done when the golden route test, the decom and UI-contract gates and the acceptance suite all pass.

- [ ] T5.1 Route the /ai/conversation subtree to system-setting with a read timeout of at least 90 s [type=feature] [h=3] [ac=AC-1]
  Serves: FR-SYS-07, NFR-CO-01
  Code: modify `backend/services/gateway/internal/routing/default_table.go`
  Verify: "AC-1 a chat turn is answered, stored and listable" via `go test -C backend/services/gateway ./internal/routing/... -count=1`
  Done when: the `gone("UF-P1a-03", "AI chat drawer: the service behind it is retired", Subtree, "/ai/conversation")` entry at line 159 is replaced by a system-setting subtree entry, the read timeout for it is at least 90 s, and the routing table tests pass.

- [ ] T5.2 Move the five route and UI-contract registers to the new status codes [type=task] [h=4] [ac=AC-1,AC-5] [depends=T5.1]
  Serves: NFR-CO-01, NFR-CO-02
  Code: modify `backend/services/gateway/internal/router/testdata/route_decisions.approved.yaml`
  Verify: "AC-5 the flag off does not hide the routes" via `go test -C backend/services/gateway ./internal/router/... -run TestRoutesGolden`
  Done when: the `UF-P1a-03` rows for `/ai/conversation` are updated in that file and in `tools/decom/route_classes_dec.yaml`, `tools/decom/ui_route_decisions.yaml` and `tests/golden/ui-neutral/route_classes.json`, the six paths are entered as a registered exception in `backend/specs/crs/v4/UI-CONTRACT-REGISTER.md`, and `routes_golden_test.go` and the wp_f3 UI-route tests pass.

- [ ] T5.3 Record only Claude and Bedrock as deferred and remove the AI chat deferral [type=task] [h=2] [depends=T5.2]
  Serves: ADR-12, NFR-CO-02, BRL-64
  Code: modify `tools/conformance/deferred_features.yaml`
  Verify: "No stub path remains in the AI code" via `bash tools/conformance/run.sh`
  Done when: one `AI-PROVIDER-CLAUDE-BEDROCK` entry exists with its retained docs and code, the entry that deferred the AI conversation path is removed, and the conformance run including the deferred-unreachable check passes.

- [ ] T5.4 Revise the documents the change makes false, naming FR-SYS-07 and ENT-17 [type=task] [h=4] [depends=T5.3]
  Serves: FR-SYS-07, ENT-17, ADR-12
  Code: modify `docs/features/08-ai-advanced/F24_ai-integration.md`
  Verify: "The chat route is a real path" via `bash tools/conformance/run.sh`
  Done when: `docs/features/08-ai-advanced/F24_ai-integration.md`, `backend/services/platform/system-setting/specs/tdd.md`, `backend/services/platform/system-setting/docs/data-model.md` and the `FR-SYS-07` row of `docs/product/v1/PRD_TMS_v1.0.md` state the real chat path, its verification against the local-stack provider, the 503-when-off behaviour and the new tables, keep `FR-SYS-07`'s transform half deferred to CR-H02, and record that `ENT-17`'s conversation side is now implemented; the SAD is not touched.

- [ ] T5.5 Add and run the end-to-end acceptance suite on the real local stack [type=task] [h=5] [ac=AC-1,AC-3,AC-4,AC-5,AC-6,AC-7] [labels=test:regression] [depends=T5.4]
  Serves: FR-SYS-07, NFR-SE-04, NFR-SE-05
  Code: create `tests/integrated/api/wp_f3/test_ai_conversation.py::test_ai_conversation_acceptance`
  Verify: AC-1, AC-3, AC-4, AC-5, AC-6 and AC-7 via `python -m pytest tests/integrated/api/wp_f3/test_ai_conversation.py -v`
  Done when: the suite runs against the no-mock local stack with `mock-ext` as the configured provider and proves a chat turn is answered, stored and listed, the key never appears in a response or a log, an exhausted organisation gets 429 with no provider request, all six routes answer 503 with `AI_ENABLED=false`, two `Retry-After` retries then a real error, and a private provider address is blocked.

- [ ] T5.6 Verify a chat turn against an approved real provider in an egress-enabled environment [type=task] [h=3] [ac=AC-2] [labels=test:regression] [depends=T5.5]
  Serves: FR-SYS-07, NFR-SE-05
  Code: modify `tests/integrated/api/wp_f3/test_ai_conversation.py::test_ai_real_provider`
  Verify: "AC-2 an approved real LLM answers" via `python -m pytest tests/integrated/api/wp_f3/test_ai_conversation.py -k real_provider`
  Done when: a model source for the approved real provider answers a chat turn and the turn is persisted exactly as for the mock; if no approved provider or egress-enabled environment is available the task is reported blocked against open question Q8 rather than marked done.
<!-- orca:end plan -->

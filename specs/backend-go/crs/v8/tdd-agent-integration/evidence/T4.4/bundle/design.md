## Context

`FR-SYS-07` (SRS/URD, KG snapshot `snap-000001`) describes the AI feature as configuration plus an honest 501,
and `ENT-17` marks the conversation side `hội thoại (hoãn)`. The code is further along than that: model-source
CRUD with sealed keys exists in
`backend/services/platform/system-setting/internal/usecase/ai/ai.go` (`AIConfigUsecase.EditSource`,
`SourceList`, `SourceNames`) and `internal/handler/ai/ai.go` already has `AddConversation` and
`ListConversations` on the legacy `/system/ai-conversation/*` paths, but the six paths the SPA actually calls
are declared gone at `backend/services/gateway/internal/routing/default_table.go:159`
(`gone("UF-P1a-03", "AI chat drawer: the service behind it is retired", Subtree, "/ai/conversation")`).

The team approved the repo's own design for this work, SOL-P3c-E §3.2 / T-F3-18
(`backend/specs/crs/v4/solutions/SOL-P3c-E_files-ai-f18-f24.md:46-96`), with one owner (ms-system-setting),
one egress path, and — new relative to that document — a synchronous quota ledger, which AC-4 requires.

Constraints that shaped the design, all read in the KG at `snap-000001`: `NFR-SE-05` and `ADR-15` (all egress
through `shared/pkg/netguard`, profile `AdminTarget` for admin-configured endpoints), `NFR-SE-04` / `INT-14` /
`BRL-46` (secrets as `enc:v1:` AES-256-GCM, masked in every read, absent from logs, spans and metric labels),
`NFR-OB-02` (metrics catalogued, bounded cardinality), `ADR-06` with `NFR-MA-01` and `NFR-MA-02` (one shared
MySQL, exactly one owning service per table in `backend/db/ownership.yaml`, versioned `V###__*.sql` migrations,
expand before contract), `NFR-CO-01` (the SPA contract is frozen; changes only
through a registered UI exception), `NFR-CO-02` / `BRL-64` / `ADR-12` (a deferred feature must be invisible:
404 + `X-Route-Missing: 1`, never a 501 or a fake 200), and `ADR-09` (ms-integration as the single credential
holder and outbound caller).

## Goals / Non-Goals

**Goals:**
- One shared OpenAI-compatible client with a bounded, `Retry-After`-aware retry policy, netguard egress and
  masked keys (AC-6, AC-7, AC-3).
- Conversations and turns persisted, listable and owner-scoped, through the six paths the current SPA calls,
  without editing the SPA (AC-1, AC-2).
- A spend ceiling that is enforced before the provider is touched, not reconciled afterwards (AC-4).
- A flag that takes the whole feature out of service in one step (AC-5) and is also the rollback.

**Non-Goals:**
- `Claude` and `AWS Bedrock` providers; AI test-case generation and transform (CR-H02/BL-H03); hiding the
  navbar AI button (BL-J02); streaming to the browser; a self-service quota administration UI (the quota rows
  are seeded and edited through migration/ops in this change — see open question Q5).

## Decisions

### D1 — One owner: ms-system-setting holds the keys and makes the call
`ai_model_source` is already owned by ms-system-setting (`tools/conformance/table_owners.yaml`,
`ENT-17`), the sealing and masking helpers live there, and keeping the resolved key inside the owner means no
other service ever stores a provider credential. CR-H02 and BL-H03 reach it through a new
`tms/identity/v1/ai.proto AIService`, whose per-method caller list lives in
`backend/shared/proto/tms/identity/v1/acl.yaml` and is enforced by `grpcx.ParseACL`
(`backend/shared/pkg/grpcx/acl.go`); they call `AIService.Chat`, and `ResolveModel` answers with the key
masked, so the decrypted key never crosses a service boundary.

*Rejected:* a dedicated ai-gateway service — a tenth service and a second place credentials live, for one
outbound protocol. *Rejected:* moving the AI egress into ms-integration to satisfy `ADR-09` literally — it
would split ownership of `ai_model_source` from its reader and duplicate the sealing path; the reviewer settled
this and SOL-P3c-E §3.2 specifies ms-system-setting. The tension with `ADR-09` is real and is Q1 below.

### D2 — One provider implementation, not three
`ui/tms/src/enums/modelEnum.ts` can create only `Open AI`, `DeepSeek` and `ZhiPu AI`, and all three speak
OpenAI Chat Completions. One implementation with a provider label is less code and less drift than three
adapters; the small parameter differences are covered by `Provider.Params`.

*Rejected:* a provider-per-file adapter set — unjustified until a non-compatible provider is approved.

### D3 — The client is built the way the existing guarded clients are
`netguard.NewClientFrom(netguard.FromEnv(netguard.AdminTarget), base, 60s)` plus
`telemetry.WrapTransport(target, …, WithUntrustedTarget())`, copying
`backend/services/qa-core/api-test/pkg/safehttp/safehttp.go` and
`backend/services/platform/system-setting/pkg/smtp/smtp.go`. A blocked target keeps netguard's own fixed
refusal text so the error cannot be used to probe the internal network.

*Rejected:* a bespoke allow-list inside `aiclient` — exactly what `ADR-15` centralised away.

### D4 — Retry: at most two, `Retry-After` first, capped and jittered
On 429, 5xx and timeout the client retries twice. When `Retry-After` is present it is obeyed, but capped
against the remaining 60 s budget and jittered so a fleet does not retry in lockstep; without the header it
falls back to exponential backoff. Context cancellation wins immediately. AC-6 wants "at most 2 retries then
the error", so a third attempt is the last one and its error is returned verbatim (key masked).

*Rejected:* unbounded retry until the deadline — hides provider outages and burns the user's 90 s gateway
budget.

### D5 — Quota enforced synchronously in SQL, before `ResolveModel`
AC-4 requires that the provider is *not* called, which rules out any after-the-fact reconciliation. The check
reads the current period's counter in the same database as the ledger; after a successful turn the ledger
insert, the counter update and the two message rows commit in one `shared/pkg/uow` transaction. The period is
the calendar month; usage is the provider-reported token count; cost comes from a static per-provider,
per-model price table. Because the pre-call check uses an estimate and the ledger records the truth, an
organisation can overshoot its limit by at most one turn — accepted, and stated in the risks.

*Rejected:* a Redis counter — a second source of truth for money, and it can diverge from the ledger.
*Rejected:* a Kafka-based async meter — cannot satisfy "no provider call".

### D6 — Idempotency on the client-generated `conversationId`
`ui/tms/src/views/.../conversation.vue` fires `addChat(prompt, newId)` without awaiting it and then calls
`aiChat` with the same id. Both orders must converge on one conversation, so `add` creates the conversation
with the client's id when it matches `^[A-Za-z0-9_-]{8,64}$`, and `chat` upserts an unknown id. Another user's
id is 404 on all six routes — not 403, which would confirm that the id exists.

*Rejected:* server-generated ids — would break the SPA's un-awaited call order, and the SPA is frozen.

### D7 — `AI_ENABLED` as one subtree middleware returning 503
One middleware on the `/ai/conversation` group, read from `internal/config/config.go`, default off. One place
to audit, and the flag is the rollback. 503 (not 501, not 404) says "a service that is configured off", which
is the honest answer for a feature that exists but is switched off in this environment.

*Rejected:* a per-handler check — six places to forget. *Rejected:* keeping the gateway `gone(...)` entry and
not mounting the routes when off — that would make the flag change the route table, so the golden route test
would have two valid outputs.

### D8 — Additive migrations, split in two
`V036__ss_ai_conversation_v2.sql` for the conversation side and `V037__ss_ai_quota.sql` for the quota side, so
the quota phase can be reviewed and released on its own. No `DROP`, no narrowing; the legacy `content` column
of `ai_conversation` and the `/system/ai-conversation/*` routes stay.

### D9 — The frozen-UI contract is changed by registering an exception, not by editing the SPA
`NFR-CO-01` permits a contract change through a registered UI exception. The six paths move from
`404 + X-Route-Missing: 1` to real codes, so `backend/specs/crs/v4/UI-CONTRACT-REGISTER.md`,
`backend/services/gateway/internal/router/testdata/route_decisions.approved.yaml`,
`tools/decom/route_classes_dec.yaml`, `tools/decom/ui_route_decisions.yaml` and
`tests/golden/ui-neutral/route_classes.json` are updated here. The SPA is untouched, so with AI off the user
sees the real 503 — hiding the navbar button is BL-J02.

### Requirement texts this change makes false
No published spec exists under `openspec/specs/`, so there is no delta to write against a base spec. What the
change invalidates lives in the project documents and the registers, and is handled by tasks T5.3 and T5.4:

- `FR-SYS-07`'s statement "các hành động chat và chuyển văn bản chưa nối AI phải trả 501 trung thực" and its
  acceptance "[T] Chat/transform trả 501 có thông điệp" — the chat half becomes the real path. The transform
  half stays out of scope here (CR-H02), so the revised text must keep it separable.
- `ENT-17`'s attribute note "hội thoại (hoãn)".
- `ADR-12`'s consequence "Có ngoại lệ 501 đang tồn tại cho AI (OI-02)".
- The `UF-P1a-03` route decision for `/ai/conversation` in the four registers.
- `BRL-64` and `NFR-CO-02` as applied to the AI chat: they keep applying to `Claude`/`Bedrock` only.

The documents are revised, not deleted, and the new AI spec files in this change hold the replacement
scenarios; no scenario describing the 501 or the `X-Route-Missing` behaviour of `/ai/conversation/*` is left
anywhere in this change.

## Risks / Trade-offs

- **The pre-call quota check uses an estimate** → an organisation can exceed its limit by at most one turn.
  Mitigated by capping `AI_MAX_OUTPUT_TOKENS` so the worst-case overshoot is bounded and knowable, and by the
  ledger holding the true numbers.
- **`ADR-09` says only ms-integration calls out and holds credentials** → this change adds a second such
  service. Mitigated by reusing the same netguard profile and sealing helpers and by keeping the key inside
  ms-system-setting; but the decision text needs the owner's ruling (Q1).
- **Replacing `gone("UF-P1a-03", …)` changes a frozen contract** → the golden route test, the traffic replay
  and the Playwright gates can all fail. Mitigated by updating all five registers in the same phase as the
  gateway edit and by running the golden test in that phase.
- **AC-2 cannot be verified where egress is blocked** → it needs an approved real provider and an
  egress-enabled environment. The task for it is isolated so the rest of the change can ship; if the
  environment is not available the task reports blocked rather than being marked done.
- **DeepSeek and ZhiPu differ in small parameter details from OpenAI** → mitigated by driving the parameters
  from `Provider.Params` (`adv_settings`, only entries with `enable=true`) and by covering each provider label
  against the mock.
- **A 60 s provider call behind a 90 s gateway read timeout holds a connection and a DB-free goroutine** →
  mitigated by the per-call deadline and by not holding the transaction open across the provider call: the
  quota read commits, the provider is called, then the write transaction opens.
- **Private targets must be reachable in the local stack for `mock-ext`** → the allowance is confined to the
  local stack (`netguard.AllowLoopbackForTests` in Go tests, the local-stack netguard configuration for the
  integration suite) and must not be set in `deploy/production`.

## Migration Plan

1. Deploy `V036` and `V037` (additive, no `DROP`) and the grants in
   `deploy/{dev,dev-remote,production}/mysql-init/10-users.sql`; register the new tables in
   `tools/conformance/table_owners.yaml`.
2. Deploy ms-system-setting with the routes mounted but `AI_ENABLED=false`: the six routes answer 503.
3. Deploy the gateway with the subtree entry and the 90 s read timeout; the registers and the golden route
   test move in the same release.
4. Seed `ai_org_budget` for the organisations that are to use AI, then set `AI_ENABLED=true`.

**Rollback.** `AI_ENABLED=false` takes the chat path out of service immediately (503). A full revert restores
`gone("UF-P1a-03", "AI chat drawer: the service behind it is retired", Subtree, "/ai/conversation")` in
`backend/services/gateway/internal/routing/default_table.go` and the matching register rows, so the SPA sees
today's `404 + X-Route-Missing: 1` again. The migrations are additive and are not reverted; the conversation
and ledger data can stay in place.

## Open Questions

Each of these is a point the knowledge graph or the approved decision leaves open. They are listed rather than
assumed; Q1, Q2 and Q5 should be answered before the phase that depends on them (T1.1, T4.1, T5.3).

- **Q1 — `ADR-09` (read in full at `snap-000001`).** Its text is "ms-integration là service duy nhất giữ
  credential và gọi Jira, Google". Is that list ("Jira, Google") exhaustive of the egress it governs, so that
  LLM egress from ms-system-setting is simply outside it, or does the decision need an amendment naming
  ms-system-setting as a second credential-holding, outbound-calling service? The approved decision assumes
  the former; the decision's owner should confirm. `SVC-01` (ms-system-setting, read at the same snapshot)
  lists its responsibilities without any AI or LLM egress ("Đăng nhập, người dùng, tổ chức, nhóm quyền, tham
  số, LDAP, resource pool, trung tâm tác vụ, nhật ký, thông báo, hộp thư đi"), so the same ruling has to reach
  that container description. This change does not edit the SAD.
- **Q2 — `ADR-12`, `BRL-64` and `NFR-CO-02` versus the 503 of AC-5.** All three forbid a 501 or a fake 200 for
  a deferred feature and require 404 + `X-Route-Missing: 1`; none of them says what status a feature that is
  *implemented but configured off* should return. Is 503 accepted as "configured off" and therefore outside
  the deferred registry, so only `Claude`/`Bedrock` keep a `deferred_features.yaml` row? The approved decision
  assumes yes.
- **Q3 — the quota is not in the documents.** No `Requirement`, `BusinessRule`, `QualityTarget`, `Constraint`
  or `Entity` in the KG covers an AI quota, a cost ceiling or a usage ledger; a search for
  "hạn mức chi phí quota tổ chức" at `snap-000001` returns nothing on the subject, and `Q5` (gap) and `Q6`
  (drift) both return zero rows, so the KG does not flag the gap either. The unit (provider-reported tokens),
  the period (calendar month), the subject (the organisation the SPA sends as `organizationId`) and the price
  table are therefore taken from the approved decision, not from a document. They need a requirement id.
- **Q4 — the price table.** Which prices, in what currency, and who maintains them? The design assumes a
  static per-provider, per-model table checked into the repo, with the cost ceiling stored in micro-units.
- **Q5 — who sets a quota, and how?** No route is specified for creating or editing `ai_org_budget`. This change
  seeds rows by migration and ops only; an administration surface would be a separate request.
- **Q6 — the default quota.** What token limit and cost ceiling apply to an organisation with no quota row?
  A conservative non-zero default is assumed; an explicit number is needed.
- **Q7 — `AI_ENABLED` scope.** The flag is read per environment. Is a per-organisation enablement also wanted,
  or is the organisation quota the only per-organisation control?
- **Q8 — AC-2's environment.** Which real provider is approved, with which base URL and in which environment
  is its egress permitted? Until that is named, AC-2 cannot be executed.
- **Q9 — `FR-SYS-07`'s transform half.** This change revises the chat half of the requirement; the transform
  half remains a 501/deferred matter for CR-H02. Should `FR-SYS-07` be split into two requirement ids so the
  two halves can be traced and verified separately?
- **Q10 — table-ownership register drift.** `NFR-MA-01` names `backend/db/ownership.yaml` as *the* register of
  table owners, while the conformance gate `NFR-MA-04` reads `tools/conformance/table_owners.yaml`, and both
  files exist in the repository with the AI tables listed in the latter. This change registers the new tables
  in both, but which one is authoritative should be settled so the next change does not have to guess.
- **Q11 — no `tdd-agent status` reading.** The skill asks for `tdd-agent -C <root> status` to report sources
  that are not up to date. The command was not permitted to run in this session, so this plan cannot state
  whether the SRS, URD or SAD pinned sources are current; everything cited is as of `snap-000001`.
- **Q12 — `BRL-46` does not trace to `FR-SYS-07`.** Read at `snap-000001`, `BRL-46` ("Thông tin xác thực tích
  hợp … lưu mã hoá AES-256-GCM … API không bao giờ trả lại giá trị rõ") lists `FR-SYS-01`, `FR-SYS-03`,
  `FR-JIR-01`, `FR-JIR-12`, `FR-NTF-04` — not `FR-SYS-07`, even though the AI model source's `app_key`
  (`ENT-17`) is exactly such a credential, and its enforcement mechanism column names `ms-integration`. Should
  `BRL-46` gain `FR-SYS-07` and name ms-system-setting as a second enforcement site? This change implements the
  strict reading (encrypt, mask, never log) regardless, so AC-3 does not depend on the answer.
- **Q13 — no quota or flag element to serve.** Because of Q3 and Q7 the quota and `AI_ENABLED` tasks serve
  `FR-SYS-07` and `ENT-17` by extension only; there is no id in the KG that states them. If the answer to Q3 is
  a new requirement id, the `Serves:` lines of PH-3's T3.1 and all of PH-4 should be re-pointed at it.

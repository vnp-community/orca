# BE-REQ-SOL-034: Quản trị AI của `request-service`: `AIGateway`, ngân sách, `ModelRouter`, `PromptRegistry`, eval, grounding, egress

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Ngân sách và `egress` là P0 trước khi bật cờ cho tenant thật; phần còn lại P1.

**CR:** [CR-REQ-034](../../../../../../docs/crs/v6/ai-governance/CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md)
**Service:** `request-service` (domain, usecase, adapter, migration hai dialect, `internal/prompts/`, `evals/`, proto `AiBudgetAdminService`) · `notification-service` (hai subject) · `backend-go/ci`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, RLS), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (data egress, audit), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (outbox), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (metric), service: [`usage-service`](../../../../tdd/services/usage-service.md) (lý do không mở rộng)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: CR-REQ-034; `usage-service/{migrations/postgres/0001_init.up.sql, internal/domain/usage.go, internal/usecase/record_usage_session.go}` (khoá `(tenant_id, user_id, provider, day)`, `provider ∈ claude|codex|opencode`); `ai-provider-service/internal/usecase/record_token_usage.go`; `agent/src/relay/{ai-complete-handler.ts, agent-rpc-dispatch-ai.ts:141 (ai.complete), agent-rpc-dispatch-agent-exec.ts:176 (agent.execPrompt), agent-rpc-dispatch-fs.ts}`; `notification-service/internal/{domain/notification_event.go (TranslateEvent, subjectRules, recipientsOf), adapter/eventbus/consumer.go}`; spec v6 liên quan: `solution-analysis/solutions/BE-REQ-SOL-007-*.md` (`AICompleter`, `analysis_runs`), `BE-REQ-SOL-008-*.md` (`AgentPromptRunner`), `request-lifecycle/solutions/BE-REQ-SOL-005-*.md` (`AIConnectionResolver`, `RequestClassifier`), `request-quality-rollout/tasks/TASK-REQ-025-01-*.md` (`tenant_settings`), `approval/solutions/BE-REQ-SOL-009-*.md`. `request-service` chưa có trên đĩa: mọi file của nó là "(mới)".

### Correction relative to CR-REQ-034

| # | CR nói | Thực tế / hệ quả | Xử lý |
|---|--------|------------------|-------|
| C1 | `ai_budget_counters` khoá chính `(tenant_id, budget_id, window_start)` | Ngân sách `period=request` cần một bộ đếm **mỗi Request**; `window_start` không mang `request_id` | Khoá chính `(tenant_id, budget_id, window_key)`, `window_key` là `2026-10-06` (day), `2026-10` (month), `request:<id>` (request); `window_start` giữ làm cột thông tin |
| C2 | Ghi quyết định "sẽ tự duyệt" vào `ai_usage_ledger` | Ledger là sổ **mỗi lời gọi AI** (token, model); không có chỗ cho quyết định cổng | Bảng nhỏ `ai_gate_decisions` cho `HumanGatePolicy` |
| C3 | `tenant_settings` thêm `ai_egress_mode`, `ai_trace_level` | Bảng do TASK-REQ-025-01 tạo, chỉ có `request_flow_enabled`; CR 035 cũng thêm cột | Migration này `ALTER TABLE ... ADD COLUMN` sau 025-01; mặc định `external_allowed`, `digest_only`; **lỗi đọc cài đặt thì coi `disabled`** (fail closed) |
| C4 | So sánh-và-cộng với giới hạn lấy từ `ai_budgets` | Một câu `UPDATE` không join được hai bảng giống nhau ở hai dialect | Đọc ngân sách trong cùng `InTx`, truyền giới hạn làm tham số; toàn bộ Reserve là một giao dịch (một ngân sách vượt thì rollback hết, không bộ đếm nào bị cộng dở) |
| C5 | `AICompleter.Complete(ctx, connectionID, prompt) string` đủ dùng | SOL-007 task 04 định nghĩa chữ ký chỉ có `prompt` và trả chuỗi; CR-REQ-034 cần `model`, `maxTokens` và `usage` | Mở rộng bằng `CompleteWithOptions` (không đổi hàm cũ); lỗi có cấu trúc `RelayError{Retryable, Code}` |
| C6 | Thông báo ngân sách gửi cho ai | CR không nói; `notification-service` cần `user_ids` trong payload (`recipientsOf`) | Người nhận là `ai_budgets.updated_by`; payload thêm `user_ids` (không nội dung Request); danh sách admin tenant để câu hỏi mở |
| C7 | `agent.execPrompt` cho `agent_seconds` | `agent_seconds` là thời gian tường của lời gọi Relay đo ở `request-service` (không do agent trả) | Đo bằng đồng hồ phía `request-service` quanh `RelayByDevServer` |

## 2. Giải pháp

### A. Cây thư mục (`backend-go/services/request-service/`, mới)

```
internal/domain/
  ai_step.go               # type AIStep string (8 hằng), ParseAIStep, AIMode (complete|agent_readonly|agent_write)
  ai_budget.go             # AIBudget, BudgetScope, BudgetPeriod, BudgetAction, WindowKey(period, now, requestID)
  ai_usage_ledger.go       # LedgerEntry, UsageSource, EstimateTokens(chars)
  ai_step_policy.go        # StepPolicy, ChainItem, EgressClass, DefaultStepPolicies()
  human_gate_policy.go     # Decide(...) (hàm thuần), GateDecision
  grounding_report.go      # GroundingReport, Ref, Finding
  provenance.go            # Provenance (JSON khớp CR 2.3)
internal/usecase/
  ai_gateway.go            # AIGateway.Run: điều phối trình tự
  budget_guard.go          # Reserve, Settle
  model_router.go          # Next
  egress_guard.go          # Check
  prompt_registry.go       # Render, Current, Lookup
  grounding_checker.go     # Check
  human_gate_decider.go    # Decide + ghi ai_gate_decisions (chạy bóng)
  manage_ai_budgets.go     # List, Upsert, Delete, step policies, tenant AI settings
internal/prompts/<step>/v1.0.0.tmpl ; internal/prompts/registry.go
internal/adapter/grpcclient/ai_completion_relay.go (sửa, SOL-007)   # CompleteWithOptions, RelayError
internal/adapter/{postgres,mysql}/{ai_ledger,ai_budget,ai_step_policy,ai_gate_decision}.go
internal/adapter/grpc/server_ai_admin.go
migrations/{postgres,mysql}/NNNN_ai_governance.{up,down}.sql
evals/{golden,replay,results,baseline.json,deterministic_test.go,live_test.go}
backend-go/ci/check-prompt-version-bump.sh
backend-go/services/notification-service/internal/{domain/notification_event.go,adapter/eventbus/consumer.go}  (sửa)
```

`NNNN`: đọc `ls backend-go/services/request-service/migrations/{postgres,mysql}` lúc làm; phụ thuộc TASK-REQ-025-01 (bảng `tenant_settings`) và các CR 001, 002, 004, 006, 007, 009, 010, 035 cùng thêm migration. Cấm `helpers`, `utils`, `common`, `misc`; không thêm `max-lines` disable.

### B. Migration `NNNN_ai_governance` (cả hai dialect)

```sql
CREATE TABLE request.ai_usage_ledger (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL, run_id UUID,
  step TEXT NOT NULL CHECK (step IN ('classify','solution','diagnosis','findings','answer','plan','taskspec','execute')),
  request_type TEXT, size TEXT, prompt_id TEXT NOT NULL, prompt_version TEXT NOT NULL,
  prompt_digest CHAR(64) NOT NULL, input_digest CHAR(64) NOT NULL,
  model_requested TEXT NOT NULL, model_used TEXT NOT NULL DEFAULT '', provider TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL CHECK (mode IN ('complete','agent_readonly','agent_write')), attempt INT NOT NULL DEFAULT 1,
  input_tokens BIGINT NOT NULL DEFAULT 0, output_tokens BIGINT NOT NULL DEFAULT 0,
  usage_source TEXT NOT NULL CHECK (usage_source IN ('reported','estimated')),
  agent_seconds INT NOT NULL DEFAULT 0, cost_usd_est NUMERIC(12,6) NOT NULL DEFAULT 0,
  status TEXT NOT NULL CHECK (status IN ('ok','failed','blocked')), error_code TEXT NOT NULL DEFAULT '', route_reason TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ NOT NULL, finished_at TIMESTAMPTZ);
CREATE INDEX ai_usage_ledger_tenant_time ON request.ai_usage_ledger (tenant_id, started_at);
CREATE INDEX ai_usage_ledger_request ON request.ai_usage_ledger (tenant_id, request_id);
CREATE TABLE request.ai_budgets (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL,
  scope_kind TEXT NOT NULL CHECK (scope_kind IN ('tenant','request_type','step','project')), scope_value TEXT NOT NULL DEFAULT '',
  period TEXT NOT NULL CHECK (period IN ('day','month','request')),
  limit_tokens BIGINT, limit_calls INT, limit_agent_seconds INT, limit_cost_usd_est NUMERIC(12,6),
  warn_ratio NUMERIC(3,2) NOT NULL DEFAULT 0.80 CHECK (warn_ratio > 0 AND warn_ratio <= 1),
  action TEXT NOT NULL DEFAULT 'block' CHECK (action IN ('block','warn_only')), enabled BOOLEAN NOT NULL DEFAULT TRUE,
  version BIGINT NOT NULL DEFAULT 1, updated_by UUID NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, scope_kind, scope_value, period));
CREATE TABLE request.ai_budget_counters (
  tenant_id UUID NOT NULL, budget_id UUID NOT NULL REFERENCES request.ai_budgets(id) ON DELETE CASCADE,
  window_key TEXT NOT NULL, window_start TIMESTAMPTZ NOT NULL,
  used_tokens BIGINT NOT NULL DEFAULT 0, used_calls INT NOT NULL DEFAULT 0, used_agent_seconds INT NOT NULL DEFAULT 0,
  used_cost_usd_est NUMERIC(12,6) NOT NULL DEFAULT 0, warned_at TIMESTAMPTZ,
  PRIMARY KEY (tenant_id, budget_id, window_key));
CREATE TABLE request.ai_step_policies (id UUID PRIMARY KEY, tenant_id UUID NOT NULL, step TEXT NOT NULL, request_type TEXT, size TEXT,
  chain JSONB NOT NULL, escalate_on_schema_failure BOOLEAN NOT NULL DEFAULT FALSE, version BIGINT NOT NULL DEFAULT 1,
  updated_by UUID NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE UNIQUE INDEX ai_step_policies_scope ON request.ai_step_policies (tenant_id, step, COALESCE(request_type,''), COALESCE(size,''));
CREATE TABLE request.ai_trace_blobs (ledger_id UUID PRIMARY KEY REFERENCES request.ai_usage_ledger(id) ON DELETE CASCADE, tenant_id UUID NOT NULL,
  prompt_text TEXT NOT NULL CHECK (octet_length(prompt_text) <= 262144), response_text TEXT NOT NULL CHECK (octet_length(response_text) <= 262144),
  expires_at TIMESTAMPTZ NOT NULL);
CREATE TABLE request.ai_gate_decisions (id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL, subject_type TEXT NOT NULL,
  request_type TEXT NOT NULL, size TEXT, risk TEXT NOT NULL, confidence NUMERIC(4,3), decision TEXT NOT NULL CHECK (decision IN ('required','auto_allowed')),
  mode TEXT NOT NULL CHECK (mode IN ('shadow','enforce')), policy_version TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
ALTER TABLE request.tenant_settings
  ADD COLUMN ai_egress_mode TEXT NOT NULL DEFAULT 'external_allowed' CHECK (ai_egress_mode IN ('external_allowed','internal_only','disabled')),
  ADD COLUMN ai_trace_level TEXT NOT NULL DEFAULT 'digest_only' CHECK (ai_trace_level IN ('digest_only','full')),
  ADD COLUMN ai_gate_mode TEXT NOT NULL DEFAULT 'off' CHECK (ai_gate_mode IN ('off','shadow','enforce'));
```

Mọi bảng: `ENABLE` và `FORCE ROW LEVEL SECURITY`, policy `tenant_isolation` (mẫu SOL-001 mục 2.D). MySQL: `CHAR(36)`, `JSON`, `DECIMAL(12,6)`, `TIMESTAMP(6)`; `UNIQUE` trên `COALESCE(...)` không có ở MySQL: dùng cột sinh `request_type_key VARCHAR(40) GENERATED ALWAYS AS (IFNULL(request_type,'')) STORED` (hoặc lưu chuỗi rỗng thay NULL ở ứng dụng và `UNIQUE` thẳng); chọn **lưu chuỗi rỗng thay NULL ở cả hai dialect** để một SQL. `ALTER TABLE ... ADD COLUMN` MySQL không có `IF NOT EXISTS`; migration chạy một lần.

### C. `AIGateway` và trình tự

```go
type AICall struct {
    Step domain.AIStep; Mode domain.AIMode; RequestID, RunID, ProjectID string
    RequestType, Size string
    PromptVars map[string]any        // đưa vào PromptRegistry.Render
    MaxTokens  int
    ContextPack *domain.ContextPack  // BE-REQ-SOL-031, tuỳ chọn
    Grounding   GroundingSpec        // paths/commands cần kiểm sau khi parse đầu ra (do bước gọi cung cấp qua hàm Extract)
}
type AIResult struct{ Text string; Provenance domain.Provenance; Usage domain.Usage; LedgerIDs []string; Grounding *domain.GroundingReport }
func (g *AIGateway) Run(ctx context.Context, c AICall, extract GroundingExtractor) (AIResult, error)
```

Trình tự `Run`: (1) `EgressGuard.Check` → `REQUEST_AI_EGRESS_BLOCKED` trước khi dựng prompt (không dòng ledger "ok", ghi một dòng `blocked`); (2) `PromptRegistry.Render(step, vars)` → `{id, version, text, digest}`; (3) `BudgetGuard.Reserve(est)` → `REQUEST_AI_BUDGET_EXCEEDED` (Request giữ trạng thái, không tạo run); (4) vòng `ModelRouter.Next` tối đa 3 phần tử: gọi `AIRelay`, mỗi lần một dòng ledger (`attempt`, `route_reason`); lỗi `retryable` thì chuyển phần tử kế, lỗi nội dung thì dừng; (5) `Settle` (trả phần dư hoặc cộng thiếu theo `usage` thật); (6) tuỳ chọn `GroundingChecker`; (7) trả `Provenance` cho bước gọi lưu vào sản phẩm. Gọi `BudgetGuard.Settle` trong `defer` để không rò hạn mức khi panic.

Ước lượng token: `ceil(utf8.RuneCountInString(prompt)/4) + maxTokens` (đề xuất ký tự/4, chưa hiệu chỉnh); tiếng Việt có dấu và mã khác nhau về byte/ký tự: dùng **số rune**, không `len(s)`. Giá `cost_usd_est` từ `REQUEST_AI_PRICE_TABLE` (JSON `{"<model>":{"in_per_mtok":x,"out_per_mtok":y}}`), thiếu model thì 0 và `usage_source` vẫn ghi.

### D. `BudgetGuard`

`Reserve(ctx, c AICall, est Estimate) (Reservation, error)` trong một `InTx`: (a) đọc mọi `ai_budgets` `enabled` khớp `(tenant, project, request_type, step)` (khớp theo `scope_kind`); (b) với mỗi cái: `INSERT ... ON CONFLICT (tenant_id, budget_id, window_key) DO NOTHING` (MySQL `INSERT IGNORE`) rồi

```sql
UPDATE request.ai_budget_counters SET used_tokens = used_tokens + $1, used_calls = used_calls + 1,
  used_agent_seconds = used_agent_seconds + $2, used_cost_usd_est = used_cost_usd_est + $3
WHERE tenant_id=$4 AND budget_id=$5 AND window_key=$6
  AND ($7::bigint IS NULL OR used_tokens + $1 <= $7) AND ($8::int IS NULL OR used_calls + 1 <= $8)
  AND ($9::int IS NULL OR used_agent_seconds + $2 <= $9) AND ($10::numeric IS NULL OR used_cost_usd_est + $3 <= $10)
```

(MySQL dùng `(? IS NULL OR ...)` với tham số lặp lại.) 0 dòng bị ảnh hưởng: nếu `action=block` thì rollback toàn bộ giao dịch và trả `REQUEST_AI_BUDGET_EXCEEDED`; `warn_only` thì không rollback, phát `orca.request.ai_budget.exceeded`. (c) `ratio = used/limit` lớn nhất ≥ `warn_ratio` và `UPDATE ... SET warned_at=now() WHERE ... AND warned_at IS NULL` ảnh hưởng 1 dòng thì phát `orca.request.ai_budget.warning` (đúng một lần mỗi cửa sổ). Không có ngân sách nào: không giới hạn, chỉ ghi ledger. `Settle(res, actual)` chạy `UPDATE ... SET used_tokens = used_tokens + (actual - est)` (có thể âm), không kiểm hạn mức (đã tiêu).

### E. `ModelRouter`, `EgressGuard`, `PromptRegistry`, `HumanGatePolicy`, `GroundingChecker`

- **ModelRouter.Next(step, type, size, attempt, failed)**: chọn `StepPolicy` đặc thù nhất `(step, type, size) > (step, type) > (step)` rồi mặc định trong mã; bỏ phần tử mà provider không có khoá (`DevServerProfile.EnvPresent` từ BE-REQ-SOL-031 task 04; không đọc được hồ sơ thì không bỏ, ghi cảnh báo); bỏ phần tử vi phạm egress; chuyển phần tử kế khi `RelayError.Retryable` (HTTP 429, 5xx, timeout, kết nối) hoặc thông điệp `No API key found`, tối đa 2 lần chuyển mỗi lần gọi; lỗi nội dung không chuyển trừ khi `escalate_on_schema_failure`. Agent cũ không có `error.data`: suy từ thông điệp (`Anthropic API error 429`), ghi `route_reason=parsed_message`. `agent_readonly`, `agent_write` chỉ họ `claude`.
- **EgressGuard**: `external_allowed` đi tiếp; `internal_only` chỉ phần tử `internal` (hiện chưa có đường nào ⇒ như `disabled`, ghi `REQUEST_AI_NO_ELIGIBLE_MODEL` hoặc `REQUEST_AI_EGRESS_BLOCKED` tuỳ chain); `disabled` chặn mọi bước. Với `ContextPack` (SOL-031 Q8): `internal_only` loại mảnh `trust=low` và nguồn `mcp` (cổng `EgressFilter`). Tenant `internal_only` mà hồ sơ dev server có khoá nhà cung cấp ngoài: cảnh báo, không chặn.
- **PromptRegistry**: `//go:embed` mỗi `v*.tmpl`; `registry.go` khai `{id, version, requiredVars, outputSchemaID}`; `Render` dùng `text/template` với `Option("missingkey=error")`; thiếu biến bắt buộc là lỗi lập trình, không gửi prompt. File đã phát hành bất biến (kiểm bằng `ci/check-prompt-version-bump.sh`). `Provenance` JSON đúng CR 2.3.
- **HumanGatePolicy**: hàm thuần `Decide(in GateInput) GateDecision` (bảng tổ hợp ở CR 2.4); `risk` chưa có CR chấm điểm ⇒ `unknown` ⇒ `required`; `mode=shadow` chỉ ghi `ai_gate_decisions`.
- **GroundingChecker**: sau khi qua schema, trước khi lưu; đường dẫn qua `RepoReader` (BE-REQ-SOL-031 task 03: `Glob`, `ReadFile`) bằng `SafeRelPath`; `new:true` bỏ qua; lệnh đối chiếu `DevServerProfile.Tools`, `package.json`, `Makefile`; `ratio < 0.8` thì thử lại đúng một lần với danh sách thiếu; vẫn thấp thì `needs_review=true`, không xoá phần sai; tối đa 100 kiểm tra; không dev server thì `status="skipped"`.

### F. Phân quyền, lỗi, sự kiện, quan sát

`AiBudgetAdminService` (proto mới, `orca.request.v1`): `ListAiBudgets`, `UpsertAiBudget`, `DeleteAiBudget`, `ListAiStepPolicies`, `UpsertAiStepPolicy`, `GetAiTenantSettings`, `SetAiTenantSettings`; chỉ `role=admin` (nhóm `admin` của BE-REQ-SOL-035; trong lúc chờ kiểm `tenant.Role`). Lỗi: `REQUEST_AI_BUDGET_EXCEEDED` (ResourceExhausted), `REQUEST_AI_EGRESS_BLOCKED` (FailedPrecondition), `REQUEST_AI_NO_ELIGIBLE_MODEL` (FailedPrecondition), `REQUEST_AI_GROUNDING_FAILED` (chỉ khi `grounding_action=fail`). Sự kiện: `orca.request.ai_budget.warning|exceeded` `{tenant_id, budget_id, scope, ratio, user_ids}`. Metric (không dùng `tenant_id` làm nhãn): `request_ai_calls_total{step,model,status}`, `request_ai_tokens_total{step,kind}`, `request_ai_budget_blocked_total{step}`, `request_ai_fallback_total{step}`, `request_ai_grounding_ratio`, `request_ai_schema_failure_total{step}`. Audit (qua `AuditRecorder` của TASK-REQ-024-02): `ai.budget.set`, `ai.egress.set`, `ai.trace.set`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Ledger, ngân sách ở `request-service` | `usage-service` không có chiều Request, bước |
| 2 | Reserve là một giao dịch, rollback hết khi một ngân sách chặn | Không rò hạn mức |
| 3 | `window_key` thay `window_start` trong khoá | Hỗ trợ `period=request` (C1) |
| 4 | Mặc định không giới hạn nhưng luôn ghi sổ | Có số liệu trước khi đặt hạn mức |
| 5 | Lỗi đọc cài đặt egress thì `disabled` | Không lộ dữ liệu khi chưa chắc |
| 6 | Chỉ chuyển model khi lỗi `retryable` | Lỗi nội dung không sửa được bằng đổi model |
| 7 | Prompt phát hành bất biến, kiểm bằng CI | Tái lập |
| 8 | Eval xác định chạy mọi PR; eval LLM thật chạy riêng | PR không phụ thuộc mạng và khoá |

## 4. Phụ thuộc và thứ tự

```
SOL-001, 002, TASK-REQ-025-01 (tenant_settings) ─▶ 034-01 ─▶ 034-02 ─▶ 034-04 ─▶ CR 005, 007, 008, 012 gọi qua AIGateway
034-03 (PromptRegistry) ─────────────────────────────────────▲
034-01 ─▶ 034-05 (admin RPC, thông báo)       034-06 (grounding, sau BE-REQ-SOL-031 task 03)
034-04 ─▶ 034-07 (gate chạy bóng)             034-03, 034-06 ─▶ 034-08 (eval)
CR-REQ-033 (usage, maxTokens, error.data) là điều kiện của 034-04 (agent)
```

## 5. Kiểm thử

- **Unit:** `BudgetGuard` bảng cửa sổ UTC (ranh giới 23:59:59 và 00:00:00), `WindowKey`; `ModelRouter.Next` (loại theo khoá, egress, lỗi retryable, giới hạn 2 lần chuyển); `HumanGatePolicy.Decide` đủ tổ hợp; `GroundingChecker` với `RepoReader` giả; ước lượng token tiếng Việt có dấu (rune khác byte); `PromptRegistry` thiếu biến.
- **Integration hai DB:** so sánh-và-cộng dưới đua (hai goroutine, vượt hạn: đúng một qua); `ON CONFLICT` và `INSERT IGNORE`; JSON tiếng Việt trong `chain`; cách ly tenant; `warned_at` chỉ một lần.
- **Hợp đồng:** golden `provenance`; `buf breaking` cho `AiBudgetAdminService`; hai subject mới qua `TranslateEvent`.
- **Eval:** tầng xác định trong CI; `eval_live` thủ công (chưa chạy).
- Lệnh: `cd backend-go && go test ./services/request-service/... && go test -tags=integration ./services/request-service/internal/adapter/...`. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Ước lượng ký tự/4 sai lệch; `execPrompt` chưa có số thật nên `agent_seconds` mới đáng tin hơn.
- `cost_usd_est` từ bảng giá tự nhập, dễ lỗi thời.
- Chuỗi dự phòng cần nhiều khoá nhà cung cấp trên dev server; thực tế có thể chỉ một.
- `GroundingChecker` tăng tải lên dev server dùng chung; chi phí Relay chưa đo.
- Mẫu vàng từ Request thật chứa dữ liệu khách: phải ẩn danh (BE-REQ-SOL-035 `secretscan`).
- Chưa dịch vụ nào gọi `RecordTokenUsage` của `ai-provider-service`: không dựa vào quota của nó.

## 7. Câu hỏi mở

1. Người nhận cảnh báo ngân sách: `updated_by` (v1) hay mọi admin tenant (cần RPC liệt kê admin của `tenant-service`, chưa kiểm chứng).
2. Đẩy tổng hợp sang `usage-service` (mở rộng enum `provider`): giữ riêng ở v1.
3. Hạn mặc định theo Request (1 triệu token) và ngưỡng grounding 0,8 cần chủ sản phẩm xác nhận sau khi có dữ liệu ledger.
4. Model nội bộ cho `ai.complete`/`execPrompt`: CR agent nào sở hữu.
5. `auto_allowed` có tồn tại ở v1 không (mặc định `ai_gate_mode=off`).

## 8. Tham chiếu

- `backend-go/services/usage-service/migrations/postgres/0001_init.up.sql`; `backend-go/services/ai-provider-service/internal/usecase/record_token_usage.go`
- `agent/src/relay/ai-complete-handler.ts`, `agent-rpc-dispatch-ai.ts`, `agent-rpc-dispatch-agent-exec.ts`
- `backend-go/services/notification-service/internal/domain/notification_event.go`, `internal/adapter/eventbus/consumer.go`
- `specs/backend-go/crs/v6/{solution-analysis,request-lifecycle,approval,request-quality-rollout,context-sources}/...`
- `docs/research/receive-request/enterprise-readiness-checklist.md` (mục 4, 5, 7), `ai-steps-and-dev-server-connection-flows.md`

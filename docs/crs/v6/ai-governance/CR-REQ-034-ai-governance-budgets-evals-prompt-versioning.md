# CR-REQ-034 — Quản trị AI: ngân sách, chọn model và dự phòng, phiên bản prompt, bộ đánh giá, người trong vòng lặp, chống ảo giác, lưu vết, không gửi dữ liệu ra ngoài

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-034 |
| **Tên** | Lớp quản trị cho mọi lời gọi AI của `request-service`: ngân sách theo tenant/loại Request/bước, `ModelRouter` có dự phòng, `PromptRegistry` có phiên bản và provenance, eval harness trong CI, chính sách người trong vòng lặp, kiểm chứng đường dẫn và lệnh, sổ cái tái lập, chế độ không gửi dữ liệu ra ngoài |
| **Loại** | Feature (nền tảng phi chức năng) |
| **Priority** | 🟠 P1 (ngân sách và `egress` là P0 trước khi bật cờ cho tenant thật) |
| **Effort** | Large (10 đến 14 ngày: ledger và ngân sách 4, router 2, registry và provenance 2, eval 3, grounding 3) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-001, 002 (bảng, migration), CR-REQ-005, 007, 008, 012 (các bước AI), CR-REQ-033 (`usage`, `maxTokens`, lỗi có cấu trúc của `ai.complete`; hồ sơ năng lực dev server) |
| **Mở khoá** | CR-REQ-025 (rollout có số liệu), cờ bật cho tenant thật; CR-REQ-029 (đề xuất, chưa có file) dùng `GroundingChecker` |
| **Tác động** | `request-service` (domain, usecase, adapter, migration hai dialect, `evals/`, `internal/prompts/`), `notification-service` (hai subject), `ci/`; không đổi `usage-service`, không đổi `ai-provider-service` |

## 1. Bối cảnh và vấn đề (đã đọc code ngày 2026-10-06)

1. **Chi phí AI chưa có ai đo.** `usage-service` (`migrations/postgres/0001_init.up.sql`) lưu `usage.sessions` với `provider` chỉ nhận `claude|codex|opencode` (CLI), khoá `(tenant_id, user_id, provider, day)`, không có chiều Request hay bước, không có ngân sách. Chỉ route `POST /v1/usage/sessions` của gateway ghi vào đó. `ai-provider-service` có `RecordTokenUsage` và `QuotaLimitDay` theo **account** (cảnh báo 80%, vượt thì chuyển trạng thái lỗi và phát `ai_provider.usage.quota_exceeded`), nhưng grep toàn `backend-go/services` không thấy dịch vụ nào gọi `RecordTokenUsage`, và đường `ai.complete` không mang `accountId`.
2. **Không biết đã tốn bao nhiêu.** `ai.complete` (`agent/src/relay/ai-complete-handler.ts`) chỉ trả `{content, model}`; phản hồi nhà cung cấp có `usage` nhưng bị bỏ. `agent.execPrompt` trả `stdout` văn bản. CR-REQ-033 mục 2.8 thêm `usage` cho `ai.complete`; với `execPrompt` chưa có số (chưa kiểm chứng `--output-format json`).
3. **Model do agent quyết định.** Client Go chỉ gửi `prompt`; agent chọn theo `params.model`, rồi `config.defaultModel`, rồi `ORCA_AI_MODEL_ID`, rồi mặc định cứng `claude-opus-4-5`. Khoá lấy từ biến môi trường của tiến trình agent. Không có chọn theo bước, không có dự phòng khi lỗi 429/5xx. `ai.complete` chỉ hỗ trợ tiền tố `claude`, `gpt`, `o1`, `o3`, `o4`, `gemini`; không có model nội bộ (`ollama` chỉ có ở `agent.spawn`).
4. **Prompt là chuỗi dựng trong mã** (`buildDecomposePrompt` ở task-service, `buildSolutionPrompt` ở CR-REQ-007). Không có phiên bản, không ghi vào kết quả. Không thể trả lời "Solution này sinh bằng prompt nào".
5. **Không có đo chất lượng.** CR-REQ-007 mục 5 ghi "chưa kiểm chứng khả năng tuân thủ schema của model"; không có bộ mẫu vàng cho phân loại, Solution, Plan, TaskSpec.
6. **Cổng người đã có nhưng cứng.** README v6 3.4 đặt cổng theo loại; CR-REQ-010 đặt chính sách người duyệt. Chưa có chính sách nói khi nào được tự động duyệt.
7. **Ảo giác chưa có lưới đỡ.** Solution/Plan nhắc đường dẫn, lệnh kiểm; CR-REQ-007 chỉ kiểm schema, CR-REQ-008 che bí mật. Không ai kiểm đường dẫn có thật, lệnh có thật.
8. **Khách hàng cấm dữ liệu ra ngoài** (`enterprise-readiness-checklist.md` mục 7) chưa có công tắc nào.

## 2. Giải pháp đề xuất

Một lớp mỏng `AIGateway` trong `request-service` bọc cổng gọi hiện có (adapter `ai_completion_relay.go` của CR-REQ-007 và `agent_prompt_relay.go` của CR-REQ-008). Mọi bước AI đi qua nó theo một trình tự cố định:

```
bước AI ─▶ EgressGuard ─▶ BudgetGuard.Reserve ─▶ PromptRegistry.Render ─▶ ModelRouter.Next
        ─▶ Relay (ai.complete | agent.execPrompt) ─▶ [lỗi có thể thử lại: model kế tiếp]
        ─▶ GroundingChecker ─▶ Ledger.Settle + Provenance ─▶ kết quả
```

Bước (`step`) là enum cố định: `classify`, `solution`, `diagnosis`, `findings`, `answer`, `plan`, `taskspec` (dành cho CR-REQ-029), `execute` (chỉ để đếm, thực thi do task-service).

### 2.1 Sổ cái AI và ngân sách

**Vị trí:** ở `request-service`, không ở `usage-service`. Lý do: `usage-service` khoá theo user/provider CLI và không có `request_id` làm chiều; mở rộng nó buộc đổi một service ngoài series. Có thể đẩy tổng hợp sang `usage-service` về sau (không thuộc CR này).

Bảng `ai_usage_ledger` (mới; mọi bảng có `tenant_id`, RLS `tenant_isolation` ở Postgres):

| Cột | Postgres | MySQL | Ghi chú |
|---|---|---|---|
| `id`, `tenant_id`, `request_id` | `UUID` | `CHAR(36)` | không FK chéo service |
| `run_id` | `UUID NULL` | `CHAR(36) NULL` | `analysis_runs.id` hoặc id lần chạy của CR-REQ-012/013 |
| `step` | `TEXT CHECK (...)` | `VARCHAR(16)` | enum ở trên |
| `request_type`, `size` | `TEXT NULL` | `VARCHAR NULL` | chụp lúc gọi, để lọc ngân sách |
| `prompt_id`, `prompt_version` | `TEXT` | `VARCHAR(64)` | `classify`, `1.2.0` |
| `prompt_digest`, `input_digest` | `CHAR(64)` | `CHAR(64)` | sha256 prompt đã render; sha256 đầu vào chuẩn hoá |
| `model_requested`, `model_used`, `provider` | `TEXT` | `VARCHAR(64)` | |
| `mode` | `TEXT` | `VARCHAR(16)` | `complete`, `agent_readonly`, `agent_write` |
| `attempt` | `INT` | `INT` | số lần thử trong chuỗi dự phòng |
| `input_tokens`, `output_tokens` | `BIGINT` | `BIGINT` | |
| `usage_source` | `TEXT CHECK IN ('reported','estimated')` | | `estimated` = `ceil(ký tự / 4)`, đề xuất, chưa hiệu chỉnh |
| `agent_seconds` | `INT` | `INT` | thời gian chạy `execPrompt` |
| `cost_usd_est` | `NUMERIC(12,6)` | `DECIMAL(12,6)` | từ bảng giá cấu hình, không phải hoá đơn |
| `status`, `error_code`, `route_reason` | `TEXT` | | `ok|failed|blocked`; lý do chuyển model |
| `started_at`, `finished_at` | `TIMESTAMPTZ` | `TIMESTAMP(6)` | |

Chỉ mục `(tenant_id, started_at)`, `(tenant_id, request_id)`. Giữ lâu theo chính sách của CR-REQ-035 (mặc định 400 ngày, đề xuất).

Bảng `ai_budgets` (mới): `id`, `tenant_id`, `scope_kind` (`tenant|request_type|step|project`), `scope_value` (rỗng với `tenant`), `period` (`day|month|request`), `limit_tokens`, `limit_calls`, `limit_agent_seconds`, `limit_cost_usd_est` (mỗi giới hạn `NULL` = không giới hạn), `warn_ratio` (mặc định 0.8), `action` (`block|warn_only`), `enabled`, `version`, `updated_by`, `updated_at`. Bảng `ai_budget_counters` (mới): `(tenant_id, budget_id, window_start)` khoá chính, `used_tokens`, `used_calls`, `used_agent_seconds`, `used_cost_usd_est`, `warned_at`.

**Thi hành (`BudgetGuard`)**: trước lời gọi, tìm mọi ngân sách `enabled` khớp `(tenant, project, request_type, step)`; với mỗi cái chạy một câu so sánh-và-cộng, cùng cú pháp cho hai DB:
`UPDATE ai_budget_counters SET used_tokens = used_tokens + :est WHERE tenant_id=:t AND budget_id=:b AND window_start=:w AND (:limit IS NULL OR used_tokens + :est <= :limit)`; 0 dòng bị ảnh hưởng nghĩa là vượt. Ước lượng đặt trước (`Reserve`) = ký tự prompt / 4 + `maxTokens`; sau lời gọi `Settle` trả phần dư hoặc cộng phần thiếu theo `usage` thật. `warn_ratio` vượt lần đầu trong cửa sổ thì ghi `warned_at` (một lần, so sánh-và-ghi) và phát outbox `orca.request.ai_budget.warning`. Vượt hạn: `action=block` trả `REQUEST_AI_BUDGET_EXCEEDED` (ResourceExhausted), Request giữ nguyên trạng thái, run không tạo; `warn_only` chỉ phát `orca.request.ai_budget.exceeded`. Hạn mức theo `request` (mặc định đề xuất 1 000 000 token ước lượng, chưa đo) chặn vòng sinh lại vô hạn của CR-REQ-007 (`analysis_revision`). Cửa sổ `day|month` theo UTC.

CRUD ngân sách: RPC quản trị `AiBudgetAdminService` (`List`, `Upsert`, `Delete`; chỉ `role=admin`; cùng cách xử lý "RPC chưa có trong README 3.6" như CR-REQ-010, xem mục 7). Không có ngân sách nào thì mặc định dựng sẵn: không giới hạn (hành vi hiện nay) nhưng vẫn ghi ledger, để có số liệu trước khi đặt hạn mức.

### 2.2 `ModelRouter`: chọn model theo bước và dự phòng

Cấu hình `ai_step_policies` (mới, theo tenant; mặc định dựng sẵn trong mã): `(tenant_id, step, request_type NULL, size NULL)` → `chain` JSON mảng `{model, egress_class, max_tokens, timeout_seconds}`. Ví dụ mặc định đề xuất: `classify` chuỗi `[haiku-class, sonnet-class]`; `solution` và `plan` `[opus-class, sonnet-class]`. Tên model cụ thể là cấu hình, không cứng trong mã (id model đổi nhanh; chưa kiểm chứng id hiện hành).

Thuật toán `Next(step, attempt)`: bỏ phần tử mà nhà cung cấp tương ứng không có khoá trên dev server (đọc `env.present` của hồ sơ năng lực, CR-REQ-033: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`); bỏ phần tử vi phạm `egress` (2.7); phần tử đầu còn lại được gửi bằng tham số `model` của `ai.complete` (agent đã hỗ trợ, phía Go chưa gửi). Chuyển phần tử kế khi lỗi `retryable` (`error.data.retryable` của CR-REQ-033 mục 2.8: HTTP 429, 5xx, timeout, kết nối) hoặc `No API key found`; tối đa 2 lần chuyển mỗi lần gọi; không chuyển khi lỗi nội dung (JSON sai schema thì tính vào lần thử lại của CR-REQ-007, không đổi model trừ khi chuỗi ghi `escalate_on_schema_failure=true`). Mỗi lần chuyển ghi một dòng ledger với `route_reason`. Agent cũ không có `error.data`: router suy từ chuỗi thông điệp (`Anthropic API error 429`), coi là mong manh, ghi `route_reason=parsed_message`.

`agent.execPrompt` chỉ chạy `claude` (CR-REQ-033, `agent-print-mode-exec.ts`), nên chuỗi cho `diagnosis|findings|answer|execute` chỉ có model họ `claude`; dự phòng sang họ khác chưa khả thi (đã nêu, không bịa).

### 2.3 `PromptRegistry` và provenance

- Prompt là file mẫu trong repo: `internal/prompts/<step>/v<MAJOR.MINOR.PATCH>.tmpl` (mới) cộng `internal/prompts/registry.go` (đăng ký `id@version`, tham số bắt buộc, schema đầu ra tương ứng). Mỗi bước có một `current`. File đã phát hành **bất biến**: sửa nội dung bắt buộc thêm file phiên bản mới (kiểm bằng CI, 2.5).
- Provenance ghi vào mọi sản phẩm AI: khối `provenance` trong JSON của `solutions.options` (CR-REQ-007/008), của Plan (`plan_task` metadata do CR-REQ-012 quyết) và của kết quả phân loại (`requests.classification_reason` giữ nguyên, thêm vào `request_type_history.reason` JSON hoặc cột; xem mục 8). Dạng: `{"tool":"request-service","step":"solution","prompt_id":"solution","prompt_version":"1.2.0","model":"...","run_id":"...","input_digest":"...","generated_at":"..."}`. Khớp "Nguồn gốc" ở `artifact-formats-ontology-and-execution-readiness.md` mục 5.
- Prompt cũ vẫn dịch ngược được: ledger lưu `prompt_digest`; thay đổi tham số thì digest đổi.

### 2.4 Chính sách người trong vòng lặp (`HumanGatePolicy`)

Hàm thuần `Decide(subject_type, request_type, size, risk) → required | auto_allowed`. **Mặc định mọi cổng của README 3.4 là `required`** (không đổi hành vi). Cho phép `auto_allowed` chỉ khi tenant bật và đồng thời: `subject_type ∈ {request_type, task_list}`, `request_type ∈ {task, docs}`, `size=S`, không có nhãn rủi ro cao; không bao giờ cho `hotfix`, `security`, `ops_request`, `pre_deploy`, `size=L`, hoặc `confidence` < ngưỡng (mặc định 0.9, chưa hiệu chỉnh). Điểm rủi ro `risk` đến từ mô-đun chấm điểm bằng quy tắc (đề xuất `impact-assessment-and-risk-scoring.md`, chưa có CR); khi chưa có, `risk=unknown` và kết quả luôn `required`.

Có **chế độ chạy bóng** (`mode=shadow`, mặc định): chỉ ghi "sẽ tự duyệt" vào ledger và metric, không tự duyệt thật; dùng để hiệu chỉnh trước khi bật (`enterprise-readiness-checklist.md` mục 5). Tự duyệt thật tạo Approval do hệ thống quyết định với `decided_by='system:policy'`, `comment` ghi `policy_version`; cần CR-REQ-009 hỗ trợ `decided_by_kind` (mục 8).

### 2.5 Eval harness trong CI

Thư mục `backend-go/services/request-service/evals/` (mới):
- `golden/<step>/*.json`: mỗi mẫu `{id, input, expect}`. `classify`: `expect.type` (cho phép `accept[]`), `size`; `solution`: schema hợp lệ, số phương án trong khoảng, `affected_areas` phải chứa mọi mục `must_mention[]`; `plan`: phủ mọi `AC-n`, có nhãn bắt buộc theo loại (CR-REQ-012/014), không có vòng phụ thuộc; `taskspec`: đủ trường bắt buộc, `check` có lệnh. Khởi đầu đề xuất ít nhất 30 mẫu mỗi bước, trộn mẫu tổng hợp và Request thật đã ẩn danh (chưa có dữ liệu thật; đây là việc phải làm sớm).
- Hai tầng: **(a) xác định, chạy mọi PR** (`go test ./services/request-service/evals/... -run Deterministic`): kiểm bộ kiểm schema, trình trích JSON, `GroundingChecker`, registry bằng đầu ra đã ghi sẵn (`replay/*.json`), không gọi LLM, không cần khoá. **(b) mô hình thật, chạy thủ công hoặc hằng đêm** (build tag `eval_live`): gọi dev server thật, có trần chi phí (`EVAL_MAX_COST_USD_EST`), ghi `evals/results/<ngày>.json`.
- Chỉ số: tỉ lệ qua schema, độ chính xác phân loại và macro-F1, tỉ lệ phủ `AC`, `grounding_ratio`, số lần thử trung bình, token trung bình. So với `evals/baseline.json`; CI đỏ khi một chỉ số tụt quá ngưỡng (đề xuất 3 điểm phần trăm, chưa hiệu chỉnh).
- Kiểm tra "đổi prompt phải bump" (`ci/check-prompt-version-bump.sh`, mới): file dưới `internal/prompts/` đổi nhưng không có file phiên bản mới, hoặc phiên bản đổi mà `baseline.json` không được cập nhật, thì thất bại.
- Chưa dùng LLM làm giám khảo ở v1 (khó tái lập). Promptfoo là lựa chọn thay thế đã nêu ở `openspec-and-ai-tooling-integration.md` mục 3.2; chọn bộ Go để khỏi thêm bộ công cụ Node vào CI backend.

### 2.6 Chống ảo giác (`GroundingChecker`)

Chạy sau khi đầu ra qua kiểm schema, trước khi lưu. Trích tham chiếu có cấu trúc: đường dẫn (`affected_areas[].name` kind `module|api|schema`, `scope.include`, bằng chứng `file:line` của `diagnosis`, file trong task) và lệnh (`checks[].command`, lệnh test trong Plan).

- **Đường dẫn:** qua Relay tới `fs.stat` và `fs.glob` của agent (đã có ở `agent-rpc-dispatch-fs.ts`); đường dẫn mà tài liệu khai là "sẽ tạo" (`new:true`) được bỏ qua. `file:line` kiểm file tồn tại và `line` nhỏ hơn số dòng (cách đọc số dòng bằng `fs.readFile` có tham số khoảng hay không: chưa kiểm chứng; phương án thay: `fs.grep`).
- **Lệnh:** tên lệnh đầu có trong `tools` của hồ sơ năng lực (CR-REQ-033); script `pnpm run <x>`/`make <x>`/`go test` được đối chiếu bằng đọc `package.json` và `Makefile` qua `fs.readFile`. Lệnh không xác định được là `unverified`, không phải `missing`.
- **Kết quả:** `grounding_report {checked, missing[], unverified[], ratio}` lưu cùng sản phẩm. `ratio` dưới ngưỡng (mặc định 0.8) thì thử lại đúng một lần với danh sách đường dẫn không tồn tại làm phản hồi; vẫn dưới ngưỡng thì lưu với cờ `needs_review=true` hiển thị cho người duyệt, **không tự bỏ phần sai**. Tối đa 100 kiểm tra mỗi sản phẩm, gộp theo `fs.glob` để giảm số lời gọi Relay (chi phí và độ trễ chưa đo). Cần dev server kết nối; không có thì `grounding_report.status="skipped"`, không chặn.

### 2.7 Chế độ không gửi dữ liệu ra ngoài (`egress`)

Cài đặt theo tenant trong `tenant_settings` (bảng đã có ở CR-REQ-025; thêm cột `ai_egress_mode`): `external_allowed` (mặc định) | `internal_only` | `disabled`. Mỗi phần tử `chain` có `egress_class` (`external` | `internal`).

- `disabled`: mọi bước AI trả `REQUEST_AI_EGRESS_BLOCKED` (FailedPrecondition) trước khi dựng prompt; luồng Request vẫn chạy thủ công (người xác nhận loại ở CR-REQ-005, người viết Solution/Plan, nhập tay). Giao diện thủ công thuộc CR-REQ-019/020/021, chưa có "nhập tay Solution": ghi ở mục 7.
- `internal_only`: chỉ dùng phần tử `internal`. Hiện **chưa có đường nào**: `ai.complete` không hỗ trợ model nội bộ (tiền tố chỉ `claude|gpt|o1|o3|o4|gemini`), `execPrompt` chỉ `claude`. Dùng được khi cấu hình `ANTHROPIC_BASE_URL` trỏ cổng nội bộ trên dev server (chưa kiểm chứng) hoặc khi có CR agent hỗ trợ `ollama`/OpenAI-compatible. Vì vậy `internal_only` không có phần tử `internal` nào thì kết quả như `disabled`.
- Phát hiện lệch: `EgressGuard` đọc hồ sơ năng lực để cảnh báo dev server có khoá nhà cung cấp ngoài khi tenant là `internal_only` (chỉ cảnh báo).
- Che bí mật khỏi prompt, và các biện pháp bảo mật của Request, ở CR-REQ-035.

### 2.8 Lưu vết tái lập

`ai_trace_level` theo tenant: `digest_only` (mặc định: ledger có `prompt_digest`, `input_digest`, không lưu nội dung) | `full` (thêm bảng `ai_trace_blobs`: `ledger_id`, `tenant_id`, `prompt_text`, `response_text`, `expires_at`, tối đa 256 KB mỗi cột, đã qua che bí mật, hết hạn 30 ngày đề xuất). Lý do mặc định `digest_only`: prompt chứa mã nguồn và nội dung nghiệp vụ của khách (cùng nguyên tắc `ai-complete-handler.ts` chỉ trace độ dài). Tái lập = dựng lại prompt từ `prompt_id@version` + `input_digest` khi còn dữ liệu đầu vào, hoặc đọc blob khi `full`. Bản ghi `raw_output` của `analysis_runs` (CR-REQ-007, cắt 256 KB) giữ như hiện nay nhưng chịu cùng hạn lưu giữ.

### 2.9 Mã lỗi, sự kiện, quan sát

Lỗi: `REQUEST_AI_BUDGET_EXCEEDED` (ResourceExhausted), `REQUEST_AI_EGRESS_BLOCKED` (FailedPrecondition), `REQUEST_AI_NO_ELIGIBLE_MODEL` (FailedPrecondition, mọi phần tử bị loại), `REQUEST_AI_GROUNDING_FAILED` (chỉ ghi ở run khi cấu hình `grounding_action=fail`). Sự kiện outbox: `orca.request.ai_budget.warning`, `orca.request.ai_budget.exceeded` (payload `tenant_id`, `budget_id`, `scope`, `ratio`; không có nội dung Request); `notification-service` cần hai dòng ở `consumer.go` và `notification_event.go` như CR-REQ-010 mục 2.6. Metric (cùng quy ước CR-REQ-024, không đặt `tenant_id` làm nhãn): `request_ai_calls_total{step,model,status}`, `request_ai_tokens_total{step,kind}`, `request_ai_budget_blocked_total{step}`, `request_ai_fallback_total{step}`, `request_ai_grounding_ratio` (histogram), `request_ai_schema_failure_total{step}`.

### 2.10 Tệp sẽ tạo trong `backend-go/services/request-service/` (mới)

`internal/domain/ai_budget.go`, `ai_usage_ledger.go`, `ai_step_policy.go`, `human_gate_policy.go`, `grounding_report.go`; `internal/usecase/ai_gateway.go`, `budget_guard.go`, `model_router.go`, `prompt_registry.go`, `grounding_checker.go`, `egress_guard.go`, `manage_ai_budgets.go`; `internal/prompts/<step>/v1.0.0.tmpl`; `internal/adapter/{postgres,mysql}/ai_ledger_repository.go`, `ai_budget_repository.go`; `evals/`; migration `NNNN_ai_governance.{up,down}.sql` cho cả hai dialect; `ci/check-prompt-version-bump.sh` (backend-go/ci/, mới).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Ledger và ngân sách ở `request-service` | `usage-service` không có chiều Request/bước và provider là CLI; không đổi service ngoài series |
| 2 | Ước lượng token khi chưa có số thật, đánh dấu `usage_source` | Có số liệu ngay, hiệu chỉnh sau khi có `usage` thật |
| 3 | Mặc định không giới hạn nhưng luôn ghi sổ | Tránh chặn nhầm trước khi biết mức dùng thật |
| 4 | Dự phòng chỉ cho lỗi `retryable` | Lỗi nội dung đổi model không sửa được và tốn tiền |
| 5 | Prompt bất biến theo phiên bản | Tái lập và so sánh được trước/sau |
| 6 | Tự duyệt tắt, có chế độ chạy bóng | Điểm rủi ro chưa hiệu chỉnh |
| 7 | Eval xác định chạy mọi PR, eval LLM thật chạy riêng có trần chi phí | PR không được phụ thuộc mạng và khoá |
| 8 | Grounding không tự xoá phần sai, chỉ cờ và thử lại một lần | Xoá âm thầm làm hỏng ý nghĩa tài liệu |
| 9 | `digest_only` là mặc định lưu vết | Mã nguồn khách không nên nằm trong DB theo mặc định |
| 10 | `internal_only` không có phần tử nội bộ thì như `disabled` | Không giả vờ hỗ trợ khi agent chưa có |

## 4. Tiêu chí chấp nhận

- [ ] Migration chạy lên và xuống trên Postgres 14+ và MySQL 8.0.1+; câu so sánh-và-cộng từ chối bản ghi vượt hạn ở cả hai.
- [ ] Hai lời gọi đồng thời cùng làm ngân sách vượt: đúng một cái qua, một cái `REQUEST_AI_BUDGET_EXCEEDED`.
- [ ] Vượt `warn_ratio` phát đúng một sự kiện cảnh báo mỗi cửa sổ.
- [ ] Mỗi lời gọi AI (kể cả lần thử và lần chuyển model) có đúng một dòng ledger với `prompt_id`, `prompt_version`, `prompt_digest`.
- [ ] Lỗi 429 ở model đầu: chuyển model thứ hai, ledger có hai dòng, `route_reason` đúng; lỗi JSON sai schema không chuyển model.
- [ ] Mọi `solutions.options`, kết quả Plan và phân loại do AI sinh có khối `provenance` hợp lệ.
- [ ] Sửa file prompt đã phát hành mà không thêm phiên bản làm `ci/check-prompt-version-bump.sh` thất bại.
- [ ] Eval xác định chạy trong PR dưới 2 phút và thất bại khi bộ kiểm schema bị làm hỏng cố ý.
- [ ] `GroundingChecker`: đường dẫn không tồn tại vào `missing`; đường dẫn `new:true` không; ratio thấp thì đúng một lần thử lại rồi `needs_review=true`.
- [ ] Tenant `disabled`: không có lời gọi Relay nào, lỗi `REQUEST_AI_EGRESS_BLOCKED`; tenant `internal_only` không có phần tử nội bộ cũng vậy.
- [ ] `HumanGatePolicy` ở chế độ chạy bóng không tạo Approval tự động; ghi đúng ledger "sẽ tự duyệt"; `hotfix` và `size=L` luôn `required` (test bảng).
- [ ] `ai_trace_level=digest_only` không lưu nội dung prompt ở bất kỳ bảng nào.

## 5. Kiểm thử

- **Unit (domain):** `BudgetGuard` bảng cửa sổ và giới hạn, `ModelRouter.Next` (loại theo khoá, egress, lỗi retryable), `HumanGatePolicy.Decide` bảng đủ tổ hợp, `GroundingChecker` với `fs` giả, ước lượng token tiếng Việt có dấu (độ dài byte khác ký tự).
- **Integration hai DB:** so sánh-và-cộng dưới đua, `UPSERT` bộ đếm cửa sổ (Postgres `ON CONFLICT`, MySQL `ON DUPLICATE KEY`), JSON tiếng Việt, chỉ mục ledger.
- **Hợp đồng:** golden JSON `provenance`; `buf breaking` cho `AiBudgetAdminService`; hai subject mới được `TranslateEvent` của `notification-service` chấp nhận.
- **Eval:** tầng xác định trong CI; tầng `eval_live` chạy thủ công (chưa chạy, cần dev server và khoá).
- **Chưa kiểm chứng:** độ chính xác của ước lượng ký tự/4 so với số thật; hành vi 429 thật của nhà cung cấp qua `ai.complete`; chi phí Relay của `GroundingChecker` trên repo lớn.

## 6. Rủi ro và điểm chưa kiểm chứng

- Ước lượng token sai lệch có thể chặn sớm hoặc muộn; với `execPrompt` chưa có số thật nên ngân sách `agent_seconds` mới là tín hiệu đáng tin hơn.
- Giá `cost_usd_est` đến từ cấu hình tự nhập, không phải hoá đơn; dễ lỗi thời.
- Chuỗi dự phòng chỉ dùng được khi dev server có nhiều khoá nhà cung cấp; thực tế có thể chỉ có một.
- Eval với 30 mẫu có sai số lớn; ngưỡng 3 điểm chỉ là điểm xuất phát.
- `GroundingChecker` thêm độ trễ và tải lên dev server (một dev server chung nhiều Request).
- Mẫu vàng từ Request thật có thể chứa dữ liệu khách; phải ẩn danh trước khi vào repo (CR-REQ-035).
- Chưa có dịch vụ nào đang gọi `RecordTokenUsage`; không dựa vào quota của `ai-provider-service` cho Request.

## 7. Câu hỏi mở

1. Có chấp nhận thêm `AiBudgetAdminService` và bảng cấu hình mà README 3.6 không liệt kê, hay v1 chỉ seed qua migration/biến môi trường?
2. Nên đẩy tổng hợp xuống `usage-service` (mở rộng enum `provider` thêm `request_flow`) hay giữ riêng? CR này chọn giữ riêng.
3. "Nhập tay Solution/Plan" cho tenant `disabled`: cần CR giao diện riêng; hiện CR-REQ-020/021 chỉ có duyệt.
4. Mức hạn mặc định theo Request (1 triệu token) và ngưỡng grounding 0.8 cần chủ sản phẩm xác nhận sau khi có dữ liệu ledger.
5. Hỗ trợ model nội bộ cho `ai.complete`/`execPrompt` thuộc một CR agent khác; ai sở hữu?
6. Tự duyệt (`auto_allowed`) có nên tồn tại ở v1 hay bỏ hẳn cho đến khi chấm điểm rủi ro hiệu chỉnh?

## 8. Tác động tới CR hiện có (không sửa trong lần này)

| CR | Cần sửa gì |
|---|---|
| CR-REQ-005 | Gọi `ai.complete` qua `AIGateway` (bước `classify`, có `model` do router chọn); ghi `provenance` vào `request_type_history.reason` (hoặc thêm cột `provenance JSON`); `EgressGuard` khiến tenant `disabled` chỉ phân loại tay |
| CR-REQ-007 | Mục 2.4 prompt qua `PromptRegistry` (`solution@x.y.z`); `ai_completion_relay.go` nhận `model`, `maxTokens`; thêm `provenance` vào schema `options` 2.3; mục 6 rủi ro "chưa có hạn mức" trỏ CR này; `BudgetGuard` đặt trước bước 3 của 2.5; thêm lỗi `REQUEST_AI_*` ở 2.8 |
| CR-REQ-008 | `agent_readonly` đếm `agent_seconds` và `GroundingChecker` cho `file:line`; chuỗi model chỉ họ `claude` |
| CR-REQ-012 | Plan qua `PromptRegistry`, `maxTokens` lớn hơn, `GroundingChecker` cho đường dẫn và lệnh `checks`; `provenance` trong Plan |
| CR-REQ-009 | `Approval` thêm `decided_by_kind` (`user|system_policy`) hoặc ghi chú `system:policy`; test không cho tự duyệt `hotfix`, `security`, `pre_deploy` |
| CR-REQ-010 | Bảng `approval_policies` thêm cột `auto_approve_allowed BOOLEAN NOT NULL DEFAULT FALSE`; mặc định 2.5 giữ nguyên |
| CR-REQ-002 | Ghi các bảng mới (`ai_usage_ledger`, `ai_budgets`, `ai_budget_counters`, `ai_step_policies`, `ai_trace_blobs`) vào danh sách bảng bổ sung của README v6 mục 8 dòng 3 |
| CR-REQ-024 | Thêm metric `request_ai_*` (2.9) và hành động audit `ai.budget.set`, `ai.egress.set` |
| CR-REQ-025 | `tenant_settings` thêm `ai_egress_mode`, `ai_trace_level`; e2e cho ngân sách và `disabled`; chạy eval xác định trong CI; cửa phát hành GA yêu cầu eval `eval_live` đạt baseline |
| CR-REQ-013 | Task thực thi (`execute`) ghi ledger `agent_seconds`; cân nhắc ngân sách `request` cho cả lúc thực thi |
| CR-REQ-033 | Mục 2.8 là điều kiện của CR này (đã bao gồm) |

## 9. Tham chiếu

- `/opt/repos/orca/backend-go/services/usage-service/migrations/postgres/0001_init.up.sql`, `internal/domain/usage.go`, `internal/usecase/record_usage_session.go`; `backend-go/services/api-gateway/internal/adapter/httpgateway/usage_routes.go`
- `/opt/repos/orca/backend-go/services/ai-provider-service/internal/usecase/record_token_usage.go`, `model_provider_map.go`; `migrations/postgres/0005_health_and_usage_writes.up.sql`
- `/opt/repos/orca/agent/src/relay/ai-complete-handler.ts`, `agent-rpc-dispatch-ai.ts`, `agent-print-mode-exec.ts`, `agent-rpc-dispatch-fs.ts`
- `/opt/repos/orca/docs/crs/v6/solution-analysis/CR-REQ-007-*.md`, `CR-REQ-008-*.md`; `approval/CR-REQ-010-*.md`; `request-quality-rollout/CR-REQ-024-*.md`, `CR-REQ-025-*.md`
- `/opt/repos/orca/docs/research/receive-request/enterprise-readiness-checklist.md` (mục 4, 5, 7), `ai-steps-and-dev-server-connection-flows.md` (mục 4, 5, 8), `artifact-formats-ontology-and-execution-readiness.md` (mục 5, 10, 11), `openspec-and-ai-tooling-integration.md` (mục 3.2)
- `/opt/repos/orca/docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md`

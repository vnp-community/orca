# TASK-REQ-034-04: `EgressGuard`, `ModelRouter` và `AIGateway` (trình tự, ledger, relay mở rộng)

**From Solution:** BE-REQ-SOL-034 (mục C, E)
**Priority:** P0 (`EgressGuard`, ngân sách), P1 (`ModelRouter`)
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/{ai_gateway.go,egress_guard.go,model_router.go,ai_relay_errors.go}` (mới), `.../internal/adapter/grpcclient/ai_completion_relay.go` (sửa, do SOL-007 task 04 tạo), `.../internal/adapter/grpcclient/agent_prompt_relay.go` (sửa, SOL-008 task 02), `.../internal/domain/errors.go` (sửa), `.../internal/usecase/ports.go` (sửa), và `_test.go` tương ứng
**Depends on:** TASK-REQ-034-01, 034-02, 034-03; BE-REQ-SOL-005 (`AIConnectionResolver`), BE-REQ-SOL-007 task 04 (`AICompleter`), BE-REQ-SOL-008 task 02 (`AgentPromptRunner`); CR-REQ-033 cho `usage` và `error.data` (không chặn: có đường dự phòng)
**Status:** [ ] TODO

---

## Context

- Agent hiện nay (đã đọc `agent-rpc-dispatch-ai.ts:141`): `ai.complete` nhận `{prompt, format, taskId?, model?, accountId?, resolvedApiKey?}`, trả `{content, model}`; lỗi trả `ai.complete failed: <thông điệp>` mã `ServerError`, **không** có `error.data.retryable` (CR-REQ-033 mục 2.8 thêm sau). Router phải chạy được với agent cũ bằng cách phân tích thông điệp (`Anthropic API error 429`, `No API key found`), đánh `route_reason=parsed_message`.
- `agent.execPrompt` chỉ chạy `claude` (CR-REQ-033); chuỗi model của `agent_readonly|agent_write` chỉ họ `claude`.
- `AIConnectionResolver` (SOL-005 mục C1): `connectionID = projectID` có thể trượt (BUG-025), rơi về `RelayByDevServer`. `AIGateway` dùng cổng này, không tự dựng lại.
- Hồ sơ năng lực (`EnvPresent`: tên biến khoá có mặt) lấy từ `DevServerProfileReader` của BE-REQ-SOL-031 task 04; chưa có RPC của CR-REQ-033 thì `ErrNotConnected` ⇒ router không loại phần tử nào (không đoán), ghi log.
- `ai_egress_mode` đọc từ `tenant_settings` (task 01); **lỗi đọc ⇒ coi `disabled`**. Không cache dài: cache 10 giây chỉ cho giá trị `external_allowed`; mọi giá trị khác không cache.
- Không bao giờ gửi `resolvedApiKey` (BE-REQ-SOL-035).

## Việc cần làm

1. `ports.go`: 

```go
type AIRelay interface {
    Complete(ctx context.Context, conn domain.AIConnection, req CompleteRequest) (CompleteResponse, error)
    ExecPrompt(ctx context.Context, conn domain.AIConnection, req ExecPromptRequest) (ExecPromptResponse, error)
}
type CompleteRequest struct{ Prompt, Format, Model string; MaxTokens int; AccountID string }
type CompleteResponse struct{ Content, Model string; Usage *domain.Usage } // Usage nil nếu agent cũ
type EgressSettings interface{ Mode(ctx context.Context) (domain.EgressMode, error) }
```

2. `ai_completion_relay.go`: thêm `CompleteWithOptions(ctx, conn, CompleteRequest)` bọc cùng `Relay`; hàm cũ `Complete(ctx, connectionID, prompt)` gọi lại nó với `Model=""` (không đổi hành vi cho SOL-007 đang chạy). `params_json` gồm `{"prompt":..., "format":"json"|"text", "model":..., "maxTokens":...}` (chỉ thêm khoá khi có giá trị; `maxTokens` chỉ gửi sau khi CR-REQ-033 xác nhận agent hiểu; cờ cấu hình `REQUEST_AI_SEND_MAX_TOKENS=false` mặc định). Đọc `result.usage{input_tokens, output_tokens}` nếu có.
3. `ai_relay_errors.go`: `type RelayError struct{ Retryable bool; Code, Message, Source string }` (`Source` = `structured|parsed_message`); `ClassifyRelayError(err error) RelayError`: đọc `error.data.retryable` nếu có; nếu không, khớp thông điệp (không phân biệt hoa thường): `429`, `rate limit`, `overloaded`, `5\d\d`, `timeout`, `ECONNRESET`, `No API key found` ⇒ `Retryable=true` (khoá thiếu coi là "thử model khác"); mọi thứ khác `false`. Bảng test dương/âm.
4. `egress_guard.go`: `Check(ctx, step AIStep, chain []ChainItem) (allowed []ChainItem, err error)`: `external_allowed` → trả nguyên; `internal_only` → chỉ `EgressClass=internal`, rỗng thì `ErrAINoEligibleModel` (hiện chưa có đường nội bộ); `disabled` → `ErrAIEgressBlocked`; lỗi đọc cài đặt → `ErrAIEgressBlocked`. Hook `EgressFilter` cho Context Pack: `FilterPack(mode, pack) pack` loại mảnh `trust=low` và nguồn `mcp` khi `internal_only` (BE-REQ-SOL-031 Q8). Cảnh báo (không chặn) khi `internal_only` mà hồ sơ có khoá nhà cung cấp ngoài.
5. `model_router.go`: `Next(ctx, in RouteInput) (ChainItem, RouteDecision, error)` với `RouteInput{Step; RequestType, Size string; Attempt int; Failed []string; Mode AIMode}`: (a) `StepPolicy` đặc thù nhất `(step,type,size) > (step,type) > (step) > mặc định` từ `StepPolicyRepository` + `DefaultStepPolicies`; (b) lọc theo `Mode` (agent chỉ họ `claude`); (c) lọc khoá nhà cung cấp theo tiền tố model: `claude*`→`ANTHROPIC_API_KEY`, `gpt|o1|o3|o4*`→`OPENAI_API_KEY`, `gemini*`→`GOOGLE_API_KEY` (không đọc được hồ sơ thì không lọc); (d) bỏ `Failed`; (e) rỗng ⇒ `ErrAINoEligibleModel`. `RouteDecision{ChainIndex int; Reason string}`.
6. `ai_gateway.go`: `Run(ctx, c AICall, extract GroundingExtractor) (AIResult, error)` theo solution mục C:
   1. `egress.Check` (chặn thì ghi **một** dòng ledger `status=blocked`, `error_code`, không `Insert` prompt);
   2. `PromptRenderer.Render` → `Rendered`;
   3. `BudgetGuard.Reserve`;
   4. vòng tối đa 3 phần tử: `router.Next` → `AIRelay.Complete|ExecPrompt` với `context.WithTimeout(item.TimeoutSeconds)`; **mỗi lần thử** một `ai_usage_ledger.Insert` (`attempt`, `route_reason`), kết thúc `Finish` (`ok|failed`, token `reported` hoặc `estimated`, `agent_seconds` đo đồng hồ phía service); `RelayError.Retryable` ⇒ thêm vào `Failed`, đi tiếp (tối đa 2 lần chuyển); lỗi nội dung ⇒ dừng; `EscalateOnSchemaFailure` do bước gọi báo qua `ErrSchemaFailure`;
   5. `defer BudgetGuard.Settle` kể cả lỗi (trả phần dư);
   6. nếu `extract != nil` thì `GroundingChecker.Check` (task 06; cổng rỗng khi chưa có);
   7. `ai_trace_level=full` ⇒ `InsertBlob` với `prompt_text`, `response_text` đã qua `secretscan.Redact` và cắt 256 KB, `expires_at = now + ai_trace_retention_days` (mặc định 30); `digest_only` ⇒ **không** ghi blob (test).
   8. trả `AIResult{Text, Provenance, Usage, LedgerIDs, Grounding}`.
7. Lỗi miền: `ErrAIEgressBlocked` (`REQUEST_AI_EGRESS_BLOCKED`), `ErrAINoEligibleModel` (`REQUEST_AI_NO_ELIGIBLE_MODEL`), `ErrAIBudgetExceeded` (đã có ở task 02). Ánh xạ gRPC ở nơi use case gọi `AIGateway` (CR 005/007/008/012), không ở đây.
8. Metric: bộ đếm `request_ai_calls_total{step,model,status}`, `request_ai_tokens_total{step,kind}`, `request_ai_fallback_total{step}`, `request_ai_budget_blocked_total{step}` qua cổng `AIMetrics` (bản cài Prometheus ở adapter; cùng quy ước nhãn của CR-REQ-024: không `tenant_id`).
9. Cập nhật CR gọi (ghi vào PR, không sửa trong task này): CR 005/007/008/012 thay lời gọi `AICompleter`/`AgentPromptRunner` trực tiếp bằng `AIGateway.Run`.

## Kiểm thử

- `ai_relay_errors_test.go`: bảng thông điệp (`Anthropic API error 429` ⇒ retryable; `ai.complete failed: invalid JSON in prompt` ⇒ không); cấu trúc `error.data.retryable=true` thắng thông điệp.
- `egress_guard_test.go`: `disabled` ⇒ `ErrAIEgressBlocked` và `FakeAIRelay` **không bị gọi**; `internal_only` không có phần tử nội bộ ⇒ như `disabled`; lỗi đọc cài đặt ⇒ chặn; `FilterPack` loại mảnh `low` và `mcp`.
- `model_router_test.go`: loại phần tử thiếu khoá; chuyển phần tử kế sau 429; tối đa 2 lần chuyển; lỗi JSON sai schema không chuyển trừ `escalate`; `agent_readonly` bỏ phần tử `gpt-*`; không hồ sơ ⇒ không lọc.
- `ai_gateway_test.go` (fake `AIRelay`, `FakeClock`): (a) 429 ở model đầu, model hai thành công ⇒ **hai** dòng ledger, `route_reason` đúng; (b) mỗi lời gọi có `prompt_id`, `prompt_version`, `prompt_digest`; (c) `digest_only` không có blob, `full` có blob đã che bí mật (fixture `ghp_...`); (d) ngân sách chặn ⇒ không `Relay`; (e) panic trong relay vẫn `Settle`; (f) `usage` nil ⇒ `usage_source=estimated`; (g) **không** có `resolvedApiKey` trong `params_json` của bất kỳ lời gọi (kiểm bằng fake ghi lại).
- Integration (hai dialect): `TestGateway_LedgerRowsPersisted`, `TestGateway_BlockedWritesBlockedRow`.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpcclient/...`; `go test -tags=integration ./services/request-service/internal/usecase/...`.

## Tiêu chí hoàn thành

- [ ] Tenant `disabled`: không lời gọi Relay nào, lỗi `REQUEST_AI_EGRESS_BLOCKED`; `internal_only` không có phần tử nội bộ cũng vậy.
- [ ] Mỗi lời gọi AI (kể cả lần thử, lần chuyển model) có đúng một dòng ledger.
- [ ] 429 ở model đầu chuyển model hai; JSON sai schema không chuyển.
- [ ] `digest_only` không lưu nội dung prompt ở bảng nào.
- [ ] `resolvedApiKey` không bao giờ xuất hiện trong tham số Relay.

## Rủi ro và lưu ý

- Khoá lọc theo tiền tố model dựa vào quy ước CR 2.2; id model mới có tiền tố lạ sẽ không được lọc (không chặn, chỉ không loại).
- Phân tích thông điệp lỗi mong manh: chỉ là cầu nối tới khi agent có `error.data`.
- `AIGateway.Run` dài: tách `ai_gateway_attempts.go` khi gần 300 dòng (không thêm `max-lines` disable).
- Gọi tuần tự trong chuỗi dự phòng nhân độ trễ (tối đa 3 lần timeout); `TimeoutSeconds` phải cộng dồn dưới timeout của run (`REQUEST_AI_COMPLETE_TIMEOUT` 120s ở SOL-007).

# backend-go Solutions: AI Governance (v6)

**CRs:** [docs/crs/v6/ai-governance](../../../../../../docs/crs/v6/ai-governance/README.md)
**Hợp đồng chung:** [docs/crs/v6/README.md](../../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`usage-service`](../../../../tdd/services/usage-service.md)

> 📋 Proposed. Chưa triển khai, chưa chạy test nào. `request-service` chưa có trên đĩa; mọi file của nó là "(mới)".

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-REQ-034](../../../../../../docs/crs/v6/ai-governance/CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md) | [BE-REQ-SOL-034](./BE-REQ-SOL-034-ai-governance-budgets-evals-prompt-versioning.md) | `request-service`, `notification-service`, `proto`, `backend-go/ci` | Large | `TASK-REQ-034-01` đến `-08` |

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| `usage-service` khoá theo user/provider CLI, không có chiều Request | Đúng (`provider ∈ claude|codex|opencode`) | Không |
| `ai.complete` chỉ trả `{content, model}`, lỗi không có `error.data` | Đúng (`agent-rpc-dispatch-ai.ts:141`); lỗi là `ai.complete failed: <msg>` mã `ServerError` | Không; router phân tích thông điệp tới khi có CR-REQ-033 |
| Client Go chỉ gửi `prompt` | Agent đã nhận `model`, `format`, `accountId`, **`resolvedApiKey`** | Bổ sung: không bao giờ gửi `resolvedApiKey` |
| Bộ đếm khoá `(tenant, budget, window_start)` | Không biểu diễn được `period=request` | **Lệch** ⇒ `window_key` (C1) |
| Quyết định cổng ghi vào ledger | Ledger là sổ mỗi lời gọi AI | **Lệch** ⇒ bảng `ai_gate_decisions` (C2) |
| `tenant_settings` thêm `ai_egress_mode`, `ai_trace_level` | Bảng do TASK-REQ-025-01 tạo, CR 035 cũng thêm cột | Phụ thuộc thứ tự migration (C3) |
| Thông báo ngân sách | `TranslateEvent` cần `user_ids` trong payload | Người nhận là `updated_by` (C6) |
| `agent_seconds` do agent trả | Agent không trả | Đo ở `request-service` (C7) |

## Thứ tự thực thi và phụ thuộc

```
SOL-001, 002, TASK-REQ-025-01 ─▶ 034-01 ─▶ 034-02 ─┬─▶ 034-04 (AIGateway) ─▶ 034-07, CR 005/007/008/012 đổi sang AIGateway
                                  034-03 (prompt) ─┘          ▲
                                  034-01 ─▶ 034-05 (admin RPC, thông báo)
   BE-REQ-SOL-031 task 03, 04 ─▶ 034-06 (grounding) ─▶ 034-04 (bước 6) ─▶ 034-08 (eval)
```

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| H1 | Một cổng `AIGateway` cho mọi bước AI | Một chỗ ngân sách, định tuyến, ghi sổ, provenance |
| H2 | Sổ cái và ngân sách ở `request-service` | `usage-service` không có chiều Request, bước |
| H3 | `Reserve` là một giao dịch, rollback hết khi chặn | Không rò hạn mức |
| H4 | Lỗi đọc cài đặt egress ⇒ `disabled` | Fail closed |
| H5 | Chỉ chuyển model khi `retryable`; không bao giờ gửi `resolvedApiKey` | Chi phí; khoá ở dev server |
| H6 | Prompt bất biến theo phiên bản, CI chặn sửa | Tái lập |
| H7 | `HumanGatePolicy` chỉ chạy bóng ở v1 | Chưa có điểm rủi ro hiệu chỉnh |

## Điều còn mở

- `AiBudgetAdminService` và `ai_step_policies` chưa có trong README v6 mục 3.6 (người điều phối).
- Người nhận cảnh báo ngân sách (Q1), ngưỡng 0,8 và hạn 1 triệu token (Q3), `auto_allowed` (Q5).
- Chưa có dịch vụ nào gọi `RecordTokenUsage` của `ai-provider-service`; không dựa vào nó.

# ai-governance: tasks (backend-go, v6)

> 📋 Proposed. Mọi task `Status: [ ] TODO`; chưa task nào được làm hay chạy. Phần `request-service` dựa trên SOL-001, 002 (service chưa có); phần `notification-service`, agent, `usage-service` đã đối chiếu với file thật.

## Solution → Task

| Solution | Task | Tên | Priority | Phụ thuộc |
|---|---|---|---|---|
| [BE-REQ-SOL-034](../solutions/BE-REQ-SOL-034-ai-governance-budgets-evals-prompt-versioning.md) | [TASK-REQ-034-01](./TASK-REQ-034-01-ai-governance-migration-and-repositories.md) | Migration `ai_governance` và repository | P0 | SOL-001, 002, TASK-REQ-025-01 |
| | [TASK-REQ-034-02](./TASK-REQ-034-02-ai-domain-and-budget-guard.md) | Domain AI và `BudgetGuard` | P0 | 034-01 |
| | [TASK-REQ-034-03](./TASK-REQ-034-03-prompt-registry-and-provenance.md) | `PromptRegistry`, `provenance`, script bump phiên bản | P1 | 034-02 |
| | [TASK-REQ-034-04](./TASK-REQ-034-04-egress-model-router-and-ai-gateway.md) | `EgressGuard`, `ModelRouter`, `AIGateway` | P0 | 034-01..03, SOL-005/007/008 |
| | [TASK-REQ-034-05](./TASK-REQ-034-05-ai-budget-admin-service-and-notifications.md) | `AiBudgetAdminService`, thông báo | P0 | 034-01, 034-02, TASK-REQ-024-02 |
| | [TASK-REQ-034-06](./TASK-REQ-034-06-grounding-checker.md) | `GroundingChecker` | P1 | 034-04, BE-REQ-SOL-031 task 03, 04 |
| | [TASK-REQ-034-07](./TASK-REQ-034-07-human-gate-policy-shadow-mode.md) | `HumanGatePolicy` chạy bóng | P2 | 034-01, BE-REQ-SOL-009 |
| | [TASK-REQ-034-08](./TASK-REQ-034-08-eval-harness-and-ci.md) | Eval harness và cổng CI | P1 | 034-03, 034-06 |

## Thứ tự phụ thuộc

```
034-01 ─▶ 034-02 ─┬─▶ 034-04 ─▶ 034-06 ─▶ 034-08
                  │      ▲          ▲
034-03 ───────────┴──────┘          │ BE-REQ-SOL-031 task 03, 04
034-01 ─▶ 034-05  (song song 034-04)
034-01 ─▶ 034-07  (độc lập, P2)
```

## Ghi chú

- Làm sớm (P0 trước khi bật cờ cho tenant thật): 034-01, 034-02, 034-04 (phần `EgressGuard`, `BudgetGuard`), 034-05.
- Số migration `NNNN` ở 034-01: đọc thư mục migrations lúc làm; hai dialect cùng số; chờ TASK-REQ-025-01 (bảng `tenant_settings`).
- Trước khi sửa `ai_completion_relay.go` (SOL-007 task 04), chạy `gitnexus_impact` trên `AICompleter` (quy ước repo).
- CR 005, 007, 008, 012 cần đổi sang `AIGateway.Run` và `PromptRegistry` (xem mục 8 của CR); các task đó không nằm trong feature này.
- Lệnh chung: `cd backend-go && go test ./services/request-service/... && go test -tags=integration ./services/request-service/internal/adapter/...`; `cd services/notification-service && go test ./internal/...`; `buf lint && buf breaking` trong `backend-go/proto`. Chưa chạy.
- Số liệu (ngưỡng 3 điểm, 0,8, 0,9, 1 triệu token, 30 mẫu, giá) là đề xuất chưa đo.

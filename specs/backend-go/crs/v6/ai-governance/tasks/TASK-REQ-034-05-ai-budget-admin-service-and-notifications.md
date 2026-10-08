# TASK-REQ-034-05: `AiBudgetAdminService`, cài đặt AI theo tenant và thông báo ngân sách

**From Solution:** BE-REQ-SOL-034 (mục F)
**Priority:** P0 (admin ngân sách, egress) / P1 (thông báo)
**Service:** `request-service`, `notification-service`, `proto`
**File:** `backend-go/proto/orca/request/v1/ai_admin.proto` (mới), `backend-go/services/request-service/internal/usecase/manage_ai_budgets.go` (mới), `.../internal/adapter/grpc/server_ai_admin.go` (mới), `.../cmd/server/main.go` (sửa: đăng ký service), `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (sửa), `.../internal/domain/notification_event.go` (sửa) và `_test.go` tương ứng
**Depends on:** TASK-REQ-034-01, 034-02; TASK-REQ-024-02 (`AuditRecorder`); BE-REQ-SOL-035 task 04 (nhóm hành động `admin`; trong lúc chờ kiểm `tenant.Role`)
**Status:** [ ] TODO

---

## Context

- CR 034 mục 2.1: "CRUD ngân sách: RPC quản trị `AiBudgetAdminService` (`List`, `Upsert`, `Delete`; chỉ `role=admin`)". Mục 7 câu 1 hỏi README 3.6 chưa liệt kê service này; chỉ người điều phối sửa README, task này không sửa.
- `tenant.Role(ctx)` có từ `common/tenant/tenant.go:84`; `x-orca-role` rỗng phải coi là không phải admin (CR 035 mục 1 điểm 2). Mã `REQUEST_FORBIDDEN` do BE-REQ-SOL-035.
- `notification-service`: `Subjects` ở `consumer.go` (`{StreamName, Subject, Durable}`), `subjectRules` và `TranslateEvent` ở `notification_event.go:263`; `recipientsOf(payload)` cần `user_ids` trong payload, thiếu thì `ErrNoRecipients`. Mẫu thêm subject: TASK-REQ-010-05 (cùng stream `REQUEST`, tên stream phải khớp `EnsureStream` của `request-service`; chưa kiểm chứng vì service chưa có).
- `usage-service` và `ai-provider-service` **không** đổi (CR 034 phần Tác động).
- Audit `ai.budget.set`, `ai.egress.set`, `ai.trace.set` qua `AuditRecorder` (TASK-REQ-024-02); metadata không chứa nội dung, chỉ id, giá trị enum, số.

## Việc cần làm

1. `ai_admin.proto` (`package orca.request.v1`, service `AiBudgetAdminService`): 

```proto
service AiBudgetAdminService {
  rpc ListAiBudgets(ListAiBudgetsRequest) returns (ListAiBudgetsResponse);
  rpc UpsertAiBudget(UpsertAiBudgetRequest) returns (UpsertAiBudgetResponse);
  rpc DeleteAiBudget(DeleteAiBudgetRequest) returns (DeleteAiBudgetResponse);
  rpc ListAiStepPolicies(ListAiStepPoliciesRequest) returns (ListAiStepPoliciesResponse);
  rpc UpsertAiStepPolicy(UpsertAiStepPolicyRequest) returns (UpsertAiStepPolicyResponse);
  rpc GetAiTenantSettings(GetAiTenantSettingsRequest) returns (AiTenantSettings);
  rpc SetAiTenantSettings(SetAiTenantSettingsRequest) returns (AiTenantSettings);
}
message AiBudget { string id=1; string scope_kind=2; string scope_value=3; string period=4;
  optional int64 limit_tokens=5; optional int32 limit_calls=6; optional int32 limit_agent_seconds=7; optional string limit_cost_usd_est=8;
  double warn_ratio=9; string action=10; bool enabled=11; int64 version=12; string updated_by=13; google.protobuf.Timestamp updated_at=14; }
message AiTenantSettings { string ai_egress_mode=1; string ai_trace_level=2; string ai_gate_mode=3; }
```

   (Các message request/response còn lại đặt tên theo tên RPC; `expected_version` có ở Upsert; chạy `buf lint`, `buf breaking`.) Số tiền là chuỗi thập phân để tránh `double`.
2. `manage_ai_budgets.go`: `ManageAIBudgets{budgets, policies, settings, audit, outbox, tx}` với `List`, `Upsert(in)` (`AIBudget.Validate`, CAS theo `version`, `updated_by` lấy từ `tenant.UserID(ctx)`), `Delete`, `ListStepPolicies`, `UpsertStepPolicy` (`Validate`, kiểm mọi `Model` hoặc `Class` giải được qua `ModelClasses`; id model không có trong cấu hình thì cảnh báo trong phản hồi, không chặn), `GetSettings`, `SetSettings(egress, trace, gate)` (enum hợp lệ; `ai_gate_mode=enforce` bị **từ chối** ở v1: chỉ `off|shadow`, câu hỏi mở 5 của solution). Mỗi thao tác ghi gọi `AuditRecorder.Record` (`ai.budget.set` cho budget và step policy, `ai.egress.set` khi đổi `ai_egress_mode`, `ai.trace.set` khi đổi `ai_trace_level`) trong cùng luồng sau commit.
3. `server_ai_admin.go`: mọi RPC bắt đầu bằng `requireAdmin(ctx)` (`tenant.Role(ctx) == "admin"`, rỗng/khác ⇒ `PermissionDenied` `REQUEST_FORBIDDEN`); ánh xạ `ErrVersionConflict → Aborted`, `Validate lỗi → InvalidArgument` mã `REQUEST_AI_BUDGET_INVALID` (mới, thêm vào bảng lỗi); không log thân request.
4. Đăng ký `AiBudgetAdminService` trong `main.go` và thêm các phương thức vào bảng phân loại của `flow_gate` (TASK-REQ-025-02: lớp `classSafeExit`? **Quyết định**: quản trị cài đặt AI luôn cho phép khi cờ tắt (admin cần đặt ngân sách trước khi bật cờ), nên lớp `classRead` cho `List*`/`Get*` và `classSafeExit` cho `Upsert*`/`Delete*`/`Set*`; ghi lý do trong comment ngắn).
5. `notification-service/consumer.go`: thêm `{StreamName: "REQUEST", Subject: "orca.request.ai_budget.warning"}` và `{StreamName: "REQUEST", Subject: "orca.request.ai_budget.exceeded", Durable: "notification-service-request-ai-budget-exceeded"}` (vượt hạn là mất sự kiện có hại, dùng `Durable` như mẫu `INFRAFLEET terminal.closed`).
6. `notification_event.go`: `subjectRules` thêm `Type: "request.ai_budget_warning"`, `Title: "AI budget nearly used"`, `Severity: SeverityWarning`, `Channels: ws`, `DeepLink: "/?section=settings"`; `Type: "request.ai_budget_exceeded"`, `Title: "AI budget exceeded"`, `Severity: SeverityCritical`, `Channels: ws + push`. Không Locked (title do payload không cần; dùng `rule.Title`); payload có `user_ids`.
7. Payload golden `testdata/ai_budget_warning.json`: `{"tenant_id":"…","budget_id":"…","scope":"tenant:day","ratio":0.83,"user_ids":["…"]}` (sao vào `notification-service/internal/domain/testdata`, không import chéo service).
8. Cấu hình: `REQUEST_AI_PRICE_TABLE`, `REQUEST_AI_MODEL_CLASSES` đọc ở `config.go` (đã mô tả ở task 02) và truyền vào `ManageAIBudgets` cho kiểm tra `UpsertAiStepPolicy`.

## Kiểm thử

- `manage_ai_budgets_test.go` (repo giả): upsert hợp lệ; xung đột `version`; `scope_kind=tenant` kèm `scope_value` lỗi; `SetSettings` với `ai_gate_mode=enforce` bị từ chối; mỗi thao tác ghi đúng một `AuditEvent` (`ai.budget.set`/`ai.egress.set`/`ai.trace.set`), metadata không có nội dung.
- `server_ai_admin_test.go`: người không phải admin, `x-orca-role` rỗng, `user` ⇒ `PermissionDenied` cho cả bảy RPC (bảng); admin thành công.
- `notification_event_test.go`: `TranslateEvent` với golden: đúng `Type`, người nhận, kênh; không `user_ids` ⇒ `ErrNoRecipients`; subject cũ không đổi.
- `consumer_test.go`: hai binding có mặt, `Durable` đúng tên.
- Hợp đồng: `buf lint && buf breaking --against '.git#branch=main'` trong `backend-go/proto`.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpc/... && cd services/notification-service && go test ./internal/domain/... ./internal/adapter/eventbus/...`.

## Tiêu chí hoàn thành

- [ ] Bảy RPC admin chỉ chạy được bởi `role=admin`.
- [ ] Mỗi thay đổi ngân sách, egress, trace ghi audit đúng một bản, không chứa nội dung.
- [ ] `ai_gate_mode=enforce` bị từ chối ở v1.
- [ ] Hai subject mới được `TranslateEvent` chấp nhận; test cũ của `notification-service` không đổi.
- [ ] `buf breaking` xanh.

## Rủi ro và lưu ý

- Người nhận chỉ là `updated_by`: nếu người đó rời tổ chức, không ai nhận cảnh báo (Q1 của solution).
- Tên stream `REQUEST` chưa kiểm chứng (service chưa có); xác nhận với SOL-001 mục C3 (`EnsureStream`) trước khi merge và báo chủ `notification-service` triển khai lại.
- Danh sách subject cố định trong mã: đổi cần deploy lại notification-service.
- `ListAiBudgets` có thể trả nhiều dòng: phân trang chưa cần ở v1 (giới hạn 200 dòng, ghi `truncated`).

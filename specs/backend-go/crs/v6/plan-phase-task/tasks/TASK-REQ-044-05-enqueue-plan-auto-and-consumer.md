# TASK-REQ-044-05: `EnqueuePlanAuto` và consumer bền `request-service-plan-auto-generate`

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/enqueue_plan_auto.go` (mới), `internal/usecase/enqueue_plan_auto_test.go` (mới), `internal/adapter/eventbus/plan_auto_consumer.go` (mới), `internal/adapter/eventbus/plan_auto_consumer_test.go` (mới)
**Depends on:** TASK-REQ-044-02, TASK-REQ-044-04
**Status:** [x] DONE (2026-10-20)

---

## Context

- Mẫu trực tiếp: `ResumeAfterClarification.Handle` (`usecase/resume_after_clarification.go:84`) và `ClarificationResumeConsumer` (`adapter/eventbus/clarification_resume_consumer.go:15`): durable riêng trên subject `domain.SubjectRequestStatusChanged`, lọc trigger trước, dấu `processed.MarkProcessed(txCtx, ev.ID, domain.SubjectRequestStatusChanged)` cùng transaction, trả lỗi **chỉ** khi giao lại có ích (bus giao lại ngay, không trễ: `common/eventbus/eventbus.go:123-155`).
- `processed_events` khoá chính `(tenant_id, event_id)` (`migrations/postgres/0001_init.up.sql:19`): một sự kiện chỉ được một consumer đánh dấu; bộ lọc của consumer này phải rời với `classification` (`to=classifying`) và `resume` (`trigger=information_provided`) — điều kiện `to=planning` thoả.
- `statusChangedPayload` của package `eventbus` (`classification_consumer.go:43`) chỉ có `request_id`, `to`, `trigger`; cần thêm `project_id`, `type`, `reporter_id` (có sẵn trong `usecase.StatusChangedPayload`, `transition_request_events.go:15-37`).
- `latestApprovedAnalysis(ctx, solutions, requestID)` (`run_plan_generation.go`) trả Solution đã duyệt mới nhất.

## Việc cần làm

1. `EnqueuePlanAuto` (`usecase`): phụ thuộc `PlanAutoJobRepository`, `ProcessedEventRepository`, `RequestRepository`, `SolutionStore`, `PlanAutoSwitch`, `PlanAutoPolicy`, `TxRunner`. `Handle(ctx, PlanAutoEvent{ID, TenantID, RequestID, ProjectID, To, Trigger})`:
   - lọc nhanh `To == planning` và trigger ∈ {`analysis_approved`, `type_confirmed`}; khác thì `nil` **trước** khi đánh dấu;
   - trong `tx.InTx`: `MarkProcessed`; đã xử lý thì thoát; `PlanAutoSwitch.Enabled` (lỗi hạ tầng → trả lỗi để giao lại; `false` → thoát và log mức debug); đọc Request, `FlowFor`, `domain.WantsAutoPlan` (sai → thoát, log mức debug với lý do); anchor = `SolutionAnchor(solutionID)` với loại có Analysis (không có Solution đã duyệt → không chèn, log cảnh báo), `TypeConfirmAnchor(ev.ID)` với `type_confirmed`; chèn job `pending`, `origin=event`, `attempt=1`, `max_attempts = 1 + policy.MaxExtraAttempts`, `next_check_at = now`, `project_id` từ Request. `inserted=false` là bình thường (log info, không lỗi).
   - không bao giờ trả lỗi vì lý do nghiệp vụ.
2. `PlanAutoConsumer` (`eventbus`): durable `request-service-plan-auto-generate`, `Run(ctx)`/`Handle(ctx, ev)` như mẫu: payload hỏng hoặc không có `request_id` → ack và log cảnh báo (không nội dung); thiếu `TenantID` → ack; gọi `EnqueuePlanAuto.Handle` với `tenant.WithTenantID`; lỗi thì ghi cảnh báo "will be redelivered" rồi trả lỗi.
3. Mở rộng struct payload của consumer, hoặc dùng struct riêng của consumer này, để đọc `project_id`, `type`; không sửa `statusChangedPayload` đang dùng bởi hai consumer khác nếu điều đó làm đổi test của chúng.

## Kiểm thử

- `EnqueuePlanAuto` với fake: (a) `analysis_approved` của `bug` chèn đúng một job với anchor là id Solution, `max_attempts=2`; (b) giao lại cùng sự kiện → vẫn một job (dấu `processed_events`); (c) công tắc env tắt hoặc dự án tắt → không job; (d) `spike`, `question`, `hotfix`, `plan_revision`, Request đã có `PlanTaskID` → không job; (e) `type_confirmed` của `task` chèn anchor `ty-<eventId>`; của `bug` thì không; (f) đã có job hoạt động (ví dụ job `retry`) → không chèn, không lỗi; (g) lỗi đọc cài đặt dự án → trả lỗi, **không** chèn; (h) lỗi cơ sở dữ liệu của chính transaction → trả lỗi.
- Consumer: payload hỏng → ack; không tenant → ack; trigger khác → ack không gọi usecase; lỗi usecase → trả lỗi.
- Bảng 11 loại × trigger đã có ở task 01; ở đây chỉ kiểm chọn đúng.
- `cd backend-go/services/request-service && go test ./internal/usecase/... ./internal/adapter/eventbus/... -run 'EnqueuePlanAuto|PlanAutoConsumer' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Giao lại cùng sự kiện hai lần chỉ tạo một job.
- [x] Không có job cho `spike`, `question`, `hotfix`, replan, Request đã có Plan, hay khi một công tắc tắt.
- [x] Lỗi cài đặt hoặc hạ tầng được giao lại; lỗi nghiệp vụ thì ack.
- [x] Không nội dung Request trong log.

## Rủi ro và lưu ý

- Một sự kiện chỉ được một consumer đánh dấu; nếu sau này thêm consumer khác cho `analysis_approved`, `processed_events` sẽ làm chúng tranh nhau (xem Context). Ghi chú điều đó ở đầu file consumer.
- Thứ tự: sự kiện đến sau commit của transition nên Request đã `planning`; nhưng vẫn đọc lại Request (có thể đã bị người đổi trong lúc chờ).

## Ghi chú triển khai

**Hoàn thành 2026-10-20.** Đường dẫn dưới `backend-go/services/request-service/`.

- `internal/usecase/enqueue_plan_auto.go`: `EnqueuePlanAuto.Handle` lọc `to=planning` và trigger `analysis_approved`/`type_confirmed` trước khi đánh dấu; trong một `InTx`: `MarkProcessed`, đọc Request (không tìm thấy thì ack), `PlanAutoSwitch.Enabled` (lỗi đọc trả lỗi để giao lại, giao lại được vì dấu `processed` rollback cùng transaction), Request còn `planning`, `domain.WantsAutoPlan`, anchor (`SolutionAnchor` bằng cách đọc thẳng `ListByRequest` để lỗi đọc không bị nuốt thành "không có Solution"; `TypeConfirmAnchor(eventID)`), chèn job `pending`/`event` với `MaxAttempts = 1 + MaxExtraAttempts`. Chèn trùng chỉ ghi log info. `internal/adapter/eventbus/plan_auto_consumer.go`: durable `request-service-plan-auto-generate`, struct payload riêng (`request_id`, `project_id`, `to`, `trigger`), ghi chú đầu type về ràng buộc `processed_events` theo `event_id`.
- Test: `enqueue_plan_auto_test.go` (13 ca: anchor/`max_attempts=2`, giao lại 3 lần còn một job, job `retry` đang hoạt động, bảng ba công tắc, lỗi đọc cài đặt rồi giao lại thành công, lỗi chèn, `spike`/`question`/`hotfix`/trigger khác/trạng thái khác, đã có `PlanTaskID`, Request đã rời `planning`, `type_confirmed` của `task` có anchor `ty-<id>` còn `bug` thì không, không có Solution đã duyệt, Request không tồn tại, `MaxExtraAttempts=0`), `plan_auto_fakes_test.go` (kho job/processed trong bộ nhớ có rollback theo transaction, dùng chung với task 06 và 07), `plan_auto_consumer_test.go` (ack payload hỏng, không request, không tenant, trigger/status khác, trả lỗi khi use case lỗi).
- Lệnh: `go test ./internal/usecase/ ./internal/adapter/eventbus/ -run 'EnqueuePlanAuto|PlanAutoConsumer'` xanh. Test NATS thật của consumer không viết (mẫu `*_nats_integration_test.go` cần NATS; thuộc task 09).
- Lệch spec: không. Chưa kiểm: giao lại sự kiện qua `processed_events` thật hai dialect (task 09).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

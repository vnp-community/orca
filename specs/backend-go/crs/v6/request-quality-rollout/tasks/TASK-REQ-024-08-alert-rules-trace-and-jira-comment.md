# TASK-REQ-024-08: Luật cảnh báo, `traceparent` trong payload và bình luận Jira tuỳ chọn

**From Solution:** BE-REQ-SOL-024
**Priority:** P2
**Service:** `request-service`, `issue-status-sync`, `deploy`
**File:** `backend-go/deploy/alerts/request.rules.yaml` (mới), `backend-go/services/request-service/internal/adapter/outbox/` (thêm `traceparent` vào payload), `backend-go/services/issue-status-sync/internal/usecase/sync_request_status.go`, `.../internal/usecase/ports.go` (`IssueCommenter`), `.../internal/adapter/grpcclient/issuetracking_client.go`
**Depends on:** TASK-REQ-024-04, TASK-REQ-024-07
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./... -count=1` trong `services/request-service` (512 test PASS, 0 FAIL) và `services/issue-status-sync` (186 PASS); test `TestAlertRulesQueryOnlySeriesThatExist` và `TestTransitionPayloads_*`)

---

## Context

- `deploy/alerts/mcp.rules.yaml`: nhóm `orca-mcp`, `alert`, `expr`, `for`, `labels: {severity}`, `annotations`; ghi chú "Not loaded by anything in this repository" (không có Prometheus trong repo).
- `common/eventbus.Event` không có chỗ cho trace; giải pháp D6: `traceparent` trong payload (SOL-024 mục 1 điểm 5). `common/tracing.Init(ctx, serviceName, otlpEndpoint, opts...)` đã có.
- `issue-tracking-service` có RPC `AddIssueComment` (`proto/orca/issuetracking/v1`, dòng 29).
- Bình luận mặc định tắt (`ISSUE_SYNC_REQUEST_COMMENTS_ENABLED=false`).

## Việc cần làm

1. `request.rules.yaml`: nhóm `orca-request` với 6 alert của CR-REQ-024 mục 2.11 (`RequestApprovalBacklog`, `RequestStuck`, `RequestAIGenerationFailures`, `RequestOutboxLag`, `RequestReturnedSpike`, `IssueSyncRequestFailures`), dùng đúng tên metric ở task 07; comment đầu file ghi "đề xuất, chưa hiệu chỉnh, chưa có nơi nạp".
2. `request-service`: khi dựng payload `status_changed` và `completed`, thêm `traceparent` từ span hiện tại (`otel` propagator `TraceContext`); trường tuỳ chọn, `omitempty`.
3. `issue-status-sync`: đọc `traceparent` trong `handleRequestEvent`, tạo span mới có `trace.Link` tới ngữ cảnh đó quanh `HandleRequestStatus`. Không có thì tạo span mới.
4. `IssueCommenter` (cổng) và adapter `AddIssueComment`; khi cấu hình bật, sau khi chuyển Jira thành công hoặc khi `request_backlog`: gửi bình luận ngắn (số Request, loại, trạng thái, liên kết Orca); **không** kèm `body`, Solution. Best effort: lỗi chỉ tăng metric.

## Kiểm thử

- `promtool check rules backend-go/deploy/alerts/request.rules.yaml` (cần `promtool`, chưa kiểm chứng trong repo); nếu không có thì kiểm YAML hợp lệ bằng test Go nhỏ (đọc, unmarshal, tên alert).
- Unit: payload có `traceparent` khi span hợp lệ; consumer tạo link; bình luận không chứa `body` (test với marker); bật/tắt cấu hình.
- `go test ./...` ở hai service.

## Tiêu chí hoàn thành

- [x] `request.rules.yaml` hợp lệ cú pháp, metric tồn tại trong task 07.
  Test Go đọc và unmarshal YAML, kiểm 6 tên alert và mọi series nằm trong bảng 2.9. `promtool` không có trong môi trường nên chưa kiểm chứng bằng `promtool check rules`.
- [x] Bình luận tắt mặc định; khi bật không rò nội dung.
  Mặc định không có commenter; test khẳng định bình luận chỉ có số, loại, trạng thái, liên kết, không rò `title`/`body` của payload.
- [x] `traceparent` không phá consumer cũ (trường lạ bị bỏ qua).
  Consumer bỏ qua trường lạ (có test) và tạo span link khi có `traceparent`; phía `request-service` chưa phát trường này.

## Rủi ro và lưu ý

- Ngưỡng cảnh báo là giả định.
- Nếu nhóm quyết định sửa `common/eventbus.Event` thay vì payload (Q của CR), task này đổi thành thêm header; ghi vào mô tả PR.

## Tiến độ (2026-10-07)

Đã làm: `backend-go/deploy/alerts/request.rules.yaml` (nhóm `orca-request`, 6 alert, comment đầu file "đề xuất, chưa hiệu chỉnh, chưa có nơi nạp"); `adapter/eventbus/request_trace_link.go` (span mới có `trace.Link` tới `traceparent` trong payload, không có thì span trơn; test bằng `tracetest.SpanRecorder`); cổng `IssueCommenter`, `IssueTrackingClient.AddComment` (gọi `AddIssueComment`), option `WithIssueComments`, cấu hình `ISSUE_SYNC_REQUEST_COMMENTS_ENABLED` (mặc định false) và `ISSUE_SYNC_ORCA_BASE_URL`. Bình luận gửi sau chuyển Jira thành công và khi `request_backlog`, best effort (lỗi tăng `comment:failed`).

Còn thiếu: `request-service` đặt `traceparent` vào payload `status_changed`/`completed` (outbox của request-service đang do agent khác sửa); `promtool check rules` chưa chạy.

## Kết quả triển khai (2026-10-08)

`traceparent` đặt vào payload `status_changed`/`completed` từ span `request.Transition` (e2e xác nhận có trong outbox). Mọi series trong `request.rules.yaml` đã được đối chiếu với series request-service thật. `promtool check rules` chưa chạy (không có công cụ). Bình luận Jira chưa chạy với Jira thật.

# TASK-REQ-024-08: Luật cảnh báo, `traceparent` trong payload và bình luận Jira tuỳ chọn

**From Solution:** BE-REQ-SOL-024
**Priority:** P2
**Service:** `request-service`, `issue-status-sync`, `deploy`
**File:** `backend-go/deploy/alerts/request.rules.yaml` (mới), `backend-go/services/request-service/internal/adapter/outbox/` (thêm `traceparent` vào payload), `backend-go/services/issue-status-sync/internal/usecase/sync_request_status.go`, `.../internal/usecase/ports.go` (`IssueCommenter`), `.../internal/adapter/grpcclient/issuetracking_client.go`
**Depends on:** TASK-REQ-024-04, TASK-REQ-024-07
**Status:** `[ ] TODO`

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

- [ ] `request.rules.yaml` hợp lệ cú pháp, metric tồn tại trong task 07.
- [ ] Bình luận tắt mặc định; khi bật không rò nội dung.
- [ ] `traceparent` không phá consumer cũ (trường lạ bị bỏ qua).

## Rủi ro và lưu ý

- Ngưỡng cảnh báo là giả định.
- Nếu nhóm quyết định sửa `common/eventbus.Event` thay vì payload (Q của CR), task này đổi thành thêm header; ghi vào mô tả PR.

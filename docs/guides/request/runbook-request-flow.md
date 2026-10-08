# Runbook vận hành luồng Request

**Cập nhật:** 2026-10-08 · Đối chiếu với: [CR-REQ-025 mục 2.8](../../crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md),
`backend-go/services/request-service/internal/adapter/metrics/`, `backend-go/deploy/alerts/request.rules.yaml`,
[CR-REQ-024](../../crs/v6/request-quality-rollout/CR-REQ-024-jira-status-sync-audit-observability.md).

## Nơi xem

| Nguồn | Cách xem |
|---|---|
| Log `request-service` | log container/tiến trình `request-service` (ví dụ `docker compose logs request-service` ở `deploy/dev`) |
| Metric | cổng HTTP của `request-service`, đường `/metrics` (cùng cổng với `/healthz`, `/readyz`) |
| Metric đồng bộ Jira | `/metrics` của `issue-status-sync` |
| Cờ luồng | kênh WS `request.flowStatus`, hoặc RPC `GetRequestFlowSettings` |
| Audit | audit log của `auth-service`, action `request.*`, `approval.*`, `solution.choose`; metadata không chứa tiêu đề hay nội dung |

## Metric của `request-service`

`orca_request_created_total`, `orca_request_transitions_total`, `orca_request_classification_total`, `orca_request_classification_confidence`,
`orca_request_approvals_total`, `orca_request_approval_wait_seconds`, `orca_request_approvals_pending`, `orca_request_stuck`,
`orca_request_returned_total`, `orca_request_ai_generation_seconds`, `orca_request_task_outcome_total`, `orca_request_outbox_pending`.

- Ngưỡng `stuck` mặc định: `classifying` 10 phút, `analyzing` 30, `planning` 30. Đổi bằng `REQUEST_STUCK_THRESHOLD_<TRẠNG_THÁI>` (ví dụ
  `REQUEST_STUCK_THRESHOLD_CLASSIFYING=15m`). Chu kỳ lấy mẫu gauge: `REQUEST_METRICS_SAMPLE_INTERVAL` (mặc định 30s).
- **Chưa có** số liệu `orca_request_ai_generation_seconds` cho solution/plan và `orca_request_task_outcome_total` cho tới khi các CR đó gọi bộ quan sát
  (CR-REQ-007/008/012/013). Hiện chỉ phân loại có số liệu AI; `task_outcome` luôn rỗng.

Metric của `issue-status-sync`: `orca_issuesync_request_events_total{event,result}`, `orca_issuesync_skipped_request_owned_total{source}`,
`orca_issuesync_jira_transition_seconds`.

## Luật cảnh báo

`backend-go/deploy/alerts/request.rules.yaml` có 6 alert: `RequestApprovalBacklog`, `RequestStuck`, `RequestAIGenerationFailures`, `RequestOutboxLag`,
`RequestReturnedSpike`, `IssueSyncRequestFailures`. Đây là **đề xuất**: ngưỡng chưa hiệu chỉnh và **chưa có nơi nạp** file này, nên hiện không alert nào tự bắn.
Phải xem metric bằng tay hoặc nạp file vào Prometheus.

## Khẩn cấp: tắt cờ

Khi nghi ngờ mất dữ liệu, Jira bị chuyển sai hàng loạt, hoặc luồng chạy ngoài kiểm soát: **tắt cờ trước, điều tra sau**. Các bước chính xác
(tắt tenant bằng `request.flowSet {enabled:false}`, hoặc `REQUEST_FLOW_ENABLED=false` rồi khởi động lại) ở
[Bật luồng Request, mục Rollback](./admin-enable-request-flow.md). Đọc, huỷ, trả về backlog, `Reject`/`Cancel` Approval và xử lý callback vẫn chạy khi tắt.
Trạng thái Jira đã chuyển không tự lùi.

## Triệu chứng, kiểm tra, xử lý

| Triệu chứng | Kiểm | Lệnh / nơi xem | Xử lý |
|---|---|---|---|
| Request kẹt `classifying` | `orca_request_stuck{status="classifying"}`, `orca_request_ai_generation_seconds`, `orca_request_classification_total` | `/metrics`; log `request-service` tìm "request classification failed" (có lý do: no dev server connected, classifier timeout, invalid classifier output) | kiểm `ai-provider-service` và dev server agent đang kết nối (đường `ai.complete` qua `infra-fleet-service`); gọi lại `ClassifyRequest` (kênh `request.classify`); tối đa 5 lần mỗi Request, quá thì dùng `ChangeRequestType` để người chọn loại hoặc `ReturnToBacklog` |
| Request kẹt `analyzing` / `planning` | `orca_request_stuck{status=...}` | `/metrics`, `GetRequestFlow` | **Hiện là bình thường**: các giai đoạn Solution/Plan chưa có (CR-REQ-007/008/012). Dùng `ReturnToBacklog` hoặc `CancelRequest` nếu cần dọn |
| Sự kiện không tới `issue-status-sync` | `orca_request_outbox_pending`, consumer JetStream, `orca_issuesync_request_events_total{result}` | `/metrics` của cả hai service; log `issue-status-sync` | kiểm NATS; `result=skipped_*` cho biết lý do (`skipped_no_actor`, `skipped_transition_unavailable`, `skipped_request_owned`, `skipped_stale`, `skipped_not_jira`, `skipped_category`) |
| Jira không chuyển | `result=skipped_no_actor`, `skipped_transition_unavailable`, `skipped_request_owned` | `/metrics` của `issue-status-sync`; log | kết nối Jira của người dùng; workflow Jira có đúng tên trạng thái cấu hình (`ISSUE_SYNC_JIRA_STATUS_IN_PROGRESS` / `_DONE`) không. `skipped_request_owned` là đúng thiết kế khi issue có Request chưa kết thúc |
| Lookup Request từ `issue-status-sync` lỗi | `result=failed` | log `issue-status-sync`; kiểm `REQUEST_SERVICE_ADDR`, `REQUEST_SERVICE_INTERNAL_TOKEN` khớp `SERVICE_INTERNAL_TOKEN` của `request-service` | sự kiện được giao lại tối đa 3 lần rồi bỏ; sửa cấu hình rồi đồng bộ lại bằng tay (Jira đổi tay) |
| Approval chất đống | `orca_request_approvals_pending` theo `subject_type`, `orca_request_approval_wait_seconds` | `/metrics`; hộp duyệt (`approval.listPending`) | báo người duyệt; kiểm `REQUEST_APPROVAL_ENABLED` (compose dev mặc định `false`); Approval quá hạn do bộ quét (`REQUEST_APPROVAL_SWEEP_INTERVAL`) tự hết hạn và đưa Request về backlog; có thể `ExtendApproval` |
| Duyệt bị từ chối (`REQUEST_APPROVAL_NOT_APPROVER`, `..._SELF_APPROVAL_FORBIDDEN`) | audit `approval.approve` với `outcome=denied` | audit log | xem chính sách người duyệt ([Phê duyệt](./approving-requests.md)); admin sửa bằng `UpsertApprovalPolicy` |
| Mở Approval báo `REQUEST_APPROVAL_SUBJECT_UNAVAILABLE` | log khởi động "approval subject has no backing service bound" | log `request-service` | **Hiện là bình thường** cho mọi cổng trừ `request_type` (chưa có handler) |
| Lệnh báo `REQUEST_FLOW_DISABLED` | `GetRequestFlowSettings` | kênh `request.flowStatus` | cần cả `REQUEST_FLOW_ENABLED=true` và cài đặt tenant bật ([Bật luồng](./admin-enable-request-flow.md)) |
| Phase / task không chạy | `orca_request_task_outcome_total`, lease của `task-service` | `/metrics`; log `task-service` | **Chưa có** đường thực thi từ Request (CR-REQ-013); chưa áp dụng được |
| Outbox đầy | `orca_request_outbox_pending` tăng liên tục | `/metrics`; log relay outbox của `request-service`; NATS | kiểm NATS còn sống và stream tồn tại; Jira sync, thông báo, audit trễ tới khi relay hồi phục |

## Diễn tập rollback

**Trạng thái: chưa diễn tập ở dev.** Chưa ai chạy kế hoạch rollback; mục này không có kết quả để ghi. Diễn tập là điều kiện vào giai đoạn Beta
của rollout.

Các bước cần làm khi diễn tập (ghi kết quả thật vào đây sau khi chạy):

1. Dựng stack dev với `REQUEST_FLOW_ENABLED=true`, `REQUEST_APPROVAL_ENABLED=true`; admin bật `request.flowSet {enabled:true}` cho một tenant thử.
2. Tạo vài Request: một đang `classifying`, một ở `awaiting_type_confirmation` với Approval `request_type` `pending`, một đã xác nhận ở `analyzing`.
3. Tắt cờ tenant. Kiểm: `CreateRequest`, `ClassifyRequest`, `ConfirmRequestType`, `Approve` trả `REQUEST_FLOW_DISABLED`; `GetRequest`, `ListRequests`,
   `CancelRequest`, `ReturnToBacklog`, `Reject` còn chạy; tenant khác không bị ảnh hưởng.
4. Kiểm audit có `request.flow.set` (cả lần Set bị từ chối khi gọi bằng người không phải admin).
5. Bật lại; Approval còn `pending` xử lý tiếp được. Tắt lần nữa bằng `REQUEST_FLOW_ENABLED=false` và khởi động lại; xác nhận kết quả như bước 3.
6. Với issue Jira thử (nếu có): ghi nhận trạng thái Jira không tự lùi.
7. Ghi ngày, người chạy, lệnh, kết quả, và mọi sai lệch so với tài liệu vào mục này. Test tự động tương ứng là kịch bản E20 trong T1
   (`e2e/feature_flag_test.go`); nó không thay cho diễn tập trên stack dev.

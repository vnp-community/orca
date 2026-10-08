# Bật, tắt và rollout luồng Request (dành cho admin và ops)

**Cập nhật:** 2026-10-08 · Đối chiếu với: `backend-go/services/request-service/internal/adapter/grpc/flow_gate.go` (`flowMethodClass`),
`internal/usecase/flow_settings.go`, `internal/domain/flow_settings.go`, `deploy/dev/docker-compose.yml`,
[CR-REQ-025 mục 2.2, 2.6, 2.7](../../crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md).

## Cờ hai tầng

| Tầng | Nơi đặt | Mặc định |
|---|---|---|
| Tổng | biến `REQUEST_FLOW_ENABLED` của `request-service` (compose dev: `${REQUEST_FLOW_ENABLED:-false}`) | `false` |
| Theo tenant | dòng `request.tenant_settings` của tenant, qua RPC `SetRequestFlowSettings` / `GetRequestFlowSettings` | không có dòng = tắt |

**Hiệu lực = cả hai bật.** Thiếu dòng hoặc lỗi đọc = tắt (fail closed). `GetRequestFlowSettings` trả giá trị **hiệu lực**.
Cờ theo **loại** Request chưa có cơ chế (cờ chỉ theo tenant); câu hỏi mở Q1 của CR-REQ-025. `task-service` không đọc cờ; cờ không ẩn hay xoá dữ liệu.

## Bật cho tenant

1. Ops đặt `REQUEST_FLOW_ENABLED=true` cho `request-service` (ở dev: biến môi trường khi chạy `deploy/dev/docker-compose.yml`) rồi khởi động lại service.
   Để dùng cổng phê duyệt trên stack dev cần thêm `REQUEST_APPROVAL_ENABLED=true` (compose dev mặc định `false`).
2. Admin của tenant gửi kênh WS `request.flowSet` với `{enabled:true}` (hoặc gọi trực tiếp RPC `SetRequestFlowSettings`). Kênh `request.flowStatus` đọc
   giá trị hiện hành.
3. Với `issue-status-sync`: đặt `REQUEST_SERVICE_ADDR` và `REQUEST_SERVICE_INTERNAL_TOKEN` (trùng `SERVICE_INTERNAL_TOKEN` của `request-service`),
   xem [Jira ↔ Orca](../jira/jira-orca-mapping.md). Token rỗng thì `LookupRequestBySource` bị từ chối.

Chỉ role `admin` được Set; người khác nhận lỗi `REQUEST_FLOW_ADMIN_ONLY`. Mỗi lần Set ghi audit `request.flow.set`, **kể cả lần bị từ chối**
(`outcome=denied`).

## Khi cờ tắt, RPC nào còn chạy

| Nhóm | RPC | Khi tắt |
|---|---|---|
| Đọc | `GetRequest`, `ListRequests`, `GetRequestFlow`, `ListRequestTypeHistory`, `ListRequestLinks`, `ListApprovals`, `GetApproval`, `ListPendingForUser`, `GetRequestFlowSettings`... | chạy |
| Thoát an toàn | `CancelRequest`, `ReturnToBacklog`, `Reject` và `Cancel` của Approval | chạy |
| Nội bộ | `ReportTaskOutcome`, `LookupRequestBySource`, consumer outbox, hết hạn Approval | chạy (không mất kết quả việc đang chạy) |
| Đi tiếp | `CreateRequest`, `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType`, `ReopenRequest`, `SpawnChildRequest`, `Approve`, `RequestApproval`, `ExtendApproval` và các RPC đi tiếp khác | `FailedPrecondition` `REQUEST_FLOW_DISABLED` |

Bảng phân loại đầy đủ là `flowMethodClass` trong `flow_gate.go`; test quét proto đảm bảo mọi RPC có một dòng. Một số RPC trong bảng đó
(ví dụ `GenerateSolution`, `StartPhase`) chưa có handler thật nên trả `Unimplemented`.

## Rollout đề xuất (5 giai đoạn)

Số ngày và số Request là **đề xuất ban đầu, chưa có số đo thực**. Hiện chưa có tenant nào chạy các giai đoạn này.

| GĐ | Phạm vi | Điều kiện vào | Điều kiện chuyển tiếp |
|---|---|---|---|
| 0. Nội bộ | `REQUEST_FLOW_ENABLED=true` ở dev; một tenant thử | T1 xanh hai dialect; `check-request-service-wiring.sh` xanh | không có alert `RequestOutboxLag`, `RequestStuck` trong 3 ngày |
| 1. Dogfood | tenant đội Orca, mọi loại | T2 xanh 5 đêm liên tiếp; một issue Jira thật chạy hết kịch bản E18 bằng tay | 2 tuần không mất dữ liệu; `orca_request_stuck` về 0 sau mỗi lần xử lý |
| 2. Beta | tenant opt-in (admin bật `request.flowSet`); loại rủi ro thấp trước: `task`, `docs`, `question`, `spike` | review quyền duyệt; runbook được diễn tập | `orca_request_returned_total` không tăng bất thường; không Jira nào bị chuyển sai |
| 3. Mở rộng loại | thêm `bug`, `refactor`, `change_request`; sau cùng `security`, `performance`, `ops_request`, `hotfix` | các loại này cần năng lực agent chưa kiểm chứng | mỗi loại có ít nhất 5 Request thật hoàn tất |
| 4. GA | cờ theo tenant bật được; mặc định `false` của biến tổng không đổi cho tới quyết định riêng | review bảo mật độc lập | |

"Bật theo loại" ở GĐ 2 và 3 **chưa có cơ chế** (cờ chỉ theo tenant); hiện chỉ có thể hướng dẫn tenant chỉ dùng một số loại khi xác nhận.
Ngoài ra các giai đoạn Solution, Plan, Phase chưa có ([README](./README.md)), nên GĐ 1 trở đi chưa thể chạy trọn vẹn cho tới khi CR-REQ-007, 008, 012, 013 xong.

## Rollback (7 bước)

1. **Tắt cờ**: `request.flowSet {enabled:false}` cho tenant, hoặc `REQUEST_FLOW_ENABLED=false` rồi khởi động lại `request-service`. Có hiệu lực ngay cho ghi mới.
2. Request đang `executing`: cho chạy xong (callback vẫn xử lý) hoặc `ReturnToBacklog`.
3. Approval đang `pending`: `Reject` hoặc `Cancel` được; sau khi bật lại, người duyệt xử lý tiếp.
4. **Không chạy `down` migration** của `task-service` (`CHECK` loại `plan`, `phase`) khi đã có dòng loại đó. Dữ liệu giữ nguyên.
5. Jira: trạng thái đã chuyển không tự lùi; đổi tay nếu cần.
6. Kill switch riêng cho tool MCP `request_*`: tool chưa có trên nhánh này ([Request từ agent](./requests-from-agents-mcp.md)).
7. Diễn tập rollback là điều kiện của GĐ 2. **Chưa diễn tập** (xem cuối [runbook](./runbook-request-flow.md)).

Sự cố khẩn: xem [runbook](./runbook-request-flow.md) mục "Khẩn cấp: tắt cờ".

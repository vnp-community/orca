# Tạo và phân loại Request

**Cập nhật:** 2026-10-08 · Đối chiếu với: `internal/domain/request_source.go`, `internal/usecase/create_request.go`,
`internal/usecase/propose_request_classification.go`, `internal/domain/request_confirmation_rules.go`.
Điều kiện: cờ luồng Request phải bật cho tenant, nếu không các lệnh dưới trả `REQUEST_FLOW_DISABLED`
([Bật luồng Request](./admin-enable-request-flow.md)).

## Nguồn của Request

`CreateRequest` nhận trường nguồn `provider` ∈ `jira`, `github`, `gitlab`, `linear`, `mcp`, `manual`, `webhook`, kèm `site`, `ref`, `url`.

| Nguồn | Hành vi đã có |
|---|---|
| `jira`, `linear` | nếu có `ref`, service lấy tiêu đề và nội dung từ issue (qua `issue-tracking-service`); không lấy được thì dùng nội dung người gọi gửi, trừ khi thiếu tiêu đề thì từ chối. Chưa kiểm chứng với Jira/Linear thật |
| `github`, `gitlab` | được nhận và lưu nguồn; **không** tự lấy nội dung, người gọi phải gửi tiêu đề |
| `manual` | nhập tay |
| `mcp` | tạo từ agent, ghi `actor_type=agent` trong audit ([Request từ agent](./requests-from-agents-mcp.md)) |
| `webhook` | `POST /v1/request-webhooks/{source_name}` trên `request-service`, ký HMAC-SHA256, tenant qua header `X-Orca-Tenant-Id`, thân tối đa 256 KiB; chỉ bật khi đặt `REQUEST_WEBHOOK_SOURCES`. `api-gateway` chưa chuyển tiếp tới cổng này |

Tạo lặp cùng (tenant, provider, site, ref) không sinh Request thứ hai nhờ khoá idempotency. Request con từ `SpawnChildRequest` đi qua
`CreateRequest` và vào `classifying` với gợi ý loại (`type_hint`).

Giao diện: kênh WS của `api-gateway` là `request.create`, `request.get`, `request.list`, `request.classify`, `request.confirmType`,
`request.changeType`, `request.typeHistory`. Phần tạo Request từ màn hình Jira/GitHub trong frontend thuộc CR-REQ-018 đến 023; tài liệu này
không khẳng định giao diện đó đã hoàn chỉnh.

## Phân loại bằng AI

`ClassifyRequest` chạy **bất đồng bộ** và trả `run_id`. Việc chạy nền có lease và tự phục hồi; gọi AI qua relay `ai.complete` của
`infra-fleet-service` (dev server agent). Tối đa 5 lần cho một Request (`classification_attempts`; mở lại Request đặt lại bộ đếm).

Kết quả thành công: Request sang `awaiting_type_confirmation`, có `type`, `size`, `urgency`, `confidence`, `classification_reason`, một dòng
trong lịch sử loại (`ListRequestTypeHistory`), và một Approval `request_type` được mở. Thất bại (không có dev server, đầu ra không hợp lệ, quá thời gian)
ghi lý do và tăng bộ đếm; chưa kiểm chứng với dev server agent thật.

## Xác nhận hoặc đổi loại

Người xác nhận loại qua `ConfirmRequestType` hoặc duyệt Approval `request_type` ([Phê duyệt](./approving-requests.md)). Luật xác nhận
(`ValidateConfirmation`):

| Luật | Mã lỗi / hành vi |
|---|---|
| Phải có loại | lỗi thiếu loại |
| `bug` và `refactor` phải có size | lỗi thiếu size |
| `hotfix` phải `urgent` | lỗi hotfix cần urgent |
| `hotfix` và `security` cần lý do không rỗng | lỗi cần lý do |
| `hotfix` và `security` bắt buộc **người** xác nhận | người yêu cầu không tự duyệt được (xem chính sách mặc định) |

Sau xác nhận: Request vào `analyzing`, trừ `task` và `docs` vào `planning`. `ChangeRequestType` đổi loại sau đó và ghi lịch sử;
`ListRequestTypeHistory` xem lại. Từ bước này trở đi các giai đoạn tiếp theo **chưa có** (xem [README](./README.md)).

## Thoát khỏi luồng

`ReturnToBacklog` (kèm danh mục lý do và giai đoạn, xem [Backlog](./request-and-task-backlogs.md)), `CancelRequest`, `ReopenRequest`
(từ backlog, phân loại lại).

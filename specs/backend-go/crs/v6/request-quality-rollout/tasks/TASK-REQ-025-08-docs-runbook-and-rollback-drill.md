# TASK-REQ-025-08: Tài liệu `docs/guides/request/`, runbook và diễn tập rollback

**From Solution:** BE-REQ-SOL-025
**Priority:** P1
**Service:** `docs`
**File:** `docs/guides/request/README.md`, `creating-and-classifying-requests.md`, `approving-requests.md`, `request-and-task-backlogs.md`, `requests-from-agents-mcp.md`, `admin-enable-request-flow.md`, `runbook-request-flow.md` (đều mới); cập nhật `backend-go/README.md`, `backend-go/services/api-gateway/README.md`, `docs/guides/mcp/README.md`, `docs/guides/jira/jira-orca-mapping.md`, cột "Trạng thái" của các CR v6 khi triển khai
**Depends on:** TASK-REQ-025-01..06; BE-REQ-SOL-024 (tên metric, alert); BE-REQ-SOL-016, 017
**Status:** `[x] DONE`

---

## Context

- Danh sách tài liệu và runbook: CR-REQ-025 mục 2.8 (bảng triệu chứng, kiểm, xử lý) và 2.7 (7 bước rollback).
- Đã có: `docs/guides/mcp/` (admin-guide, approving-agent-actions, connect-*), `docs/guides/jira/jira-orca-mapping.md`. `docs/guides/request/` chưa có.
- Ngôn ngữ tiếng Việt (docs v6). Không ghi "Real" cho RPC còn `Unimplemented` ở README service (quy ước README các service).
- Tên metric và alert: BE-REQ-SOL-024; kênh và mã lỗi: CONTRACT-request-ui-api.md.

## Việc cần làm

1. Viết 7 tài liệu theo CR mục 2.8, mỗi tài liệu chỉ nói điều đã hiện thực (khi viết, đối chiếu code thật; chỗ chưa có ghi "chưa có").
2. `runbook-request-flow.md`: bảng triệu chứng (Request kẹt `classifying`, sự kiện không tới `issue-status-sync`, Jira không chuyển, Approval chất đống, Phase không chạy, khẩn cấp), mỗi dòng nêu metric (`orca_request_stuck`, `orca_request_outbox_pending`, `orca_issuesync_request_events_total{result}`...), nơi xem log và cách xử lý; liên kết `request.rules.yaml`.
3. `admin-enable-request-flow.md`: cờ hai tầng, `request.flowSet`, biến `REQUEST_FLOW_ENABLED` ở compose, rollout 5 giai đoạn (điều kiện vào, điều kiện chuyển), rollback 7 bước, ghi rõ "bật theo loại chưa có cơ chế".
4. Cập nhật README: `backend-go/README.md` (hàng `request-service`: RPC nào thật, nào chưa), README gateway (đã ở TASK-REQ-016-07, chỉ kiểm lại), MCP guide (đã ở TASK-REQ-017-05, kiểm lại), `jira-orca-mapping.md` (Request sở hữu issue, đồng bộ theo Request).
5. **Diễn tập rollback ở dev** (điều kiện giai đoạn 2): tắt cờ giữa lúc có Request `executing` và Approval `pending`, thực hiện các bước 1 đến 5 của mục 2.7, ghi kết quả (ngày, người, quan sát, vấn đề) vào cuối runbook. Không thể chạy thì ghi "chưa diễn tập" và không đánh dấu hoàn thành tiêu chí này.
6. Không sửa file CR; không sửa `docs/crs/v6/README.md` (người điều phối).

## Kiểm thử

- Kiểm liên kết tương đối giữa các tài liệu bằng công cụ có sẵn (nếu repo có `markdown-link-check` hoặc script tương đương; nếu không, kiểm tay).
- Một người ngoài làm theo `admin-enable-request-flow.md` trên dev: ghi kết quả vào PR.

## Tiêu chí hoàn thành

- [x] 7 tài liệu có đủ, không mô tả tính năng chưa có như đã có.
- [x] README các service không ghi "Real" cho RPC `Unimplemented`.
- [x] Rollback được diễn tập một lần và ghi vào runbook (hoặc ghi rõ chưa).

## Rủi ro và lưu ý

- Tài liệu dễ lệch code; viết sau khi các task còn lại gần xong và đối chiếu lần cuối trước khi hợp nhất.
- Số ngày, số Request ở các giai đoạn rollout là đề xuất, chưa có số đo thực.

# SOL-E2E-001: Khoá hợp đồng giữa service, gateway, agent và frontend bằng kiểm tra tự động

**Resolves:** BUG-E2E-001, BUG-E2E-004, OBS-D (một phần) · **Nguyên nhân gốc:** RC-A
**Service:** `api-gateway`, `agent`, `request-service` (đọc), `frontend` (xem [SOL-FE-E2E-001](../../../../../frontend/bugs/e2e-acceptance-v1/solutions/SOL-FE-E2E-001-rpc-contract-and-dead-end.md))
**Tasks:** TASK-E2E-001 đến 005

## Vấn đề

Bốn tầng (service, gateway, agent, frontend) mỗi tầng được test với mock của tầng kế bên, nên khi một mắt xích thiếu (kênh gateway, method agent) thì không test nào đỏ. Bản vá từng chỗ (thêm `clarification.*`, thêm `fs.checkPath`) chỉ giải quyết hai trường hợp đã biết.

## Thiết kế

Tạo **ba kiểm tra tĩnh**, mỗi kiểm tra so sánh hai nguồn sự thật có sẵn, không thêm tầng chạy mới:

| Kiểm tra | So sánh | Chạy ở đâu | Đỏ khi |
|---|---|---|---|
| **P1** RPC ↔ kênh gateway | `requestv1.RequestService_ServiceDesc.Methods` (descriptor sinh từ proto) với kênh đã đăng ký trong `wscompat`, qua **bảng ánh xạ tường minh** `rpc → kênh` hoặc `rpc → exclusion(lý do)` | `api-gateway` unit test | thêm RPC công khai mà không thêm kênh hoặc dòng loại trừ |
| **P2** Kênh gateway ↔ hằng số frontend | file `testdata/channel_inventory.json` (xuất từ `Registry.Channels()`) với `REQUEST_RPC_METHODS` và các bảng tên kênh khác của frontend | gateway test ghi/so khớp file vàng; frontend vitest đọc file đó | frontend gọi kênh không tồn tại, hoặc kênh bị xoá khỏi gateway |
| **P3** Method backend gọi agent ↔ method agent hỗ trợ | các literal truyền vào `Relay`/`RelayByDevServer`/`RelayStream` trong Go với danh sách method của agent | script CI mở rộng `backend-go/ci/check-agent-capability-golden.sh` | backend gọi method agent không có |

**Vì sao bảng ánh xạ tường minh (P1) thay vì suy ra tên:** tên RPC (`ListClarifications`) và tên kênh (`clarification.list`) khác quy ước; suy ra tự động sẽ sinh false positive. Bảng ánh xạ là tài liệu sống: mỗi RPC công khai phải nằm ở đó, hoặc có lý do loại trừ (ví dụ RPC nội bộ nhóm `GroupInternal`).

**Hoàn tất hai bản vá đang chờ commit:**
- Kênh `clarification.*` (đã có mã và unit test): bổ sung vào CONTRACT-request-ui-api.md mục kênh, và giữ `clarification.*` trong `excluded_channels.yaml` (Clarification là việc của người, không phải agent).
- `fs.checkPath` ở agent (đã có mã, **chưa có test**): thêm test vitest, thêm vào danh sách capability/golden, và ghi vào `specs/agent/api/agent-rpc-catalog-git-fs.md`.

## Tương thích

Không đổi proto, không đổi hành vi runtime. Chỉ thêm test, tệp vàng và script. Agent cũ chưa có `fs.checkPath`: `project-service` nhận `Method not found` và trả lỗi rõ ràng (đã có `INFRA_AGENT_EXEC_FAILED`); TASK-E2E-005 thêm cảnh báo phiên bản tối thiểu của agent.

## Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| Bảng ánh xạ P1 trở thành nơi dồn việc thủ công | mỗi dòng bắt buộc có lý do khi là loại trừ; test báo RPC chưa được liệt kê kèm tên |
| Tệp vàng P2 gây xung đột merge | định dạng sắp xếp theo tên, một kênh một dòng |
| P3 sót method gọi qua biến chứ không phải literal | script cũng quét hằng số tên method; ghi giới hạn trong README của script |

## Tiêu chí hoàn thành

- Thêm một RPC công khai mới vào `RequestService` mà không cập nhật bảng ánh xạ làm test P1 đỏ (kiểm tra bằng cách thử thật).
- Xoá `clarification.answer` khỏi gateway làm P2 đỏ ở frontend.
- Xoá `fs.checkPath` khỏi agent làm P3 đỏ.
- e2e `RQL-14` đến `RQL-17`, `DEV-04`, `DEV-05` xanh.

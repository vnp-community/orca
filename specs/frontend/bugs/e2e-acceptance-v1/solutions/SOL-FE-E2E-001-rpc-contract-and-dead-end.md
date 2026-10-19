# SOL-FE-E2E-001: Khoá hợp đồng kênh RPC và không để người dùng rơi vào ngõ cụt im lặng

**Resolves:** FE-E2E-A, FE-E2E-C (góp phần chặn BUG-E2E-001 tái phát) · **Gốc:** RC-A
**Tasks:** TASK-FE-E2E-001, 002

## Vấn đề

Frontend gọi `clarification.list/get/answer/cancel` nhưng không có test nào kiểm kênh có tồn tại ở gateway. Khi kênh thiếu, `useClarifications` đặt lỗi `unsupported` thành "không có Clarification", nên người dùng thấy Request "đang chờ" mà không có câu hỏi. Đây là ngõ cụt im lặng.

## Thiết kế

### 1. Kiểm tra hợp đồng kênh (P2 phía frontend)

Backend xuất tệp vàng `channel_inventory.json` (../../../../backend-go/bugs/e2e-acceptance/v1/tasks/TASK-E2E-003-channel-inventory-golden-file.md). Thêm một test vitest đọc tệp đó và khẳng định: mọi giá trị trong `REQUEST_RPC_METHODS` (và các bảng tên kênh Request/Approval/Solution/Backlog/Clarification khác) có mặt trong danh mục. Báo tên kênh thiếu. Kênh chỉ dùng ở chế độ Electron cục bộ được liệt kê vào danh sách cho phép có lý do.

Không import code backend vào bundle: chỉ test đọc tệp JSON ở thời điểm test.

### 2. Không nuốt `unsupported` ở trạng thái đang chờ người

Quy tắc: nếu Request ở `awaiting_information` mà `clarification.list` thất bại (bất kể kiểu) hoặc trả rỗng, panel hiển thị **trạng thái chặn có tên**:
- tiêu đề "Không tải được câu hỏi làm rõ";
- nguyên nhân (`unsupported`: "máy chủ chưa hỗ trợ trả lời câu hỏi", `network`/`unavailable`: "thử lại");
- nút "Thử lại" và mã lỗi để báo vận hành;
- giữ nguyên hành vi hiện tại (không báo lỗi) ở các trạng thái **không** chờ người.

Lý do: "không có Clarification" chỉ hợp lệ khi Request không ở `awaiting_information`.

### 3. Kiểm thử

- Vitest cho hook: `awaiting_information` + `unsupported` → trạng thái chặn; `completed` + `unsupported` → không báo lỗi.
- Playwright `request-web` hiện dùng `mock-request-ws.ts`; thêm một kịch bản mô phỏng kênh thiếu.

## Tương thích

Không đổi giao thức. Chỉ đổi cách hiển thị khi lỗi.

## Tiêu chí hoàn thành

- Xoá `clarification.answer` khỏi tệp vàng thì test hợp đồng đỏ với tên kênh.
- Với kênh `clarification.list` giả lập `unsupported` ở trạng thái `awaiting_information`, panel hiển thị trạng thái chặn có nút thử lại.

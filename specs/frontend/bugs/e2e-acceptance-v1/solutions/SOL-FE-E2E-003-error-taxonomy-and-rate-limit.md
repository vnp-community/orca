# SOL-FE-E2E-003: Bảng mã lỗi đầy đủ, thử lại có kiểm soát, thông báo có hướng khắc phục

**Resolves:** FE-E2E-B, FE-E2E-E, thông báo AI (BUG-E2E-010), BUG-E2E-007/009 phía hiển thị · **Gốc:** RC-C, RC-E
**Tasks:** TASK-FE-E2E-004, 005, 006, 007, 008

## Vấn đề

`shared/request-errors.ts` có kiểu `rate_limited` nhưng không có mã nào ánh xạ tới nó; `REQUEST_RATE_LIMITED`, `REQUEST_FLOW_DISABLED`, `REQUEST_PROJECT_NOT_FOUND` và các mã AI mới sẽ rơi vào `unknown`. Người dùng thấy lỗi chung, không biết nên đợi, cấu hình hay báo quản trị.

## Thiết kế

### 1. Bổ sung `CODE_TO_KIND` và nội dung hiển thị theo mã

| Mã | Kiểu | Hiển thị / hành vi |
|---|---|---|
| `REQUEST_RATE_LIMITED` | `rate_limited` | thông báo ngắn "Thao tác quá nhanh"; **tự thử lại một lần** sau `retryAfterMs` nếu có (mặc định 1 giây) cho thao tác đọc/ghi idempotent; không thử lại thao tác không idempotent; vô hiệu hoá nút trong khoảng chờ |
| `REQUEST_FLOW_DISABLED` | `unavailable` (hoặc kiểu mới `feature_off`) | "Luồng Request chưa được bật cho tổ chức này"; admin thấy nút tới Settings |
| `REQUEST_PROJECT_NOT_FOUND` | `not_found` | "Project không còn tồn tại" |
| `REQUEST_AI_NOT_CONFIGURED` | `no_ai` (kiểu mới) | "Dev server chưa đăng nhập Claude Code" + hướng dẫn chạy `claude auth login` trên dev server (hoặc đặt khoá API); liên kết tới Settings → AI providers cho admin |
| `REQUEST_AI_CREDENTIAL_UNAVAILABLE` | `no_ai` | "Không đọc được khoá AI, kiểm tra credential" (chỉ khi dùng khoá phía server) |
| `REQUEST_AI_SESSION_EXPIRED` | `no_ai` | "Phiên đăng nhập Claude Code trên dev server đã hết hạn, đăng nhập lại" |
| `REQUEST_AI_QUOTA_EXHAUSTED` | `rate_limited` | "Hạn mức AI của dev server đã hết, thử lại sau" (không tự thử lại) |
| `REQUEST_AI_AGENT_UNAVAILABLE` | `no_dev_server` (đã có) | "Chưa có dev server kết nối" |
| `TASK_NOT_FOUND` khi xoá | coi là thành công | xoá lặp không báo lỗi |
| `PROJECT_DEV_SERVER_*` | `no_dev_server`/`unavailable` | hướng dẫn kết nối/ duyệt dev server |

Mọi chuỗi qua i18n (G3).

### 2. Thông báo cho giai đoạn AI

Khi phân loại không có đề xuất vì thiếu AI, banner trong chi tiết Request nêu mã và việc cần làm; không hiển thị "classifier error" thô.

### 3. Task: phân biệt rỗng và lỗi

Cây task phân biệt "không có task con" với "không tải được" (và với "không có quyền xem" sau sửa backend).

### 4. Kiểm thử

Mở rộng `request-errors.test.ts`; thêm test hiển thị; Playwright `request-web` kiểm banner AI.

## Tương thích

Mã lạ vẫn là `unknown` (hành vi hiện tại). Kiểu mới được thêm vào union và mọi `switch` phải xử lý (TypeScript bắt).

## Tiêu chí hoàn thành

- Mỗi mã ở bảng có test ánh xạ.
- `REQUEST_RATE_LIMITED` không hiển thị như `unknown`; nút bị khoá trong khoảng chờ.
- Banner AI hiển thị đúng khi backend trả `REQUEST_AI_NOT_CONFIGURED`.

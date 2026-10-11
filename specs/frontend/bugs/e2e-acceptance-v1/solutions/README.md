# Solutions frontend: bug nghiệm thu e2e (v1)

| Solution | Xử lý | Tasks | Ưu tiên |
|---|---|---|---|
| [SOL-FE-E2E-001](./SOL-FE-E2E-001-rpc-contract-and-dead-end.md) | hợp đồng kênh RPC; không nuốt lỗi khi Request chờ người | 001, 002 | P0 |
| [SOL-FE-E2E-002](./SOL-FE-E2E-002-typed-clarification-inputs.md) | ô nhập câu hỏi theo `valueType` | 003 | P2 |
| [SOL-FE-E2E-003](./SOL-FE-E2E-003-error-taxonomy-and-rate-limit.md) | bảng mã lỗi, thử lại, thông báo AI, task rỗng/lỗi | 004–008 | P1 |
| [SOL-FE-E2E-004](./SOL-FE-E2E-004-web-repo-host-binding.md) | web luôn gắn host (dev server) khi tạo project/repo; chọn, hiển thị, đổi host; Browse folder có giới hạn | 015–021 | P0/P1 |
| [SOL-FE-E2E-005](./SOL-FE-E2E-005-add-project-own-project-dedupe-answer-view.md) | "Add Project" tạo project riêng cho mỗi repo; chặn thêm trùng repo; Create request hiện rõ repo; ô Answer đọc `answer_markdown` | 022–026 | P0/P1 |

Backend tương ứng: [`../../../../backend-go/bugs/e2e-acceptance-v1/solutions`](../../../../backend-go/bugs/e2e-acceptance/v1/solutions/README.md). Phụ thuộc chéo:
- TASK-FE-E2E-001 ← TASK-E2E-003 (tệp vàng kênh)
- TASK-FE-E2E-002 ← TASK-E2E-001 (kênh `clarification.*`)
- TASK-FE-E2E-003 ← TASK-E2E-032 (`valueType`)
- TASK-FE-E2E-004, 005 ← TASK-E2E-015, 024, 031 (mã lỗi mới)
- TASK-FE-E2E-022..026 ↔ agent AE (`project.create` tạo repo từ `repoPath`, mã `PROJECT_REPO_ALREADY_REGISTERED`); frontend tự kiểm trùng nên không chặn chờ backend
- TASK-FE-E2E-016..021 ↔ TASK-V2-028..034 ([SOL-V2-002](../../../../backend-go/bugs/e2e-acceptance/v2/solutions/SOL-V2-002-repo-host-binding-and-no-silent-local.md)); FE-020 ← TASK-V2-030

## Ràng buộc kiến trúc

| # | Ràng buộc |
|---|---|
| G1 | Tuân thủ `docs/STYLEGUIDE.md`: dùng token trong `main.css` và primitive shadcn trong `components/ui/`; không thêm màu/cỡ chữ/bóng mới |
| G2 | Hai target render: Electron (`window.api`) và web (`web-preload-api`). Mọi thay đổi phải chạy ở cả hai, qua `callRequestRpc` và `request-rpc-client.ts`, không gọi trực tiếp WebSocket |
| G3 | Chuỗi hiển thị qua i18n (`auto.components.request.*`); không chuỗi cứng |
| G4 | Không đoán quy tắc nghiệp vụ ở UI: máy chủ là nguồn quyết định; UI chỉ gợi ý/kiểm tra sớm khi kiểu chắc chắn |
| G5 | Phím tắt và nhãn theo nền tảng; đường dẫn dùng tiện ích path (Mac/Linux/Windows) |
| G6 | Mọi thay đổi phải tính đến SSH/dev server từ xa (không giả định hệ tệp cục bộ) |
| G7 | Tên tệp theo nghiệp vụ, không `helpers/utils/common`; không thêm `max-lines` disable (AGENTS.md) |
| G8 | Không đưa bí mật vào log hoặc thông báo lỗi |

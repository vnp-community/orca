# Bug nghiệm thu e2e các luồng ORCA (v1)

Các bug được phát hiện khi viết và chạy bộ e2e nghiệm thu [`tests/stages/e2e/orca-flows`](../../../../../tests/stages/e2e/orca-flows/README.md) trên stack Docker local, dev server là máy của người dùng. Báo cáo gốc: [`tests/ORCA-E2E-ACCEPTANCE-REPORT.md`](../../../../../tests/ORCA-E2E-ACCEPTANCE-REPORT.md).

**Mục tiêu của đợt này:** thông luồng Request → Workflow → Task trên dev server thật và dùng ổn định, bằng cách sửa theo **nguyên nhân gốc** thay vì từng bug rời, không phá vỡ kiến trúc hiện có.

## Quyết định đã chốt (2026-10-09)

1. `ai.complete` dùng **phiên đăng nhập của dev server** thay cho khoá API (đường mặc định).
2. Thêm RPC nội bộ **`ResolveProjectAccess`** vào `project-service`.

## Đọc theo thứ tự

1. [IMPACT-ASSESSMENT.md](./IMPACT-ASSESSMENT.md): đánh giá ảnh hưởng của từng bug, gom theo nguyên nhân gốc, ảnh hưởng theo luồng.
2. [solutions/](./solutions/README.md): sáu solution, mỗi solution xử lý một nguyên nhân gốc và mọi bug của nó; ràng buộc kiến trúc G1–G10.
3. [tasks/](./tasks/README.md): 35 task có checklist, phụ thuộc và định nghĩa "xong".
4. Phía giao diện: [`specs/frontend/bugs/e2e-acceptance-v1`](../../../../frontend/bugs/e2e-acceptance-v1/README.md).

## Chỉ mục bug

| ID | Bug | Mức | Gốc | Solution | Trạng thái mã |
|---|---|---|---|---|---|
| BUG-E2E-001 | Gateway thiếu kênh `clarification.*` | Critical | RC-A | SOL-E2E-001 | sửa, chưa commit |
| BUG-E2E-002 | Thiếu token nội bộ và mount OPA cho `request-service` | Critical | RC-D | SOL-E2E-004 | sửa, chưa commit |
| BUG-E2E-003 | `project-service` gọi `infra-fleet` không gắn tenant | High | RC-B | SOL-E2E-002 | sửa, chưa commit, chưa test |
| BUG-E2E-004 | Agent thiếu `fs.checkPath` | High | RC-A | SOL-E2E-001 | sửa, chưa commit, chưa test |
| BUG-E2E-005 | `build-local.sh` làm mất mount | High | RC-D | SOL-E2E-004 | sửa, chưa commit |
| BUG-E2E-006 | `task.create` không truyền `CreatorId`; dữ liệu cũ chưa vá | Medium | RC-B | SOL-E2E-002 | sửa một phần |
| BUG-E2E-007 | Xoá task không tồn tại trả `Internal` | Low | RC-C | SOL-E2E-003 | sửa, có test, chưa commit |
| BUG-E2E-008 | Workflow nền mất danh tính người chạy | High | RC-B | SOL-E2E-002 | sửa một phần |
| BUG-E2E-009 | Request vào project không tồn tại | Medium | RC-C | SOL-E2E-003 | **mở** |
| BUG-E2E-010 | Không có đường cấp khoá AI cho `ai.complete` | Critical (bước AI) | RC-E | SOL-E2E-005 | **mở**; hướng đã chốt: phiên đăng nhập CLI của dev server |
| OBS-A, B, C, D | lỗi phân loại `project.get`; kiểu câu hỏi; giới hạn ghi; tài liệu/e2e | Low–Medium | RC-C, F | SOL-E2E-003, 006 | mở |

## Phạm vi hiệu lực

Đánh giá dựa trên stack local một người dùng, một tenant, Postgres. Chưa kiểm tra: phân quyền nhiều người dùng, nguồn Jira/GitHub/webhook/MCP, MySQL, dev server từ xa qua SSH, UI. Các giới hạn này lặp lại trong báo cáo e2e mục 8 và ảnh hưởng tới việc có thể xem đợt này là "đủ" hay chưa (xem định nghĩa "xong" ở `tasks/README.md`).

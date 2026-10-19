# TASK-FE-E2E-029: gỡ bộ Playwright mock `request-web`; bảng ánh xạ độ phủ mock → thật

**From Solution:** [SOL-FE-E2E-006](../solutions/SOL-FE-E2E-006-real-e2e-on-user-repo-replaces-mocked-request-web.md) · **Resolves:** "sửa đổi case 3 thành UI với tương tác thật trên stack docker local, không chạy mock bất kỳ thành phần nào"
**Priority:** P1 · **Area:** test
**Files:** xoá `tests/stages/e2e/legacy/request-web/` (15 spec + `support/mock-request-ws.ts`, `support/mock-project-host-gateway.ts`, 74 test); sửa `tests/configs/playwright.web.config.ts` (bỏ project `request-web`, bỏ `**/request-web/**` khỏi `testIgnore` của `mcp-web`); thêm `specs/frontend/bugs/e2e-acceptance-v1/COVERAGE-request-web-mock-to-real.md`
**Depends on:** TASK-FE-E2E-028
**Status:** `[x]` XONG (xem "Kết quả thực thi")

---

## Việc cần làm

- [x] Mỗi ca trong 74 ca mock được ánh xạ sang spec thật phụ trách, hoặc ghi rõ "không thể thật" kèm lý do (cần tiêm lỗi vào hạ tầng, hoặc cần điều kiện stack không có).
- [x] Gỡ project `request-web` khỏi cấu hình Playwright và xoá các spec/support mock (lịch sử còn trong git; `git revert` khôi phục). `code-intel-web` không đổi (stub riêng của nó nằm ngoài thư mục bị xoá).
- [x] Tài liệu cũ nhắc đường dẫn `request-web` giữ nguyên làm lịch sử; tài liệu này và bảng ánh xạ là nơi tra "ca đó giờ ở đâu".
- [ ] Hành vi chỉ kiểm được bằng tiêm lỗi (gateway cũ thiếu kênh, luồng bị đóng, host rớt giữa lúc duyệt, hai host) KHÔNG còn test e2e; cần test cấp component (vitest) nếu muốn giữ: ghi vào backlog, không khẳng định đã phủ.

## Tiêu chí hoàn thành

- `grep -r request-web tests/configs` rỗng; `playwright test --project=request-live` chạy; không còn spec e2e nào dựng `mockRequestApp` hay WebSocket giả.

## Kết quả thực thi

- Đã `git rm` 15 spec + 2 tệp support (74 test mock), bỏ project `request-web` và mục `**/request-web/**` khỏi `tests/configs/playwright.web.config.ts`.
- Bảng ánh xạ: 71 dòng (74 test mock; một số dòng gộp 2 đến 3 ca): **33 R** (có spec thật xanh), **5 R\*** (có điều kiện, phụ thuộc AI), **9 một phần**, **24 —** (không còn trong e2e: cần tiêm lỗi hoặc điều kiện stack không có hoặc chưa làm).
- Tồn đọng cố ý (24 ca **—**): giữ ở cấp component nếu muốn; hai ca làm thật được nhưng chưa làm (clone, create trong thư mục tạm) ghi ở backlog.
- Kiểm: `grep -rn request-web tests/configs` rỗng; `run_ui_flow.py --real deepseek` và `--real all` chạy qua project `request-live` (xanh, xem TASK-FE-E2E-028).

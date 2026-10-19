# FE-REQ-TASK-040-05: i18n 5 locale và e2e giao diện trên repo fixture tài chính

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json`, `tests/stages/e2e/orca-flows/ui-live/request-base-branch.web.e2e.ts` (mới) + `live-base-branch-steps.ts`, `run_ui_flow.py` (đăng ký spec)
**Depends on:** FE-REQ-TASK-040-03, 040-04, backend TASK-REQ-039-06
**Status:** [ ] TODO

---

## Context

- Lint chạy `verify:localization-catalog` và `verify:localization-coverage`.
- Spec UI dùng `runLiveFlow`, `configureProject`, fixture tạm; guard quiet-checkout khi chạy `--real`.
- Fixture `ledgerlite-finance` được sao vào repo git tạm bởi helper của backend task 06.

## Việc cần làm

1. Thêm mọi khoá mới vào 5 locale (en là nguồn; es/ja/ko/zh dịch có kiểm tra; ghi vào PR rằng cần người bản ngữ rà); chạy hai lệnh verify.
2. Spec `request-base-branch.web.e2e.ts` (`ORCA_E2E_LIVE=1`): (a) mở Create request, thấy chip "Default branch"; (b) chọn nhánh `release/1.x` của fixture, tạo, Overview hiện "Worktrees start from release/1.x"; (c) tạo không chọn: Overview "Default branch"; (d) sau khi chạy Plan có worktree (claude giả), dòng khoá với tooltip đúng; (e) nhập nhánh không tồn tại: lỗi dưới ô; (f) sửa nhánh ở Overview khi chưa khoá thành công; (g) cờ tính năng tắt (giả lập) thì ô ẩn.
3. Đăng ký trong `run_ui_flow.py` (tên chế độ `--ai-stub -- request-base-branch`).
4. Chụp ảnh các trạng thái chính vào thư mục ảnh của harness.

## Kiểm thử

- `pnpm run verify:localization-catalog && pnpm run verify:localization-coverage` xanh.
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- request-base-branch` xanh trên stack local.
- Chạy lại `qa-bug-task-native-flow` và `qa-regressions` (terminal cần agent thật) để chắc không hồi quy.

## Tiêu chí hoàn thành

- [ ] Đủ khoá ở 5 locale; hai lệnh verify xanh.
- [ ] Spec UI xanh trên repo fixture thật.
- [ ] Không hồi quy các spec QA hiện có.

## Rủi ro và lưu ý

- `--ai-stub` không spawn được PTY: spec này không dùng terminal nên chạy được với stub.
- Bản dịch es/ja/ko/zh cần rà bởi người bản ngữ.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

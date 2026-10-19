# FE-REQ-TASK-041-04: Activity, i18n 5 locale và e2e giao diện trên repo fixture tài chính

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `renderer/src/components/request/activity/activity-entry-presentation.ts` (+ test), `i18n/locales/{en,es,ja,ko,zh}.json`, `tests/stages/e2e/orca-flows/ui-live/analysis-revision.web.e2e.ts` (mới) + `live-analysis-revision-steps.ts`, `run_ui_flow.py` (đăng ký)
**Depends on:** FE-REQ-TASK-041-02, 041-03, backend TASK-REQ-041-04
**Status:** [ ] TODO

---

## Context

- Activity kind lạ phải chịu được; kind mới cần biểu tượng/nhãn.
- Spec UI dùng fixture `ledgerlite-finance` được sao vào repo git tạm.

## Việc cần làm

1. `analysis.revision_requested` trong `activity-entry-presentation.ts`; test ánh xạ và kind lạ.
2. Mọi khoá i18n mới cho 5 locale; chạy `verify:localization-catalog` và `verify:localization-coverage`.
3. Spec `analysis-revision.web.e2e.ts` (claude giả): (a) `change_request` → Analysis hiện Solution, nhập yêu cầu, "Revise analysis", thấy "Analyzing version 2…", rồi bản 2 với Approval mới; lặp lần hai; danh sách có 3 bản; (b) xem bản 1 chỉ đọc; (c) `bug` chẩn đoán; (d) `spike` Findings; (e) `question` Answer; (f) tài khoản approver không phải reporter chỉnh được, tài khoản chỉ đọc bị từ chối với thông báo quyền; (g) Activity có dòng "Requested changes to the analysis" và không chứa nội dung; (h) ca lỗi `NO_CONNECTION` giữ nội dung đã gõ.
4. Đăng ký `--ai-stub -- analysis-revision`; ảnh chụp trạng thái chính.

## Kiểm thử

- Hai lệnh verify i18n xanh.
- `run_ui_flow.py --ai-stub -- analysis-revision` xanh trên stack local.
- Không hồi quy `qa-bug-task-native-flow`, `qa-regressions`, tab Analysis/History hiện có.

## Tiêu chí hoàn thành

- [ ] Spec xanh cho bốn kiểu tài liệu và hai vai trò.
- [ ] Activity hiển thị đúng, không lộ nội dung.
- [ ] 5 locale đủ khoá.

## Rủi ro và lưu ý

- Hai tài khoản (approver, chỉ đọc) cần được tạo bằng API admin của stack local trong bước chuẩn bị spec, không dùng tài khoản thật; mật khẩu sinh ngẫu nhiên, không in ra.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

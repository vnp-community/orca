# FE-REQ-TASK-045-07: Activity `plan.revision_requested` và khoá i18n 5 locale cho xem tài liệu và chỉnh Plan

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `renderer/src/components/request/activity/{activity-entry-presentation,activity-entry-text}.ts` (+ test, sửa), `renderer/src/components/request/activity/activity-locale-coverage.test.ts` (cập nhật), `renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa), `renderer/src/components/request/plan/plan-locale-coverage.test.ts` (mở rộng) và test phủ i18n mới cho `plan/documents/` và `plan/revision/`
**Depends on:** FE-REQ-TASK-045-03, 045-04, 045-05, 045-06; backend BE-REQ-TASK-045-07
**Status:** [x] DONE (2026-10-20)

---

## Context

- Activity kind lạ phải chịu được; CR-REQ-041 thêm `analysis.revision_requested` ở `activity-entry-presentation.ts` (biểu tượng `FilePenLine`, nhóm lọc `changes`, mở tab Analysis, tông trung tính) và `activity-entry-text.ts` (tiêu đề cố định, **không bao giờ** có dòng chi tiết cho kind này kể cả khi backend gửi `feedback`/`reason`).
- Backend (BE-REQ-TASK-045-07) thêm `plan.revision_requested` ("Requested changes to the plan") và loại lượt Plan khỏi `analysis.revision_requested`.
- Quy ước khoá: `auto.components.request.<khu>.<tên>`; hai lệnh `verify:localization-catalog` và `verify:localization-coverage` (đỏ sẵn trên HEAD theo ghi chú TASK-041-04: ghi đầu ra trước và sau, đầu ra không được xấu đi).
- Test phủ mẫu: `activity-locale-coverage.test.ts`, `plan-locale-coverage.test.ts`, `analysis-revision-locale-coverage.test.ts`.

## Việc cần làm

1. `activity-entry-presentation.ts`: kind `plan.revision_requested`: biểu tượng (cùng `FilePenLine` hoặc biểu tượng khác nhưng phân biệt được với Analysis), nhóm lọc `changes`, mở tab Plan (`activityTargetTab`), tông trung tính. `activity-entry-text.ts`: tiêu đề "Requested changes to the plan"; không dòng chi tiết nào cho kind này.
2. Liệt kê mọi chuỗi mới của task 03, 04, 05, 06 (`grep -rn "auto.components.request.plan.documents\|auto.components.request.plan.revision" renderer/src`), thêm khoá vào **5 locale** trong một lần sửa nguyên tử mỗi file, giữ định dạng JSON; en là bản gốc; es/ja/ko/zh tự dịch và ghi chú "cần người bản ngữ rà" (như TASK-041-04).
3. Test phủ i18n: đọc mọi lời gọi `translate(...)` của hai thư mục mới, kiểm đủ 5 catalog, cùng placeholder (`{{n}}`, `{{size}}`, `{{branch}}`), bản en = fallback trong mã, các bản dịch không sao chép tiếng Anh nguyên văn (trừ tên riêng/ký hiệu).
4. Chạy `pnpm verify:localization-catalog` và `pnpm verify:localization-coverage` (hoặc tên script thực trong `package.json`); không thêm khoá không dùng.
5. `activity-entry-text.test.ts`: kiểm tra "không lộ nội dung" cho kind mới (đưa `feedback: 'secret text'` vào payload, khẳng định chuỗi không xuất hiện ở bất kỳ dòng nào).

## Kiểm thử

- `activity-entry-presentation.test.ts`, `activity-entry-text.test.ts`, `RequestActivityTab.test.tsx` (một dòng chỉnh Plan, có tác nhân, không huy hiệu mức độ), `activity-locale-coverage.test.ts`.
- Test phủ i18n của `plan/documents` và `plan/revision`.
- Hai lệnh verify i18n: đầu ra không xấu hơn mốc HEAD.
- `pnpm vitest run` vùng đã đổi, kiểu, lint.

## Tiêu chí hoàn thành

- [x] Activity hiển thị "Requested changes to the plan", không lộ nội dung phản hồi. Bằng chứng: `activity-entry-text.test.ts` (tiêu đề, không dòng chi tiết dù payload có `feedback`/`reason`/`comment`, không dòng nào chứa chữ bí mật), `RequestActivityTab.test.tsx` (một dòng, có tác nhân "User", không huy hiệu mức độ), `activity-entry-presentation.test.ts` (nhóm `changes`, biểu tượng `ClipboardPen` khác Analysis/`plan.done`, tab Plan, tông trung tính).
- [x] Mọi chuỗi mới có trong 5 locale cùng placeholder. Bằng chứng: `plan/documents/plan-documents-locale-coverage.test.ts`, `plan/revision/plan-revision-locale-coverage.test.ts` (bằng `describeTranslateCallLocales` + kiểm bản dịch không sao chép tiếng Anh, trừ "Irreversible", "Error", "{{hours}} h" của es), `plan-locale-coverage.test.ts` (khoá `${P}` của `PlanRunFailedCard`), `solution-locale-coverage.test.ts` (checkbox GFM), `activity-locale-coverage.test.ts`, `analysis-revision-ui-locale-coverage.test.ts` (thêm `RevisionPromptBox.tsx`).
- [x] Coverage i18n không xấu hơn mốc; test phủ mới xanh. Bằng chứng: `node config/scripts/verify-localization-catalog.mjs` (chạy trong `desktop/`): "Verified 196 localization key references", parity 10573 khoá ở es/ja/ko/zh. `audit-localization-coverage.mjs --check`: đỏ sẵn vì năm nhãn `store/slices/bootstrap.ts` (không phải của task này), không có dòng nào thuộc file đã đổi.

## Rủi ro và lưu ý

- Bản dịch es/ja/ko/zh tự dịch: cần người bản ngữ rà trước khi phát hành.
- Chạy `gitnexus_impact` cho `ACTIVITY_GROUP_KINDS` và `activityTargetTab` (cùng blast radius đã ghi ở TASK-041-04).

## Ghi chú triển khai (2026-10-20)

- **Sửa**: `activity/activity-entry-presentation.ts` (`plan.revision_requested` trong nhóm `changes`, biểu tượng `ClipboardPen`, `activityTargetTab` -> `plan`), `activity/activity-entry-text.ts` (tiêu đề cố định "Requested changes to the plan"; không dòng chi tiết nào), các test activity tương ứng, năm file `i18n/locales/{en,es,ja,ko,zh}.json` (94 khoá, thêm bằng một script, mỗi file ghi một lần bằng thay tên file; en = chữ trong mã; chỉ thêm khoá, không đổi khoá cũ), hai test phủ mới.
- **Khoá**: `auto.components.request.plan.documents.*` (Documents/Proposal/kiểm tra/tri thức), `auto.components.request.plan.revision.*` (panel, lỗi, chip, danh sách phiên bản), `auto.components.request.plan.PlanRunFailedCard.{hint.noChange,hint.baseRefGone,titleRevise,tryAgain,writeNew}`, `auto.components.request.RequestMarkdownContent.{taskDone,taskOpen}`, `auto.components.request.activity.title.planRevisionRequested`. Các chuỗi dùng lại của panel Analysis (`AnalysisRevision.counter`, `.suggestions`, `.shortcutHint`) không nhân đôi.
- **Bản dịch es/ja/ko/zh tự dịch, cần người bản ngữ rà trước khi phát hành** (giữ thuật ngữ đã có: kế hoạch/プラン/plan, "Superseded", "Version n").
- **Lệnh**: `vitest run` activity + plan + solution + i18n: xanh trừ lỗi đã biết ngoài phạm vi (`no-top-level-translate` do `FleetServerStatusBadge`).
- **Blast radius**: kind lạ vẫn chịu được (test cũ); `analysis.revision_requested` không đổi.
- **Chưa kiểm**: dòng History trên stack (task 08, ca (c)); phụ thuộc BE-045-07 phát kind `plan.revision_requested` và bỏ lượt Plan khỏi `analysis.revision_requested`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

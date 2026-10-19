# FE-REQ-TASK-041-03: Danh sách phiên bản: trạng thái, ghi chú, số bản; xem bản cũ ở chế độ chỉ đọc

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `renderer/src/components/request/solution/{SolutionVersionSwitcher.tsx,SolutionPanel.tsx}` (sửa), `solution-view-model.ts` (sửa nhỏ), test
**Depends on:** FE-REQ-TASK-041-01
**Status:** [x] DONE (2026-10-19)

---

## Context

- `SolutionVersionSwitcher` hiện chỉ đánh dấu "Older version" và ẩn khi < 2 bản.
- Xem bản cũ phải loại bỏ mọi hành động quyết định.

## Việc cần làm

1. Mỗi mục: "Version n" (từ `seq`, dự phòng thứ tự `generatedAt`), ghi chú rút gọn 80 ký tự từ `revisionFeedback` hoặc "First proposal", thời điểm tương đối, huy hiệu trạng thái (Current, Superseded, Rejected, Approved) — huy hiệu có chữ, không chỉ màu.
2. Danh sách có giới hạn chiều cao, phân trang khi > 20, không lồng cuộn gây khó dùng bàn phím.
3. `SolutionPanel`: khi `viewId` khác bản hiện hành, chế độ chỉ đọc (ẩn Approve, Choose, Reject, Revise, Regenerate), dải "You're viewing an older version. Back to current".
4. Khối thu gọn "What was asked for this version" hiển thị `revisionFeedback` đầy đủ (văn bản thuần, không HTML; xuống dòng giữ nguyên).
5. Giữ hành vi hiện có khi chỉ 1 bản (ẩn).

## Kiểm thử

- Component: huy hiệu, ghi chú, dự phòng không có `seq`, phân trang, chế độ chỉ đọc, quay lại bản hiện hành, văn bản phản hồi chứa `<script>` hiển thị như chữ.
- Test `SolutionVersionSwitcher` hiện có xanh.

## Tiêu chí hoàn thành

- [x] Bản cũ xem được nhưng không quyết định được.
- [x] Phản hồi hiển thị an toàn (không diễn giải HTML).
- [x] Hoạt động với số bản lớn.

## Rủi ro và lưu ý

- Số vòng không giới hạn: kiểm hiệu năng với 100 phiên bản giả.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

---

## Ghi chú triển khai (2026-10-19)

**File** (dưới `frontend/src/renderer/src/components/request/solution/`):
- `solution-version-history.ts` (mới, thay vì sửa `solution-view-model.ts`, đã 255 dòng): `orderSolutionsNewestFirst` (theo `seq` khi cả hai có, rồi `version`, rồi `generatedAt`; thay `orderNewestFirst` trong `SolutionPanel`), `currentSolutionOf`, `solutionVersionNumbers` (`seq`, dự phòng vị trí trong thứ tự), `versionNote` (gộp khoảng trắng, cắt 80 **ký tự** không tách cặp thay thế, thêm "…"; `null` cho bản đầu), `versionBadge`, phân trang (20 mỗi trang).
- `SolutionVersionSwitcher.tsx` (viết lại, prop mới `latestId` tuỳ chọn): danh sách `ul` cao tối đa `max-h-60` (một vùng cuộn duy nhất, phần tử là nút nên bàn phím tự cuộn tới), mỗi mục "Version n", huy hiệu có chữ + biểu tượng (Current, Superseded, Rejected, Approved), thời điểm tương đối (`<time>`, tooltip tuyệt đối), ghi chú 80 ký tự hoặc "First proposal". Phân trang "Newer"/"Older" + "a–b of N" khi > 20; trang theo bản đang xem (đổi bản từ ngoài, vd. Back to current, đưa trang tương ứng vào tầm nhìn) nhưng refetch thêm bản không kéo người đọc khỏi trang đang duyệt. Ẩn khi < 2 bản.
- `OlderVersionBanner.tsx` (mới): `role=status`, "You're viewing an older version." + nút "Back to current" (`viewId = null`).
- `SolutionRevisionFeedback.tsx` (mới): `Collapsible` "What was asked for this version", `<p className="whitespace-pre-wrap break-words">{feedback}</p>`: React escape nên `<script>` hiện như chữ, xuống dòng được giữ; không qua Markdown.
- `SolutionPanel.tsx`: nối switcher/banner/feedback; chọn đúng bản hiện hành trong danh sách đặt `viewId = null` (theo bản mới nhất), nên khi một lần chỉnh xong và bản mới xuất hiện, màn hình tự chuyển sang bản mới, còn người đang cố ý xem bản cũ thì ở lại bản cũ với dải cảnh báo. Ẩn Approve/Choose/Reject/Revise/Regenerate ở bản cũ: nhờ `isCurrent` quyết định thanh quyết định, panel chỉnh và `readOnly` (đã có từ trước, nay có test).
- i18n: `request.SolutionVersionSwitcher.{badgeCurrent,badgeSuperseded,badgeRejected,badgeApproved,versionN,firstProposal,pages,newer,range,older,askedFor,viewingOlder,backToCurrent}` × 5 locale; bỏ `.old`, `.version` (không còn dùng; đã grep).
- Test mới: `solution-version-history.test.ts`, `SolutionVersionSwitcher.test.tsx` (huy hiệu, ghi chú, dự phòng không `seq`, chọn, cắt 80 ký tự và `<script>` như chữ, 100 phiên bản: 20/trang, 5 trang, giữ/đổi trang), phần "older versions" của `SolutionPanel.revision.test.tsx` (chỉ đọc + dải + Back to current, chọn lại bản hiện hành, khối phản hồi là chữ thuần với `<script>`, bản đầu không có khối, bản bị từ chối ở dạng cũ).

**Khác với task/brief (có chủ đích)**
- Hai test có sẵn ở `SolutionPanel.test.tsx` tìm nút bản cũ bằng `name: /Older version/`; nhãn nay là huy hiệu "Superseded" theo task, nên đổi truy vấn thành `/Superseded/` (cùng hành vi được kiểm). Chữ "Older version" vẫn là banner trạng thái của `SolutionStatusBanner` khi mở bản cũ.
- Logic thuần đặt ở `solution-version-history.ts` thay vì `solution-view-model.ts` (tránh phình file, đặt tên theo khái niệm).
- "Hiệu năng 100 phiên bản": chỉ 20 mục render một lúc; test 100 bản chạy trong vài chục ms. Chưa đo trên UI thật.

**Chưa kiểm**: bố cục/cuộn thực tế và theme tối; trình đọc màn hình thật; e2e (041-04).

**Kiểm chứng đã chạy**
- `cd frontend && npx vitest run --config config/vitest.config.ts src/renderer/src/components/request src/shared/request-solution-wire-parser.test.ts`: 84 file, 933 test xanh (gồm toàn bộ `components/request/solution`: 23 file, 327 test, và test của agent khác trong `components/request`).
- Kiểm kiểu: `npx tsc --noEmit -p tsconfig.json --composite false --incremental false` còn 113 lỗi, đều có sẵn (không lỗi nào ở `components/request/solution`); trước thay đổi của task này cũng 113 (sau 041-01).
- Lint: `npx oxlint frontend/src/renderer/src/components/request/solution` sạch (lỗi `max-lines`/`curly` còn lại nằm ở `src/shared/execution-host-registry.ts`, `task-status-normalization.ts`, không thuộc task).
- i18n: `verify-localization-catalog.mjs` và `audit-localization-coverage.mjs --check` chạy trên bản sao `src/renderer/src` (script `verify-i18n.sh` ở scratchpad, thư mục riêng `lroot041`): không dòng nào thuộc file của task (các lỗi còn lại có sẵn trên HEAD: `DepartmentGate`, `web/`, `useAskBack.ts`). `analysis-revision-ui-locale-coverage.test.ts` kiểm mọi `translate(...)` trong 4 file UI mới có đủ 5 locale, en khớp fallback, cùng placeholder. Các khoá của agent khác (`gitRefPicker.*`, `BaseRef*`) không bị đụng: chỉ thêm khoá của task và bỏ 2 khoá không còn dùng (`SolutionVersionSwitcher.old`, `.version`).

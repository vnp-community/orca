# FE-REQ-TASK-045-06: Danh sách phiên bản Plan và xem bản cũ ở chế độ chỉ đọc

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `renderer/src/components/request/plan/revision/{PlanVersionSwitcher,plan-version-history}.ts(x)` (+ test, mới), `renderer/src/components/request/solution/OlderVersionBanner.tsx` (sửa: prop `testId`), `renderer/src/components/request/plan/documents/{PlanDocumentsView,PlanProposalView}.tsx` (sửa: nhận `revision`), `renderer/src/components/request/plan/RequestPlanTab.tsx` (sửa)
**Depends on:** FE-REQ-TASK-045-03, 045-04, 045-05
**Status:** [x] DONE (2026-10-20)

---

## Context

- Mẫu: `SolutionVersionSwitcher.tsx` (205 dòng; trang hoá `VERSION_PAGE_SIZE`, huy hiệu có chữ, ghi chú 80 ký tự), `OlderVersionBanner.tsx` (nhãn "You're viewing an older version.", nút "Back to current", `data-testid="solution-older-version-banner"`).
- `usePlanRevisions` (task 02): `revisions` mới nhất trước, mỗi bản có `revision`, `runId`, `status`, `revisionFeedback`, `generatedAt`, `filesChanged`, `fileCount`, `textKept`, `stats?`.
- Phạm vi bản cũ: chỉ **tài liệu/đề xuất** của bản đó; cây Tree/Graph và thanh duyệt luôn là bản hiện hành (task-service chỉ giữ Plan hiện hành làm cây thao tác).
- Request cũ chỉ có tài liệu cho bản hiện hành (bản trước CR không có dòng lịch sử): `textKept=false`, `fileCount=0`.

## Việc cần làm

1. `plan-version-history.ts`: `versionNote(rev)` (80 ký tự đầu của `revisionFeedback`, "First proposal" nếu rỗng và `revision === 1`, "Revised" nếu rỗng và > 1), `versionBadge(rev)` (`current|superseded`), `versionPageOf(...)` như `solution-version-history.ts` (dùng lại hằng nếu xuất được, không sao chép), tổng hợp "N files changed".
2. `PlanVersionSwitcher`: ẩn khi < 2 bản; danh sách cuộn trong khung có chiều cao giới hạn (không lồng cuộn trang), mục "Version n", ghi chú, thời điểm tương đối + tuyệt đối trong `title`, huy hiệu chữ, số file đổi (OpenSpec), chi phí/thời lượng nếu `stats`; "Show older" tải trang kế (`loadMore`); chọn một bản gọi `onSelect(revision)`; `aria-pressed` cho bản đang xem; điều hướng bàn phím.
3. Khối thu gọn "What was asked for this version" hiển thị toàn bộ `revisionFeedback` (văn bản thuần, giữ xuống dòng, `whitespace-pre-wrap`), chỉ khi có.
4. `OlderVersionBanner`: thêm prop tuỳ chọn `testId` (mặc định giữ `solution-older-version-banner`); Plan dùng `plan-older-version-banner` và `plan-back-to-current`.
5. `RequestPlanTab`: state `viewRevision` (null = hiện hành, theo bản mới nhất khi bản mới xuất hiện); `PlanDocumentsView` và `PlanProposalView` nhận `revision` để đọc bản đó (`usePlanBundleFile(..., revision)`, `usePlanProposalOfRun({runId})`); bản cũ: hiện banner, ẩn `PlanRevisionPanel`, `PlanChecksPanel` hiện "Checks are shown for the current version only"; Tree/Graph và thanh duyệt không đổi nhưng có dòng nhắc "The tree shows the current version" khi đang xem bản cũ ở Documents/Proposal; chuyển Request thì đặt lại `viewRevision`.
6. Bản bị lược chữ: thân tài liệu hiện "The text of this version is no longer kept" cùng danh sách file (từ chỉ mục), không gọi `planBundleFile`.
7. Bản hiện hành mới xuất hiện trong lúc đang xem bản cũ: giữ bản đang xem, hiện thông báo trung tính "A newer version is available" với nút "Show it".
8. Mọi chuỗi qua `translate('auto.components.request.plan.revision.…', …)`.

## Kiểm thử

- `plan-version-history`: ghi chú (rỗng/dài/đa byte), huy hiệu, trang.
- `PlanVersionSwitcher`: ẩn khi 1 bản; huy hiệu; "Show older"; chọn bản; bàn phím; bản bị lược chữ; Request cũ.
- `RequestPlanTab`: chọn bản cũ → banner + panel chỉnh ẩn + tài liệu bản đó; "Back to current"; bản mới xuất hiện; Tree không đổi.
- `OlderVersionBanner` của Analysis vẫn đúng `testId` mặc định (test hiện có xanh).
- `pnpm vitest run`, kiểu, lint.

## Tiêu chí hoàn thành

- [x] Danh sách phiên bản hiện mọi bản (kể cả đã superseded) kèm yêu cầu chỉnh đã sinh ra nó. Bằng chứng: `PlanVersionSwitcher.test.tsx`, `PlanVersionBar.test.tsx` ("shows the whole request…"), `RequestPlanTab.revision.test.tsx` ("shows what was asked…").
- [x] Xem bản cũ ở chế độ chỉ đọc: không panel chỉnh, không nút duyệt; về bản hiện hành bằng một cú nhấp. Bằng chứng: `RequestPlanTab.revision.test.tsx` ("reads an older version read only…": banner, không panel, tài liệu của bản 1 qua `revision`, cây và thanh duyệt của bản hiện hành, "Back to current"). Thanh duyệt vẫn hiện vì thuộc bản hiện hành theo U9 của solution, không phải của bản đang đọc.
- [x] Bản bị lược chữ và Request cũ hiển thị đúng thông điệp, không hứa nội dung không có. Bằng chứng: `PlanDocumentsView.test.tsx` (không gọi `planBundleFile`, "The text of this version is no longer kept.", Request cũ không có danh sách file); `PlanVersionSwitcher.test.tsx`.
- [x] Danh sách dài (hơn 20) dùng được bằng bàn phím, không lồng cuộn. Bằng chứng: `PlanVersionSwitcher.test.tsx` (100 phiên bản trong một `ul` `max-h-60 overflow-y-auto`, mỗi mục là `button` có `aria-pressed`, "Show older" tải trang kế bằng `loadMore`).

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `OlderVersionBanner` (dùng ở `SolutionPanel`).
- Số bản của Request cũ có thể bắt đầu muộn hơn 1 (các lượt trước CR không có dòng lịch sử) nhưng vẫn liên tục.

## Ghi chú triển khai (2026-10-20)

- **File mới** (`plan/revision/`): `plan-version-history.ts`, `PlanVersionSwitcher.tsx`, `PlanVersionBar.tsx`, `usePlanVersions.ts` (+ test). **Sửa**: `solution/OlderVersionBanner.tsx` (props `testId`, `backTestId`, mặc định là id của Analysis; Plan dùng `plan-older-version-banner` / `plan-back-to-current`), `plan/RequestPlanTab.tsx`.
- **Lệnh**: `vitest run plan/revision RequestPlanTab* plan-approval-bars* …`: 17 file, 216 test xanh (gồm task 05). `tsc` mốc 113; `oxlint` sạch.
- **Thiết kế**: `usePlanVersions` giữ lựa chọn theo Request (`null` = hiện hành, theo bản mới nhất); chọn bản cũ đưa tab sang Documents/Proposal (nơi duy nhất đọc được bản cũ); bản hiện hành mới xuất hiện khi đang đọc bản cũ thì giữ bản đang đọc và hiện "A newer version is available" + "Show it". `PlanVersionBar` nằm dưới `PlanRevisionSection`, ở mọi chế độ: banner bản cũ, dòng "The tree shows the current version." (chỉ ở Documents/Proposal), danh sách trong `Collapsible` "Versions (n)", khối "What was asked for this version" (văn bản thuần, `whitespace-pre-wrap`).
- **Sai lệch**: (1) không có `versionPageOf`/phân trang phía client như `solution-version-history`: phân trang do server (`loadMore`, "Show older") và danh sách cuộn trong khung giới hạn; chỉ dùng lại `versionNote` (80 ký tự, theo ký tự Unicode). (2) Với gateway không có `request.planRevisions` không có danh sách phiên bản (đúng spec: `supported=false`).
- **Blast radius**: `OlderVersionBanner` chỉ `SolutionPanel` (mặc định giữ nguyên id, test Analysis xanh) và `PlanVersionBar`.
- **Chưa kiểm**: trên stack, ảnh `older-version-1` (task 08).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

# Phase 3 — Quyết định, gửi lại, nối review code, e2e UI (FE-OR-SOL-003)

**Phụ thuộc:** Phase 1, 2; BE Phase 4 (quyết định/duyệt)

## Quyết định và duyệt

- [x] **FE-OR-T301** `use-outcome-decisions.ts` + `OutcomeDecisionCard` (Accept as exception với ghi chú bắt buộc và lỗi cạnh ô; Send back; Change decision); mục `advisory` chỉ có nhãn.
  Files: `outcome-review/decision/`. Verify: RTL; ghi chú trống bị chặn ngay cạnh ô.
  Kết quả (2026-10-21): `use-outcome-decisions.ts` + `OutcomeDecisionCard`: ghi chú trống bị chặn ngay cạnh ô, mục `advisory` chỉ có nhãn. Sửa một lỗi bắt được khi viết test: tham chiếu phiên bản cũ trong hook. Test card + hook (8).
- [x] **FE-OR-T302** `OutcomeDecisionBar`: ghi chú, Approve (nút chính duy nhất), "Request changes (n)"; lỗi cạnh nút khi còn mục mở + cuộn tới mục đầu (không vô hiệu hoá nút); không hiện ở pha `design`.
  Verify: RTL; gọi `approve` khi còn mục mở nhận `REQUEST_REVIEW_DECISIONS_OPEN` và hiển thị đúng.
  Kết quả (2026-10-21): `OutcomeDecisionBar`: Approve là nút chính duy nhất, "Request changes (n)", lỗi cạnh nút khi còn mục mở (`REQUEST_REVIEW_DECISIONS_OPEN`) và cuộn tới mục đầu, không vô hiệu hoá nút; không hiện ở pha `design`.
- [x] **FE-OR-T303** Chữ sau duyệt: "Review approved by <name>. This is not a release sign-off."; test từ cấm.
  Kết quả (2026-10-21): "Review approved by <name>. This is not a release sign-off." Chưa có thư mục người dùng nên tên hiện là "you" hoặc "a teammate".

## Gửi lại bằng Request con

- [x] **FE-OR-T310** `outcome-send-back-draft.ts` (hàm thuần): dựng mô tả từ mục `send_back` (mục, bằng chứng, id KG + snapshot), qua bộ che, giới hạn độ dài.
  Verify: unit test (canary bí mật bị che).
  Kết quả (2026-10-21): `outcome-send-back-draft.ts` hàm thuần: mục, bằng chứng, id KG + snapshot, qua bộ che, giới hạn độ dài; test canary bí mật bị che.
- [x] **FE-OR-T311** `OutcomeSendBackDialog` → `SpawnChildRequestDialog` điền sẵn (tiêu đề, loại gợi ý, liên kết Request gốc); lưu `childRequestId`.
  Files: `components/request/SpawnChildRequestDialog.tsx` (nhận dữ liệu điền sẵn; `gitnexus_impact` trước khi sửa). Verify: Request gốc không đổi trạng thái; nếu backend chặn con từ `completed`, hiển thị lỗi đúng và dùng phương án thay (BE-OR-T423).
  Kết quả (2026-10-21): `OutcomeSendBackDialog` → `SpawnChildRequestDialog` (thêm `prefill`, `onCreated`; lý do liên kết `escalation`, backend `ValidateChild` cho phép với mọi Request không `cancelled`, kể cả `completed`) nên không cần phương án thay của BE-OR-T423; Request gốc không đổi trạng thái; lưu `childRequestId`.
- [x] **FE-OR-T312** Dải "A follow-up Request finished. Regenerate this review to check." (không tự sinh lại).
  Kết quả (2026-10-21): Dải follow-up xong (`OutcomeFollowUpBanner`, `useChildRequestStatuses`); không tự sinh lại.

## Nối review code v7

- [x] **FE-OR-T320** `outcome-code-review-link.ts` (hàm thuần): ánh xạ section → lens (`architecture`, `dataflow`, `erd`, `contract`, `requirements`, `impact`); huy hiệu container → `architecture`.
  Verify: bảng ca; id `requirements` xác minh với `review-lens-registry.ts`.
  Kết quả (2026-10-21): `outcome-code-review-link.ts` bảng ca; `requirements` **không phải lens đã đăng ký** nên các section không có lens (Requirements, Behavior, Config) không có nút thay vì nút dẫn tới nơi không liên quan (nút mờ kèm giải thích "no view in the code review"); architecture→`architecture`, flow→`dataflow`, data→`erd`, contracts→`contract`; hàng nêu tệp đã đổi → `impact`; huy hiệu container → `architecture`.
- [x] **FE-OR-T321** Thêm `'request-outcome'` vào `ReviewEntrySource` và ánh xạ telemetry (hoặc dùng nguồn sẵn có làm phương án lùi nếu enum telemetry khó đồng bộ).
  Files: `components/review-map/entry/open-review-entry.ts`, enum telemetry. Verify: test `open-review-entry.test.ts` xanh; `gitnexus_impact` cho `openReviewFromEntryPoint`.
  Kết quả (2026-10-21): Thêm nguồn `'request-outcome'` vào `ReviewEntrySource` (test `open-review-entry.test.ts` xanh). **Dùng phương án lùi:** enum telemetry chưa có giá trị riêng nên đếm như `right_sidebar`. Không có công cụ GitNexus để chạy `impact`.
- [x] **FE-OR-T322** Nguồn `worktreeId` từ `subject.heads[].taskId`; nhiều worktree → chọn; thiếu hoặc v7 tắt → nút vô hiệu **kèm giải thích**.
  Verify: RTL ba trường hợp.
  Kết quả (2026-10-21): `useReviewWorktrees` lấy `worktreeId` từ `subject.heads[].taskId`; nhiều worktree → chọn; thiếu worktree hoặc review code tắt → nút mờ kèm giải thích. Test RTL ba trường hợp.

## e2e và hoàn thiện

- [~] **FE-OR-T330** `ui-live/outcome-review.web.e2e.ts` + `live-outcome-review-steps.ts` (agent giả): tab sau Execution, pha `design` không có Approve, Execution xong → `delivered`, hình trong iframe, đổi theme, sinh lại, quyết định, Request changes.
  Verify: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- outcome-review` (hoặc lệnh tương đương sau khi đăng ký); tuân thủ quiet-checkout.
  Kết quả (2026-10-21): Đã chạy trên stack thật (2026-10-21) sau khi redeploy: `run_ui_flow.py --ai-stub -- outcome-review` PASS (2,6 phút): (a) cờ tắt không có tab và không có lưu lượng `request.review.*`, (b) tab ngay sau Execution, mở chỉ đọc, review chỉ-sự-kiện giữ nguyên sau tải lại, (c) nút agent hoặc nói lý do bằng một câu ("A skill the review needs is not installed on the dev server") và không lộ mã. Lỗi thật bắt được: Regenerate ở header gửi section rỗng (đã sửa). **Vẫn chưa phủ bằng agent giả:** pha sau code, quyết định, Request changes (agent giả không đưa Request tới `completed`); phần đó được phủ bằng lượt thật ở T332.
- [x] **FE-OR-T331** Locale 5 ngôn ngữ đầy đủ cho mọi khoá mới; kiểm truy cập (bàn phím, `aria-live`, nhãn bảng); hai render target.
  Verify: test bao phủ locale đạt.
  Kết quả (2026-10-21): 288 khoá × 5 locale, test bao phủ locale PASS; `aria-live`, nhãn bảng, bàn phím kiểm bằng RTL. **Chưa có:** kiểm bằng trình đọc màn hình thật; Electron.
- [x] **FE-OR-T332** Lượt thật (tốn tiền): chỉ khi người dùng đồng ý chi phí; ghi lại ảnh chụp và số liệu.
  Kết quả (2026-10-21): Lượt thật trên stack local, Request #2309 từ backlog BL-E07 của `vnp-qa-platform`, nhánh gốc `claude/tms-bug-triage-report-09f310` (tip `fd4f6ceb4e`), engine openspec, 9/9 task trong 4 Phase, 11p13s, mọi worktree là hậu duệ của tip. **Pha thiết kế (Plan):** agent thật chạy 86 s, 1,365 USD, 12 mục, KG `snap-000004`, không có nút Approve/quyết định, chế độ sau-code khoá; hình `architecture/behavior` ở trạng thái `missing` (skill `diagram-design` chưa vendor nên không vẽ) và dải "Design not reviewed" hiện đúng. **Pha sau code:** lượt đọc code 0,7584 USD, 4 mục (3 cần quyết định: AC-1/2/3 của BL-E07, "không có task nào gắn với tiêu chí"); thẻ Requirements ghi "0 of 3, 3 not covered" dù 9/9 task xong vì plan OpenSpec không gắn task với tiêu chí (đúng theo dữ liệu, nhưng dễ gây hiểu nhầm); Approve bị từ chối khi còn mục mở, 3 mục được "Accept as exception" kèm ghi chú, rồi Approve: "Review approved by you. This is not a release sign-off." còn nguyên sau khi tải lại. **Lỗi tìm thấy:** (1) lượt Generate đầu sau Execution chờ 32 s đọc diff của worktree rồi trả `Internal` (retry 0,1 s); đã sửa: mỗi lần đọc diff có hạn 20 s và chỉ thành `partial_facts:diff` (test `TestCollectDiff_SlowWorktreeIsAPartialFactNotAFailure`); chưa tái hiện được nguyên nhân chậm lần đầu; (2) nút Regenerate ở header gửi danh sách section rỗng, đã sửa. **Chi phí đo được:** Analysis+Plan 2,30 USD mỗi lượt (chạy hai lượt vì lượt đầu hỏng do lỗi test của tôi và bị huỷ ở bước Plan), review 1,365 + 0,7584 USD; chi phí thực thi 9 task chưa đo. **Chưa phủ:** Send back (tạo Request con), hình vẽ thật (cần `diagram-design`), Electron.

## Hoàn tất

- [~] **FE-OR-T399** vitest + typecheck/lint + `detect_changes({scope:"compare", base_ref:"main"})` (hoặc ghi rõ nếu công cụ không có); chỉ commit khi người dùng yêu cầu.
  Kết quả (2026-10-21): `vitest` các thư mục `components/request`, `shared`, `i18n`, hai hook review: 1858/1859 PASS; test hỏng duy nhất `i18n/no-top-level-translate.test.ts` do `settings/ssh/FleetServerStatusBadge.tsx` (tệp không thuộc phần này, đã commit từ trước). Chạy rộng hơn (thêm `hooks`, `review-map`, `runtime`) còn 9 tệp / 49 test hỏng ở các tệp không sửa (`localStorage.clear is not a function`, `window.api.runtime.call is not a function`) — chưa so với HEAD sạch nên chưa khẳng định có từ trước. `tsc` 115 (đường cơ sở 115); `oxlint` sạch 92 tệp; Playwright 30/30. **Không có:** `detect_changes` của GitNexus; các lệnh lint của repo (`verify-*`, `check-reliability-gates`, `lint:switch-exhaustiveness`) không chạy được trong checkout này.

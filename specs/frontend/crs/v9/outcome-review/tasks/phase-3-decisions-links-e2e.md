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
  Kết quả (2026-10-21): Đã viết và typecheck: `ui-live/outcome-review.web.e2e.ts` + `live-outcome-review-steps.ts`. **Chưa chạy** vì stack thật đang dùng bản cũ; agent giả không đưa Request tới `completed` nên spec chỉ phủ pha thiết kế (tab sau Execution, không Approve, hình trong iframe); phần sau code/quyết định/Request changes chưa được phủ end-to-end (chỉ có test đơn vị và RTL).
- [x] **FE-OR-T331** Locale 5 ngôn ngữ đầy đủ cho mọi khoá mới; kiểm truy cập (bàn phím, `aria-live`, nhãn bảng); hai render target.
  Verify: test bao phủ locale đạt.
  Kết quả (2026-10-21): 288 khoá × 5 locale, test bao phủ locale PASS; `aria-live`, nhãn bảng, bàn phím kiểm bằng RTL. **Chưa có:** kiểm bằng trình đọc màn hình thật; Electron.
- [ ] **FE-OR-T332** Lượt thật (tốn tiền): chỉ khi người dùng đồng ý chi phí; ghi lại ảnh chụp và số liệu.
  Kết quả (2026-10-21): **Chưa làm — chờ bạn đồng ý chi phí.** Chưa có lượt chạy agent thật nào, nên chưa có ảnh chụp hay số liệu đo.

## Hoàn tất

- [~] **FE-OR-T399** vitest + typecheck/lint + `detect_changes({scope:"compare", base_ref:"main"})` (hoặc ghi rõ nếu công cụ không có); chỉ commit khi người dùng yêu cầu.
  Kết quả (2026-10-21): `vitest` các thư mục `components/request`, `shared`, `i18n`, hai hook review: 1858/1859 PASS; test hỏng duy nhất `i18n/no-top-level-translate.test.ts` do `settings/ssh/FleetServerStatusBadge.tsx` (tệp không thuộc phần này, đã commit từ trước). Chạy rộng hơn (thêm `hooks`, `review-map`, `runtime`) còn 9 tệp / 49 test hỏng ở các tệp không sửa (`localStorage.clear is not a function`, `window.api.runtime.call is not a function`) — chưa so với HEAD sạch nên chưa khẳng định có từ trước. `tsc` 115 (đường cơ sở 115); `oxlint` sạch 92 tệp; Playwright 30/30. **Không có:** `detect_changes` của GitNexus; các lệnh lint của repo (`verify-*`, `check-reliability-gates`, `lint:switch-exhaustiveness`) không chạy được trong checkout này.

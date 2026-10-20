# Phase 3 — Quyết định, gửi lại, nối review code, e2e UI (FE-OR-SOL-003)

**Phụ thuộc:** Phase 1, 2; BE Phase 4 (quyết định/duyệt)

## Quyết định và duyệt

- [ ] **FE-OR-T301** `use-outcome-decisions.ts` + `OutcomeDecisionCard` (Accept as exception với ghi chú bắt buộc và lỗi cạnh ô; Send back; Change decision); mục `advisory` chỉ có nhãn.
  Files: `outcome-review/decision/`. Verify: RTL; ghi chú trống bị chặn ngay cạnh ô.
- [ ] **FE-OR-T302** `OutcomeDecisionBar`: ghi chú, Approve (nút chính duy nhất), "Request changes (n)"; lỗi cạnh nút khi còn mục mở + cuộn tới mục đầu (không vô hiệu hoá nút); không hiện ở pha `design`.
  Verify: RTL; gọi `approve` khi còn mục mở nhận `REQUEST_REVIEW_DECISIONS_OPEN` và hiển thị đúng.
- [ ] **FE-OR-T303** Chữ sau duyệt: "Review approved by <name>. This is not a release sign-off."; test từ cấm.

## Gửi lại bằng Request con

- [ ] **FE-OR-T310** `outcome-send-back-draft.ts` (hàm thuần): dựng mô tả từ mục `send_back` (mục, bằng chứng, id KG + snapshot), qua bộ che, giới hạn độ dài.
  Verify: unit test (canary bí mật bị che).
- [ ] **FE-OR-T311** `OutcomeSendBackDialog` → `SpawnChildRequestDialog` điền sẵn (tiêu đề, loại gợi ý, liên kết Request gốc); lưu `childRequestId`.
  Files: `components/request/SpawnChildRequestDialog.tsx` (nhận dữ liệu điền sẵn; `gitnexus_impact` trước khi sửa). Verify: Request gốc không đổi trạng thái; nếu backend chặn con từ `completed`, hiển thị lỗi đúng và dùng phương án thay (BE-OR-T423).
- [ ] **FE-OR-T312** Dải "A follow-up Request finished. Regenerate this review to check." (không tự sinh lại).

## Nối review code v7

- [ ] **FE-OR-T320** `outcome-code-review-link.ts` (hàm thuần): ánh xạ section → lens (`architecture`, `dataflow`, `erd`, `contract`, `requirements`, `impact`); huy hiệu container → `architecture`.
  Verify: bảng ca; id `requirements` xác minh với `review-lens-registry.ts`.
- [ ] **FE-OR-T321** Thêm `'request-outcome'` vào `ReviewEntrySource` và ánh xạ telemetry (hoặc dùng nguồn sẵn có làm phương án lùi nếu enum telemetry khó đồng bộ).
  Files: `components/review-map/entry/open-review-entry.ts`, enum telemetry. Verify: test `open-review-entry.test.ts` xanh; `gitnexus_impact` cho `openReviewFromEntryPoint`.
- [ ] **FE-OR-T322** Nguồn `worktreeId` từ `subject.heads[].taskId`; nhiều worktree → chọn; thiếu hoặc v7 tắt → nút vô hiệu **kèm giải thích**.
  Verify: RTL ba trường hợp.

## e2e và hoàn thiện

- [ ] **FE-OR-T330** `ui-live/outcome-review.web.e2e.ts` + `live-outcome-review-steps.ts` (agent giả): tab sau Execution, pha `design` không có Approve, Execution xong → `delivered`, hình trong iframe, đổi theme, sinh lại, quyết định, Request changes.
  Verify: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- outcome-review` (hoặc lệnh tương đương sau khi đăng ký); tuân thủ quiet-checkout.
- [ ] **FE-OR-T331** Locale 5 ngôn ngữ đầy đủ cho mọi khoá mới; kiểm truy cập (bàn phím, `aria-live`, nhãn bảng); hai render target.
  Verify: test bao phủ locale đạt.
- [ ] **FE-OR-T332** Lượt thật (tốn tiền): chỉ khi người dùng đồng ý chi phí; ghi lại ảnh chụp và số liệu.

## Hoàn tất

- [ ] **FE-OR-T399** vitest + typecheck/lint + `detect_changes({scope:"compare", base_ref:"main"})` (hoặc ghi rõ nếu công cụ không có); chỉ commit khi người dùng yêu cầu.

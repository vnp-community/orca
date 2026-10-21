# Phase 4 — Cơ sở thiết kế, thanh phủ flow, thẻ ghép, trạng thái suy giảm và e2e thật (FE-OR-SOL-004)

**Phụ thuộc:** BE Phase 6 (`designBasis`, `flowRef`, `adopt`) và BE Phase 7 (`flowCoverage`, `errorDetail`, `estimate` theo `flowRef`). Task FE-OR-T401–T403 có thể làm trước bằng fixture khi BE chưa sẵn. · **Trạng thái:** 📝 chưa bắt đầu (2026-10-21)

Quy ước: file ≤ 300 dòng (không thêm `max-lines` disable); tên file theo khái niệm (không `helpers`/`utils`); test đặt cạnh file; chuỗi mới qua `translate(...)` và đủ `en, es, ja, ko, zh` theo thứ tự khoá; màu chỉ dùng token trong `main.css`; chạy test bằng `cd frontend && npx vitest run <đường dẫn>`.

## Kiểu và fixture

- [ ] **FE-OR-T401** Thêm kiểu tuỳ chọn `designBasis`, `flowCoverage`, `DiagramRef.flowRef`, `OutcomeReviewRun.errorDetail/diagramCount` vào `shared/outcome-review-types.ts`; cập nhật `outcome-review.fixture.ts` với ba kịch bản: (a) 2 flow ứng viên, đã vẽ 1; (b) thiết kế bị giữ do `design_shrank`; (c) `code_compare_failed` kèm `errorDetail`.
  Verify: review cũ không có trường mới vẫn render (test tương thích); `pnpm typecheck:web` không thêm lỗi ở file đã chạm.

## Header và cơ sở thiết kế (CR-OR-040 R7)

- [ ] **FE-OR-T402** Chip cơ sở thiết kế ("17 container · 40 cạnh · 3 flow", tooltip mermaid/text và thời điểm trích) trong `OutcomeDesignControl`; dải cảnh báo `design_shrank` kèm nút "Thay bằng bản mới" (xác nhận nêu số trước/sau; ẩn với người không có quyền); dải cảnh báo khi khác bản thiết kế đã duyệt ở pha thiết kế.
  Files: `outcome-review/OutcomeDesignControl.tsx`, `outcome-review/DesignBasisBanner.tsx` (mới), `outcome-review-copy.ts`. Verify: vitest ba kịch bản; nút gọi `request.review.designBasis.adopt` đúng tham số; không có quyền ⇒ không có nút.
- [ ] **FE-OR-T403** Hook `useDesignBasisAdopt` (RPC, làm mới review sau khi thành công, toast lỗi qua `outcome-review-state-copy`).
  Files: `hooks/useDesignBasisAdopt.ts`. Verify: vitest hook với RPC giả: thành công/lỗi/quyền.

## Tab Flow (CR-OR-041 R9–R11)

- [ ] **FE-OR-T404** `FlowCoverageBar`: thanh "Đã vẽ X/Y flow" (`role="progressbar"`, nhãn chữ) và danh sách flow chưa vẽ với lý do ứng viên, trạng thái và lý do của agent.
  Files: `outcome-review/diff/FlowCoverageBar.tsx` (mới), `outcome-diff-copy.ts`. Verify: vitest 0/N, X/N, N/N; mỗi dòng chưa vẽ có lý do; không có `flowCoverage` ⇒ không hiện thanh.
- [ ] **FE-OR-T405** Nút "Vẽ flow này": hiện ước tính chi phí (`request.review.estimate` theo `flowRef`), xác nhận khi vượt hạn mức, gọi sinh lại hình theo `section` + `flowRef`; ẩn với người không có quyền sinh lại.
  Files: `outcome-review/diff/FlowCoverageBar.tsx`, `outcome-review/artifact/RegeneratePanel.tsx` (tái dùng). Verify: vitest quyền/chi phí/xác nhận; sau khi vẽ, review làm mới và dòng chuyển sang "đã vẽ".
- [ ] **FE-OR-T406** `FlowPairCard`: mỗi `flowRef` một thẻ (tên, mục SAD, nhãn "mới, chưa có trong thiết kế" khi `hasDesign=false`), hai khung Design | After code (≥ 900 px cạnh nhau, dưới đó xếp dọc), thu gọn được, mặc định mở thẻ có delta; thay `DiagramColumn` cho section Flow.
  Files: `outcome-review/diff/FlowPairCard.tsx` (mới), `outcome-review/diff/OutcomeSystemDiff.tsx`. Verify: vitest ghép đúng khi tên flow khác ("Flow 8.1" ↔ "Bug resolved…"); `aria-expanded`; chế độ Design/After code lọc nhưng giữ thẻ; khi không có `flowRef` quay về hai cột như hiện nay.

## Tab Architecture (CR-OR-040 R6)

- [ ] **FE-OR-T407** Nhãn "Lát cắt của Request: N/M container" và liên kết "Hiện toàn bộ" (danh sách container đầy đủ qua `OutcomeContainerDrilldown`); nút "+N" của hình dẫn tới danh sách đó.
  Files: `outcome-review/diff/OutcomeSystemDiff.tsx`, `outcome-review/diff/OutcomeContainerDrilldown.tsx`. Verify: vitest với 17 container/lát cắt 6; "+N" mở đủ danh sách.

## Số liệu nhất quán (CR-OR-041 R10, README E12)

- [ ] **FE-OR-T408** Huy hiệu "not in design" lấy từ **số hàng** có cờ `notInDesign` của section (cùng hàm với danh sách), không từ `verdict.*.unplanned*`.
  Files: `outcome-review/diff/outcome-diff-view-model.ts` (`subTabsOf`), `outcome-diff-view-model.test.ts`. Verify: bảng ca cho cả 7 section: huy hiệu = số hàng mang nhãn; ca của #2318 (verdict 1/4, số hàng khác) không còn lệch.

## Trạng thái suy giảm và khung chạy (CR-OR-042 R4–R6)

- [ ] **FE-OR-T409** Khi có cảnh báo `code_compare_failed`: chế độ After code của Architecture/Flow hiện dải "Lượt đọc code lỗi nên chưa so với code" kèm tối đa 3 dòng `errorDetail` và nút "Thử lại" (sinh lại phạm vi `code`); Design vẫn hiện hình thiết kế; nút Approve `disabled` với lý do `contextFailed`.
  Files: `outcome-review/OutcomeReviewStates.tsx`, `outcome-review/diff/OutcomeSystemDiff.tsx`, `outcome-review/decision/*`. Verify: vitest; không `code_compare_failed` ⇒ không dải; "Thử lại" gọi đúng phạm vi.
- [ ] **FE-OR-T410** `OutcomeAgentRunFacts`: thêm số hình đã vẽ, chi phí mỗi hình, dòng "lỗi: …" (3 dòng, "xem thêm"), số cảnh báo `coerced:*`/`dropped:*`.
  Files: `outcome-review/OutcomeAgentRunFacts.tsx`, `outcome-review-run-facts.ts`. Verify: vitest; `errorDetail` rỗng ⇒ không dòng lỗi; không in chuỗi giống bí mật (canary).

## i18n và chất lượng

- [ ] **FE-OR-T411** Bổ sung bản dịch `en, es, ja, ko, zh` cho mọi chuỗi mới theo thứ tự khoá; kiểm JSON hợp lệ.
  Files: `i18n/locales/*.json`. Verify: `python3 -c "import json,…"` đọc được cả năm; ghi chú: `verify:localization-catalog`/`verify:localization-coverage` đang trỏ tới script không tồn tại trong `config/scripts` (cần khôi phục hoặc gỡ khỏi `package.json` — việc ngoài series này).
- [ ] **FE-OR-T412** Lint và kiểu: `npx oxlint`, `npx oxfmt --check`, `pnpm typecheck:web` cho các file đã chạm; test cả thư mục `outcome-review` (`cd frontend && npx vitest run src/renderer/src/components/request/outcome-review`).
  Verify: không lỗi mới; số test tăng tương ứng.

## E2E (khung kiểm của series)

- [ ] **FE-OR-T413** E2E **không AI** (agent giả, `--ai-stub`): flow ứng viên 3, agent giả vẽ 1 ⇒ thanh "Đã vẽ 1/3", danh sách chưa vẽ 2 dòng có lý do; bấm "Vẽ flow này" ⇒ hiện ước tính, hình xuất hiện, thanh thành 2/3. Thêm vào `suite_outcome_review.py` hoặc spec Playwright tương ứng.
  Files: `tests/stages/e2e/orca-flows/suite_outcome_review.py` (hoặc `ui-live/*`). Verify: chạy với stack stub; 0 lỗi console ngoài allowlist.
- [ ] **FE-OR-T414** E2E **thật** BL-E05 (chi phí dự kiến 2–4 USD/lần, thuộc nhóm chi phí C): mở bước kiểm `drawingsShowInTheArchitectureAndFlowTabs` (đã có, `ci-cd-kit-review-drawings.ts`) thành **khẳng định** khi review đã sinh: Architecture và Flow có hình ở cả Design và After code; tab Flow báo "Đã vẽ 2/2" (Chat FR-NTF-03, nhập Sheet FR-NTF-05); mỗi thẻ có hai khung; huy hiệu = số hàng; sau regenerate phạm vi `code`, `designBasis.digest` không đổi và số container/cạnh/flow không nhỏ đi.
  Files: `tests/stages/e2e/orca-flows/ui-live/ci-cd-kit-review-drawings.ts`, `ci-cd-kit-review-steps.ts`. Verify: chạy `ORCA_E2E_DEMANDING=1 ORCA_E2E_CICDKIT_REQUEST_ID=<id> python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit-review`; không chạy chồng với lượt khác (guard sẽ báo vi phạm an toàn).
- [ ] **FE-OR-T415** Báo cáo e2e (`ci-cd-kit-review.vi.md`): thêm dòng phủ flow ("Đã vẽ X/Y"), cơ sở thiết kế (số container/cạnh/flow), cảnh báo chấp nhận một phần, chi phí từng run; ghi "Kết quả (ngày): …" vào checklist này.
  Files: `ui-live/ci-cd-kit-review-steps.ts` (`factsOf`), `ui-live/ci-cd-kit-report-file.ts`. Verify: báo cáo của một lần chạy thật có các dòng mới.

## Cổng của pha

- [ ] `cd frontend && npx vitest run src/renderer/src/components/request/outcome-review` xanh; `pnpm typecheck:web` không lỗi mới ở file đã chạm.
- [ ] E2E thật BL-E05 đạt AC7 của CR-OR-041 và AC2 của CR-OR-040; mọi dòng E1–E13 của README có test hoặc bước e2e trỏ tới.

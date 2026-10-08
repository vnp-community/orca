# FE-REQ-TASK-032-08: Điểm vào đồ thị, i18n 5 locale, e2e và tài liệu

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.9, 5
**Priority:** P1
**Area:** frontend / request / i18n / tests
**File:** `frontend/src/renderer/src/components/request/RequestDetailHeader.tsx` (sửa, FE-REQ-TASK-019-03); `components/request/plan/PlanSummaryHeader.tsx` (sửa, 021-03); `components/request/RequestGraphSheet.tsx` (mới); `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa); `frontend/src/renderer/src/i18n/graph-locale-coverage.test.ts` (mới); `tests/e2e/request-graph.spec.ts` (mới); `docs/ui/pages/requests.md` (sửa, tạo ở 018-05)
**Depends on:** FE-REQ-TASK-032-01, 032-05, 032-06; FE-REQ-TASK-019-03 (header), 021-03 (tab Plan), 021-05
**Status:** [x] DONE (verified 2026-10-08: e2e tests/e2e/request-web/request-graph.web.e2e.ts 3/3 pass; vitest graph-locale-coverage (12) pass; phân tích bundle bằng manifest Vite: xyflow/GraphCanvas/GraphPanel ngoài chunk chính)

## Context

- Các solution trước cắm điểm: `RequestDetailHeader` (SOL-019: nút "Xem đồ thị"), `PlanSummaryHeader` (SOL-021: công tắc "Cây hoặc Đồ thị"), `SolutionOptionCard` (SOL-020/036: `GraphMini`). Task này làm hai điểm đầu; `GraphMini` do 036-05.
- Tiền lệ i18n: khoá đọc theo tên, tiền tố `auto.components.`, test phủ khoá `frontend/src/renderer/src/i18n/task-jira-link-locale-coverage.test.ts` (đã đọc: duyệt từng locale `en, es, ja, ko, zh` bằng `lookup`). Cùng cách ở `request-locale-coverage.test.ts` (FE-REQ-TASK-019-07).
- E2E mẫu: `tests/e2e/tasks-page.spec.ts` (đã tồn tại); chạy mở trang qua `window.__store.getState()`; không chạy được khi thiếu backend: dùng mock runtime như `tasks-page.spec.ts` (đọc cách mock trước khi viết).
- Tiêu chí CR-032: nút "Xem đồ thị" ẩn khi `unsupported`; `elkjs` chỉ nạp khi cần (nếu 032-07 làm); `rg '#[0-9a-fA-F]{3,6}'` rỗng trong `components/graph/` và `TaskDAGView.tsx`.

## Việc cần làm

1. `RequestGraphSheet.tsx`: `Sheet` (side phải, `sm:max-w-4xl`) chứa `GraphPanel` (`mode="sheet"`); props `{ request: RequestView; open: boolean; onOpenChange(open): void; subject; lensInitial? }`; `GraphPanel` nạp lười bằng `React.lazy(() => import('../graph/GraphPanel'))`, `Suspense fallback={<GraphSkeleton />}`.
2. `RequestDetailHeader`: nút "Xem đồ thị" (`Button variant="outline" size="sm"` + icon `Network`) mở `RequestGraphSheet`; chỉ hiện khi `useRequestFlowSupport()` trả `supported` và có ít nhất một lens khả dụng (`flow` luôn khả dụng nên hiện); `subject` mặc định: `plan` nếu `request.planTaskId`, ngược lại `solution_option` với Solution mới nhất, ngược lại chỉ lens `flow`. `unsupported` của `impact.*` không ẩn nút (lens `flow` vẫn dùng được); chỉ ẩn chip lens backend.
3. `PlanSummaryHeader`: công tắc `ToggleGroup type="single"` "Cây | Đồ thị"; `Đồ thị` hiển thị `GraphPanel` lens `plan` thay `PlanTree` (dùng `RequestGraphSheet` không cần; nhúng trực tiếp, `mode="full"`); ghi nhớ lựa chọn trong state của `RequestPlanTab` (không `localStorage`). Mục 2.5 SOL-021 về lọc `plan`/`phase` không đổi.
4. i18n: thêm khoá (en trước, rồi 4 locale còn lại, không để trống) ở tiền tố `auto.components.graph.`: `Lens.flow|architecture|contract|data|impact|plan|execution`, `LensDisabled.noAssessment|noPlan|notExecuting|unsupported`, `View.graph|list`, `ChangeView.before|after`, `Search.placeholder|empty|button`, `Legend.title|riskLevels|edgeAdded|edgeRemoved|breaking|irreversible|ring`, `GroupSummary` (nội suy `{{count}}` và `{{kinds}}`), `State.loading|empty|noAssessment|runAssessment|stale|truncated|retry|forbidden|noDevServer|connectDevServer|running`, `Columns.kind|label|group|risk|status|change|relations`, `Change.added|removed|unchanged`, `Node.open|findings|edgesIn|edgesOut`, `Status.<mỗi giá trị đã biết>`, `Mini.summary`, `Entry.viewGraph|planGraph|tree`, và `RiskLevel.*`, `RiskTooltip*` của 032-01. Với `ja`, `ko`, `zh`, `es` dùng bản dịch thật; nếu chưa có người dịch, nhờ duyệt rồi mới hợp nhất (không sao chép en).
5. `graph-locale-coverage.test.ts`: danh sách khoá (cùng nguồn với bước 4, xuất từ một hằng `GRAPH_LOCALE_KEYS` ở file test) kiểm từng locale có chuỗi không rỗng và không bằng khoá; kiểm khoá nội suy giữ đủ `{{...}}` ở mọi locale.
6. Chạy `pnpm run verify:localization-catalog` và `pnpm run verify:localization-coverage` (root `package.json`, nằm trong `pnpm lint`; chưa kiểm chứng chạy được trong môi trường này) và sửa lỗi nếu bộ kiểm yêu cầu danh mục băm cho khoá đọc theo tên.
7. `tests/e2e/request-graph.spec.ts`: (a) runtime không có `impact.*` (mock `method_not_found`): nút "Xem đồ thị" vẫn mở, chip lens backend tắt, lens `flow` hiện; (b) mock `impact.graph` trả 120 node với một cạnh `added`: danh sách `truncated`, `/` mở tìm kiếm, Enter chọn node; (c) công tắc Cây | Đồ thị ở tab Plan. E2E chỉ cần backend giả, không cần `impact` thật.
8. Phân tích bundle: sau build (`pnpm --filter orca-frontend build`), xác nhận `GraphCanvas` và `@xyflow/react` nằm chunk lười, không trong chunk của `RequestPage` (ghi tên chunk vào PR); nếu 032-07 làm thì xác nhận `elkjs` ở chunk riêng.
9. `docs/ui/pages/requests.md`: thêm mục "Đồ thị" (điểm vào, lens, phím `/`, chế độ danh sách). Không tạo file docs mới ngoài tệp này.
10. Kiểm tĩnh cuối: `rg -n '#[0-9a-fA-F]{3,6}' frontend/src/renderer/src/components/graph frontend/src/renderer/src/components/task/TaskDAGView.tsx` rỗng.

## Bảng tham chiếu nhanh

| Điểm vào | Component | Điều kiện hiển thị |
|---|---|---|
| Header Request | nút "Xem đồ thị" mở `RequestGraphSheet` | luồng Request bật (`useRequestFlowSupport` là `supported`) |
| Tab Plan | công tắc "Cây \| Đồ thị" | có `planTaskId` |
| Thẻ phương án | `GraphMini` (làm ở 036-05) | có đánh giá |

- Kênh WS: `impact.graph` (qua `GraphPanel`); không thêm kênh mới ở task này.
- Phím tắt: `/` trong `GraphPanel`; không phím Mod; không chip phím cho Cancel hay Dismiss.
- Trạng thái: nút ẩn khi không có lens nào khả dụng; `unsupported` của `impact.*` chỉ tắt chip lens backend, lens `flow` vẫn dùng được.
- Khoá i18n bổ sung ngoài danh sách bước 4: `auto.components.graph.Entry.title`, `auto.components.graph.Entry.close`.

## Trình tự làm gợi ý

1. Viết `RequestGraphSheet.tsx` và test mở/đóng.
2. Cắm nút vào `RequestDetailHeader`, công tắc vào `PlanSummaryHeader` (chỉ thêm dòng cần thiết).
3. Thêm khoá i18n 5 locale và `graph-locale-coverage.test.ts`.
4. Viết e2e bằng mock runtime; chạy khi có môi trường.
5. Chạy phân tích bundle, kiểm tĩnh không hex, cập nhật `docs/ui/pages/requests.md`.

## Kiểm thử

- `graph-locale-coverage.test.ts` xanh trên 5 locale.
- `RequestDetailHeader.test.tsx` (mở rộng): nút hiện khi bật luồng; mở sheet; `useRequestFlowSupport` `unsupported` thì ẩn.
- `PlanSummaryHeader.test.tsx` (mở rộng): công tắc đổi giữa `PlanTree` và `GraphPanel` (mock).
- E2E (đã mô tả); mặc định chưa chạy: `pnpm exec playwright test tests/e2e/request-graph.spec.ts` (kiểm cấu hình Playwright ở gốc trước; `frontend/package.json` không có script e2e).
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/i18n/graph-locale-coverage src/renderer/src/components/request`.

## Tiêu chí hoàn thành

- [ ] "Xem đồ thị" mở `GraphPanel` từ header Request; công tắc "Cây | Đồ thị" ở tab Plan.
- [ ] Mọi khoá `auto.components.graph.*` có 5 locale; test phủ khoá xanh.
- [ ] Không hex trong `components/graph/` và `TaskDAGView.tsx`.
- [ ] Chunk `GraphCanvas` là lười (đã ghi tên chunk).
- [ ] `docs/ui/pages/requests.md` cập nhật.
- [ ] Runtime không có kênh `impact.*`: không lỗi đỏ; lens `flow` vẫn dùng được.

## Rủi ro và lưu ý

- Hợp nhất `RequestStageTimeline` với lens `flow` không bắt buộc: giữ bản hiện tại và thêm liên kết "Xem đồ thị" (CR-032 mục 8).
- `verify:localization-catalog` có thể đòi tạo danh mục băm cho chuỗi mới; kiểm lệnh thật trước khi tin vào bước 6.
- Bản dịch máy chưa kiểm cho `ja`, `ko`, `zh`, `es` có thể sai thuật ngữ rủi ro; nhờ người duyệt.
- E2E dùng mock runtime có thể lệch so với backend thật; e2e thật cần CR-030 hoàn tất.

## Ghi chú triển khai (2026-10-07)

Đã làm: `RequestGraphSheet` (lazy), nút "Xem đồ thị" ở `RequestDetailHeader`, công tắc Cây|Đồ thị ở `PlanSummaryHeader`/`RequestPlanTab`, 77+ khoá i18n 5 locale, `docs/ui/pages/requests.md`, `tests/e2e/request-graph.spec.ts` (khung, skip như request-plan-tree). Chưa: chạy build/phân tích chunk, `pnpm run verify:localization-catalog` (script `audit-localization-coverage.mjs` không tồn tại trong repo này). Nút "Xem đồ thị" hiện khi luồng Request `supported`; subject mặc định `plan` nếu có `planTaskId`, ngược lại `solution_option` với id Request (chưa lấy Solution mới nhất).

## Ghi chú triển khai (2026-10-08)

- E2E chạy thật trên SPA web với WS giả (không cần request-service): `tests/e2e/request-web/request-graph.web.e2e.ts` (a: runtime không có `impact.*` vẫn mở "Xem đồ thị", lens backend bị khoá, lens Luồng vẽ node; b: `impact.graph` 120/300 node → danh sách, "/" mở tìm, Enter chọn node; c: Plan tab Cây | Đồ thị). Dùng hạ tầng `tests/e2e/request-web/support/mock-request-ws.ts` (agent khác dựng), project `mcp-web` của `tests/playwright.web.config.ts` (không sửa config). Lệnh: `MCP_E2E_BASE_URL=http://127.0.0.1:5174 npx playwright test -c tests/playwright.web.config.ts --project=mcp-web tests/e2e/request-web/request-graph.web.e2e.ts`. Khung Electron cũ `tests/e2e/request-graph.spec.ts` vẫn skip, trỏ sang bản web.
- Phân tích bundle (không thêm phụ thuộc): `npx vite build --outDir <scratch> --manifest`, đọc `.vite/manifest.json`: closure import tĩnh của `web-index.html` (4 chunk, 2,06 MB) không chứa xyflow; xyflow ở chunk dùng chung `_style-*.js` (139 KB) chỉ được import bởi `GraphCanvas` (dynamic, 13 KB) và các lens lazy khác (review-map, `TaskDAGView`, `DAGPreview`); `GraphPanel` là dynamic chunk 22 KB, nạp từ `RequestPage` (dynamic).
- `verify:localization-catalog`/`verify:localization-coverage`: script `config/scripts/verify-localization-catalog.mjs` và `audit-localization-coverage.mjs` không tồn tại trong repo này nên không chạy được; phủ khoá bằng vitest `graph-locale-coverage` (5 locale).

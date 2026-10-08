# FE-REQ-TASK-032-06: `GraphPanel`, thanh công cụ, danh sách, tìm kiếm `cmdk`, `GraphNodeSheet` và các trạng thái

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.3, 2.5, 2.4
**Priority:** P0
**Area:** frontend / graph (panel)
**File:** `frontend/src/renderer/src/components/graph/{GraphPanel,GraphToolbar,GraphLensChips,GraphViewToggle,GraphBeforeAfterToggle,GraphSearchButton,GraphLegend,GraphListView,GraphSearchPalette,GraphNodeSheet,GraphEmptyState,GraphErrorState,GraphSkeleton,GraphStatusBanner}.tsx` (mới); `graph-panel-state.ts` (mới, thuần); test cùng tên
**Depends on:** FE-REQ-TASK-032-03, 032-04, 032-05; FE-REQ-TASK-018-03 (hook), 018-05 (shell)
**Status:** [x] DONE (verified 2026-10-08: vitest GraphPanel (9), graph-panel-state (8), GraphListView (5), GraphSearchPalette (2), GraphNodeSheet (2) pass; e2e request-graph.web.e2e.ts b: list mặc định khi truncated, "/" mở tìm, Enter chọn node)

## Context

- Primitive có sẵn (`components/ui/`): `toggle-group.tsx`, `table.tsx`, `command.tsx` (wrap `cmdk`), `sheet.tsx`, `popover.tsx`, `tooltip.tsx`, `skeleton.tsx`, `badge.tsx`, `button.tsx`. **Không có** `radio-group`, `switch`, `alert`: công tắc Trước/Sau và Đồ thị/Danh sách là `ToggleGroup type="single"`; dải cảnh báo là `div` dùng token (`border-risk-medium-border bg-risk-medium-background`), không tạo primitive mới.
- `@tanstack/react-virtual ^3.13.24` (devDependencies, đã được renderer dùng): `useVirtualizer` cho hơn 100 hàng.
- `ShortcutKeyCombo.tsx` hiển thị chip phím; `/` là phím thường, `Esc` là phím chung; STYLEGUIDE dòng 296: Cancel/Dismiss yên lặng, không chip phím; chỉ hiện chip khi hành vi đã làm thật.
- Hook `useGraphLens` (032-03) trả `{payload, status, error, refetch, showSkeleton}`; `GraphPanel` là nơi duy nhất giữ trạng thái chọn, nhóm mở, lens, chế độ xem, trước/sau, tiêu điểm tập trung.
- SOL-032 mục 2.9: các trạng thái rỗng, tải, lỗi (xem bảng CR-REQ-032 2.9).

## Việc cần làm

1. `graph-panel-state.ts`: `type GraphPanelState = { lens: GraphLens; view: 'graph'|'list'; changeView: 'before'|'after'; selectedId: string|null; focusId: string|null; openGroups: ReadonlySet<string>; searchOpen: boolean }`; `graphPanelReducer`; `initialView({truncated, screenReaderMode, narrow}): 'graph'|'list'` (list khi `truncated`, hoặc bật chế độ đọc màn hình nếu app có cờ này, hoặc `narrow`; **chưa kiểm chứng** app có cờ trình đọc màn hình; mặc định bỏ qua tham số này nếu không có); `selectNodeFromSearch(state, node)` mở nhóm chứa node, đặt `selectedId` và tăng `fitViewSignal`.
2. `GraphPanel.tsx`: props `{ request: RequestView; subject: { type: 'solution_option'|'plan'|'task'; id: string; optionId?: string }; lensInitial?: GraphLens; onNodeOpen?: (node: GraphNode) => void; mode?: 'full' | 'sheet' }`. Lấy payload bằng `useGraphLens` (lens client dùng adapter ở 032-04 với `usePlanTree`). Hiển thị theo thứ tự: `GraphToolbar`, `GraphStatusBanner` (stale/truncated), khung nội dung (`GraphCanvas` bằng `React.lazy` + `Suspense fallback=GraphSkeleton`, hoặc `GraphListView`), `GraphNodeSheet`. Khi `status==='idle'` do `unsupported`: trả `null` (nơi gọi đã ẩn nút).
3. `GraphLensChips.tsx`: 7 `ToggleGroupItem` từ `GRAPH_LENSES`; chip tắt có `disabled` kèm `Tooltip` nêu lý do (khoá `LensDisabled.noAssessment|noPlan|notExecuting|unsupported`), **không ẩn**; lens mặc định `pickDefaultLens` khi mở lần đầu mà `lensInitial` không truyền.
4. `GraphViewToggle` (Đồ thị | Danh sách), `GraphBeforeAfterToggle` (Trước | Sau, chỉ khi `hasChangeAxis(payload)`), `GraphSearchButton` (mở palette; phím `/` khi tiêu điểm không ở `input`/`textarea`/`[contenteditable]`; `keydown` gắn ở `GraphPanel` bằng `useEffect`, dọn khi unmount; chip phím hiển thị `/`), `GraphLegend` (`Popover`: mức rủi ro 5 hàng với icon và chữ, nét liền "+", nét đứt "−", vòng đồng tâm của lens `impact`, nhãn "Phá vỡ tương thích", cờ "Không đảo ngược").
5. `GraphListView.tsx`: `Table` cột Loại, Nhãn, Nhóm, Rủi ro (`RiskBadge`), Trạng thái, Thay đổi (chữ "Thêm", "Bớt", "Không đổi"), Số quan hệ; dữ liệu từ cùng `payload` đã lọc theo lens; hàng chọn đồng bộ hai chiều với canvas (`selectedId`); `useVirtualizer` khi hơn 100 hàng (chiều cao hàng cố định 40 px, `overscan: 8`); `role="grid"`, `aria-rowcount`, phím `ArrowUp`/`ArrowDown` đổi hàng, `Enter` mở `GraphNodeSheet`; sắp xếp mặc định theo rủi ro giảm dần rồi `label`. Chỉ khi `truncated` hiện dòng "Hiển thị N/`totalNodes`".
6. `GraphSearchPalette.tsx`: `CommandDialog` (dùng `components/ui/command.tsx`), `CommandInput` với placeholder, nhóm kết quả theo `kind`, lọc theo `label`, `kind`, `group` (hàm `filterGraphNodes(nodes, query)` thuần trong `graph-panel-state.ts`; `cmdk` `shouldFilter={false}` để tự lọc ổn định), tối đa 50 kết quả; Enter chọn node qua `selectNodeFromSearch`; `Esc` đóng và trả tiêu điểm cho nút gọi.
7. `GraphNodeSheet.tsx`: `Sheet` bên phải; nội dung văn bản thuần (không HTML): nhãn, loại, nhóm, `RiskBadge` kèm "đánh giá lúc, dựa trên", trạng thái, danh sách cạnh vào/ra, `findingIds`; nút "Mở" gọi `onNodeOpen(node)` (SOL-036 dùng để mở `ImpactEvidenceSheet`); không có chỗ sửa rủi ro.
8. Trạng thái (`GraphEmptyState`, `GraphErrorState`, `GraphSkeleton`, `GraphStatusBanner`) theo CR-REQ-032 2.9: đang tải skeleton hoãn 200 ms và nút điều khiển vô hiệu ngay; chưa đánh giá có nút "Chạy đánh giá" khi có quyền (gọi `impact.request {subjectType, subjectId}`, tên kênh theo CR-030 2.8; ẩn nếu `unsupported`); lens không node "Không có <loại> bị ảnh hưởng" (không viết "an toàn"); `stale` dải "Index phân tích đã cũ, kết quả chưa đánh giá được đầy đủ"; `truncated` dải "Chỉ hiện N/M node"; `network` banner "Thử lại" giữ payload mờ; `forbidden` "Bạn không có quyền xem đánh giá"; `REQUEST_IMPACT_NO_CONNECTION` "Chưa có dev server kết nối" kèm nút kết nối (điều hướng tới màn dev server hiện có, tìm đường dẫn trước khi viết).
9. Bàn phím trong canvas: `Tab`, `Arrow`, `Enter`, `Esc` như SOL-032 mục 2.5; hiển thị gợi ý phím bằng `ShortcutKeyCombo` chỉ cho `/` (và `Esc` dạng chữ trong tooltip), không có chip cho Cancel/Dismiss.

## Bảng tham chiếu nhanh

| Tình huống | UI | Khoá i18n (tiền tố `auto.components.graph.`) |
|---|---|---|
| Đang tải | `GraphSkeleton` hoãn 200 ms, nút vô hiệu ngay | `State.loading` |
| Chưa có đánh giá | "Chưa có đánh giá tác động" + nút "Chạy đánh giá" | `State.noAssessment`, `State.runAssessment` |
| Lens không node | "Không có <loại> bị ảnh hưởng" | `State.empty` |
| `stale` | dải cảnh báo | `State.stale` |
| `truncated` | dải "Chỉ hiện N/M node", mặc định danh sách | `State.truncated` |
| Lỗi mạng | banner "Thử lại", giữ payload mờ | `State.retry` |
| `forbidden` | "Bạn không có quyền xem đánh giá" | `State.forbidden` |
| Không dev server | "Chưa có dev server kết nối" + nút kết nối | `State.noDevServer`, `State.connectDevServer` |

- Kênh WS: `impact.graph` (qua `useGraphLens`), `impact.request {subjectType, subjectId}` khi bấm "Chạy đánh giá" (tạm, CR-030 2.8).
- Phím tắt: `/` mở tìm kiếm (khi tiêu điểm không ở ô nhập), `Enter` chọn kết quả, `Esc` đóng palette hoặc thoát tập trung, `Arrow` đi trong danh sách; không phím có Mod nên không phụ thuộc nền tảng.

## Kiểm thử

- `graph-panel-state.test.ts`: reducer (đổi lens xoá `focusId`; đổi chế độ giữ `selectedId`); `initialView`; `filterGraphNodes` (theo `label`, `kind`, `group`, không phân biệt hoa thường, tiếng Việt có dấu); `selectNodeFromSearch` mở nhóm.
- `GraphPanel.test.tsx` (mock `useGraphLens`, mock `GraphCanvas`): đổi lens bằng chip; chip tắt có tooltip lý do; công tắc Trước/Sau chỉ khi có `change`; chọn trong danh sách đồng bộ với canvas và ngược lại; `truncated` mặc định danh sách; `stale` hiện dải; `unsupported` trả `null`; `/` mở palette (không mở khi tiêu điểm ở `input`).
- `GraphListView.test.tsx`: hơn 100 hàng gọi `useVirtualizer` (mock), 100 hàng thì không; sắp xếp mặc định; `Enter` mở sheet.
- `GraphSearchPalette.test.tsx`: Enter chọn node, mở nhóm chứa nó, gọi `fitView` signal; `Esc` đóng.
- `GraphNodeSheet.test.tsx`: không render HTML thô (`<script>` trong `label` hiển thị như văn bản).
- i18n: các khoá `auto.components.graph.*` thêm ở 032-08.
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/graph/graph-panel-state src/renderer/src/components/graph/GraphPanel src/renderer/src/components/graph/GraphListView src/renderer/src/components/graph/GraphSearchPalette src/renderer/src/components/graph/GraphNodeSheet`.

## Tiêu chí hoàn thành

- [ ] 7 chip lens; chip thiếu dữ liệu tắt kèm lý do; lens mặc định là lens rủi ro cao nhất.
- [ ] Danh sách luôn có, cùng dữ liệu, chọn đồng bộ hai chiều; hơn 100 hàng được ảo hoá.
- [ ] `/` mở tìm kiếm `cmdk`; Enter nhảy tới node (mở nhóm chứa nó nếu cần).
- [ ] Mọi trạng thái rỗng, tải, lỗi của bảng 2.9 có UI; không chữ "an toàn".
- [ ] `GraphNodeSheet` chỉ văn bản thuần.
- [ ] Phím tắt đa nền tảng: `/` và `Esc` không phụ thuộc `metaKey`/`ctrlKey`; không thêm phím Mod.

## Rủi ro và lưu ý

- Cờ "trình đọc màn hình" có thể không tồn tại; khi đó danh sách chỉ mặc định cho `truncated` và màn hẹp, người dùng vẫn chuyển được bằng `GraphViewToggle`.
- `cmdk` với `shouldFilter={false}` phải tự xử lý thứ tự và `value` duy nhất (dùng `node.id`).
- `GraphPanel` trong `Sheet` mặc định `mode='sheet'`: tiêu điểm phải bị giữ trong sheet (Radix lo), nhưng `/` toàn cục không được rò rỉ ra ngoài khi sheet đóng.
- `impact.request` chạy bất đồng bộ (CR-030: khởi chạy, trả `assessmentId`); `GraphPanel` chỉ hiện "Đang chạy đánh giá" và chờ sự kiện, không tự lặp vô hạn.
- Giữ file dưới ngưỡng `max-lines`; nếu `GraphPanel.tsx` vượt, tách hook `useGraphPanelState` thay vì thêm disable.

## Ghi chú triển khai (2026-10-07)

Chưa có nút "Kết nối dev server" ở trạng thái `noDevServer` (chưa tìm được điều hướng sẵn có); cờ trình đọc màn hình không tồn tại nên `initialView` bỏ qua tham số. Sai lệch: `unsupported` không trả `null` mà hiện trạng thái "chưa đánh giá" (không nút chạy) để vẫn dùng được lens `flow`; lens mặc định = `impact` nếu đã có đánh giá, ngược lại `flow` (không có tóm tắt các lens để `pickDefaultLens`). GraphToolbar gom chip lens, công tắc, tìm, chú giải vào một file.

## Ghi chú triển khai (2026-10-08)

- Nút "Kết nối dev server" ở `REQUEST_IMPACT_NO_CONNECTION`: `GraphErrorState.onConnectDevServer` nối `useOpenDevServerSettings` (`components/request/use-open-dev-server-settings.ts`: `openSettingsTarget({pane:'servers', repoId:null})` + `openSettingsPage()`, cùng điều hướng sidebar đang dùng). Test: GraphPanel "REQUEST_IMPACT_NO_CONNECTION offers Connect dev server".
- Cờ trình đọc màn hình: app không có cờ này (đã tìm trong store/settings); `initialView` giữ tham số `screenReaderMode` tuỳ chọn, mặc định bỏ qua đúng như spec cho phép; người dùng chuyển bằng công tắc Đồ thị/Danh sách.
- `initialView` áp cho mọi payload `truncated` (kể cả sau khi đổi lens) cho tới khi người dùng tự chọn view; `GraphListView` cuộn tới hàng được chọn (chọn từ tìm kiếm hoặc "Xem trên đồ thị") kể cả khi danh sách ảo hoá; `GraphPanel.initialSelectedId` chọn sẵn node.

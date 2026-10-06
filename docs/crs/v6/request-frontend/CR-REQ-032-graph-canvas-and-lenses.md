# CR-REQ-032 — Thành phần đồ thị chung `GraphCanvas` và các lens

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-032 |
| **Tên** | Một thành phần đồ thị dùng `@xyflow/react` với 7 lens (Luồng, Kiến trúc, Hợp đồng, Dữ liệu, Phạm vi ảnh hưởng, Kế hoạch, Thực thi); hợp đồng dữ liệu đồ thị; gom nhóm, thu phóng ngữ nghĩa, chế độ tập trung, trước/sau, dạng danh sách, tìm bằng `cmdk`; chuyển `TaskDAGView` sang token màu |
| **Loại** | Feature + Refactor |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-018 (kiểu, hook, lỗi `RequestRpcError`), CR-REQ-019 đến 021 (nơi đặt nút "Xem đồ thị"); backend CR-REQ-030 (đang soạn: truy vấn tác động trả đúng dạng đồ thị); kênh CR-REQ-016 (chờ cập nhật) |
| **Mở khoá** | CR-REQ-036 (thẻ rủi ro và so sánh phương án dùng `GraphMini`, `riskPresentation`) |
| **Tác động** | `frontend/src/renderer/src/components/graph/` (mới), `shared/graph-types.ts` (mới), `assets/main.css` và `guides/STYLEGUIDE.md` (token rủi ro), `components/task/TaskDAGView.tsx` (đổi màu), `package.json` (phụ thuộc mới, cần duyệt) |

> Nguồn: [frontend-visualization-and-ux.md](../../../research/receive-request/frontend-visualization-and-ux.md) mục 4, 5, 8; [impact-assessment-and-risk-scoring.md](../../../research/receive-request/impact-assessment-and-risk-scoring.md) mục 4 đến 6. File nghiên cứu đề xuất số CR-REQ-031 cho việc này; số 031 trùng "Source Registry và Context Pack Builder" nên CR này lấy số **032**.

---

## 1. Bối cảnh và vấn đề

Request, Solution, Plan, thực thi và đánh giá tác động (CR-REQ-030) đều có dạng đồ thị. Nếu mỗi màn tự vẽ, ta có bảy đồ thị khác nhau về màu, phím tắt, hành vi thu phóng và a11y. Nghiên cứu chốt: một canvas, nhiều lens, đổi lens bằng chip; mặc định chỉ hiện thẻ tóm tắt, đồ thị là lớp chi tiết.

Hiện trạng đã đọc (2026-10-06):

- `components/task/TaskDAGView.tsx` (273 dòng) dùng `@xyflow/react` (`^12.11.2`), tự xếp node theo "làn" (`buildDAGLayout`, `HORIZONTAL_GAP=220`), **hardcode hex** ở `STATUS_COLORS` (dòng 26 đến 34) và `style={{ background, border }}` từng node, cộng `#94a3b8` cho cạnh và `#6b7280` cho chữ. Trái `guides/STYLEGUIDE.md` ("Never hardcode a hex value"). Nó còn dùng `<select>` thô và `window.confirm`; ngoài phạm vi, chỉ ghi nhận.
- `TaskGraph.tsx` (299 dòng) nạp `TaskDAGView` bằng `lazy`, đổi chế độ `tree|dag|board` bằng nút thô. `components/workflow/DAGPreview.tsx` cũng dùng xyflow.
- `package.json` đã có `@xyflow/react`, `@tanstack/react-virtual`, `cmdk` (qua `components/ui/command.tsx`), `radix-ui`, `zustand`, `mermaid`. **Chưa có** thư viện bố trí tự động (`elkjs`, `dagre`).
- `hooks/usePrefersReducedMotion.ts` đã có. `components/ui/` có `table`, `toggle-group`, `command`, `sheet`, `skeleton`, `tooltip`.
- **Token màu rủi ro: thiếu.** `main.css` có `--destructive` (`#e40014` sáng, `#ff6568` tối), `--status-success`, `--ai-action-accent`, `--chart-1..5` (chỉ sắc xanh). Không có token vàng, cam cho "Trung bình", "Cao". `--git-decoration-*` bị STYLEGUIDE cấm dùng ngoài git. Dark mode theo lớp `.dark` (không phải `prefers-color-scheme`).
- Mâu thuẫn với v7: CR-CV-050 mục 2.10 thêm `--review-changed/affected/untested/violation`; CR-CV-055 vẽ C4 bằng xyflow với bố cục tự viết, và README v7 O5 quy định **không** thêm `elkjs`/`dagre` ở MVP. Xem mục 3 và 8.

## 2. Giải pháp đề xuất

### 2.1 Hợp đồng dữ liệu đồ thị (`shared/graph-types.ts`, mới)

Khớp nghiên cứu mục 4; CR-REQ-030 phải trả đúng dạng này (đang soạn, chờ xác nhận).

```ts
type GraphRisk = 'low' | 'medium' | 'high' | 'critical' | 'unknown'
type GraphChange = 'added' | 'removed' | 'unchanged'

type GraphNode = {
  id: string            // ổn định giữa các lần gọi
  kind: string          // service|module|file|symbol|proto|ws_channel|event|mcp_tool|table|migration|phase|task|step
  label: string
  group: string | null  // đường dẫn nhóm "service/module", dùng để gom và thu phóng
  risk: GraphRisk       // 'unknown' = chưa đánh giá, KHÔNG được vẽ như 'low'
  status: string | null // theo lens: trạng thái task/request, 'breaking', 'irreversible'...
}
type GraphEdge = { from: string; to: string; kind: string; change: GraphChange }

type GraphPayload = {
  lens: GraphLens
  nodes: GraphNode[]
  edges: GraphEdge[]
  totalNodes: number    // trước khi cắt
  truncated: boolean
  assessedAt: string | null   // chuỗi hiển thị "đánh giá lúc <giờ>"
  tool: string | null         // công cụ nguồn (CodeGraph, GitNexus, buf...)
  stale: boolean              // index lỗi thời: hiện nhãn "chưa đánh giá được"
}
```

- Trường lạ bị bỏ qua; `kind`, `status` lạ rơi về hình mặc định và nhãn thô, không ném lỗi (cùng nguyên tắc parser của CR-REQ-018). Parser `parseGraphPayload` nằm trong `shared/graph-wire-parsers.ts` (mới).
- Cạnh trỏ tới node không có trong `nodes` bị bỏ và đếm vào `droppedEdges` (hiện ở chú thích).
- `risk` và `change` là hai trục độc lập: một node `critical` vẫn có thể `unchanged`.

### 2.2 Lens (`graph-lens-registry.ts`, mới)

| Lens | Node `kind` | Cạnh `kind` | Dấu hiệu ngoài màu | Nguồn dữ liệu |
|---|---|---|---|---|
| `flow` Luồng | `step` (bước Request) | `transition` | trạng thái bằng icon | **Phía client**, dựng từ `REQUEST_FLOW_REGISTRY` và `status` (CR-REQ-018), không gọi backend |
| `architecture` Kiến trúc | `service`, `module` | `depends_on`, `calls` | cạnh thêm nét liền "+", bớt nét đứt "−" | CR-REQ-030 |
| `contract` Hợp đồng | `proto`, `ws_channel`, `event`, `mcp_tool` | `used_by` | nhãn "Phá vỡ tương thích" kèm icon | CR-REQ-030 |
| `data` Dữ liệu | `table`, `migration` | `owns`, `references` | cờ "Không đảo ngược", chip "PG/MySQL" | CR-REQ-030 |
| `impact` Phạm vi ảnh hưởng | `symbol`, `flow` | `calls` | vòng đồng tâm theo bước nhảy (đổi, trực tiếp, gián tiếp) kèm số bước | CR-REQ-030 |
| `plan` Kế hoạch | `phase`, `task` | `depends_on` | biểu đồ nhiệt rủi ro kèm chữ | **Phía client** từ `usePlanTree` (CR-REQ-021) cộng `risk` từ CR-REQ-030 |
| `execution` Thực thi | `task` | `depends_on` | trạng thái trực tiếp, nhãn "lệch" | **Phía client** từ sự kiện task (CR-REQ-013) cộng bản đánh giá thực tế |

- Lens mặc định khi mở: lens có `risk` cao nhất trong bản đánh giá; nếu không có đánh giá thì `flow`.
- Lens không có dữ liệu: chip tắt kèm tooltip lý do, không ẩn.
- Mỗi lens là cấu hình `{id, labelKey, icon, nodeKinds, edgeKinds, legend, load}`; thêm lens không sửa `GraphCanvas`.

### 2.3 Cây component (`components/graph/`, mới)

```
GraphPanel                     props: payload | source, lensInitial, onNodeOpen, mode
├─ GraphToolbar
│    ├─ GraphLensChips          (ui/toggle-group, 7 chip, chip tắt có tooltip)
│    ├─ GraphViewToggle         (Đồ thị | Danh sách)
│    ├─ GraphBeforeAfterToggle  (Trước | Sau, chỉ lens có `change`)
│    ├─ GraphSearchButton       (mở GraphSearchPalette, phím "/")
│    └─ GraphLegend             (Popover: màu, hình, nét)
├─ GraphCanvas                  (ReactFlow, tải lười)
│    ├─ GraphNodeCard x N       (kind icon, label, RiskBadge, status)
│    ├─ GraphGroupNode x M      (cụm có thể mở/thu, "+12 service không bị ảnh hưởng")
│    ├─ GraphEdgeLine           (nét theo `change`, nhãn "+" "−")
│    └─ Controls, MiniMap (chỉ khi > 30 node)
├─ GraphListView                (ui/table.tsx + @tanstack/react-virtual)
├─ GraphSearchPalette           (ui/command.tsx, cmdk)
├─ GraphNodeSheet               (ui/sheet.tsx: chi tiết, bằng chứng, nút "Mở")
└─ GraphEmptyState / GraphErrorState / GraphSkeleton
```

`GraphMini` (mới) là bản thu nhỏ chỉ đọc, không toolbar, tối đa 12 node, dùng trong cột của bảng so sánh phương án (CR-REQ-036). Có `aria-hidden` khi `GraphListView` tương ứng đã có, và luôn kèm văn bản tóm tắt ("5 service, 2 cạnh thêm, 1 phá vỡ").

### 2.4 Hành vi

| Tính năng | Quy tắc |
|---|---|
| **Gom nhóm > 50 node** | `graph-grouping.ts` (mới, thuần, có test) chọn tối đa `VISIBLE_NODE_LIMIT = 50` node nhìn thấy theo thứ tự ưu tiên: node có `change != unchanged`, `risk` cao, láng giềng 1 bước của node đang chọn. Phần còn lại gom theo `group` thành `GraphGroupNode` "+N <loại> không bị ảnh hưởng"; bấm để mở nhóm tại chỗ. Con số 50 theo nghiên cứu, chưa đo |
| **Thu phóng ngữ nghĩa** | Dựa `zoom` của xyflow: dưới 0,5 chỉ thấy cấp 1 của `group` (service); 0,5 đến 1,2 cấp module; trên 1,2 node lá (file, symbol). Ngưỡng là hằng ở `graph-zoom-levels.ts` (mới). Với `prefers-reduced-motion` không chạy hoạt ảnh `fitView` |
| **Chế độ tập trung** | Bấm node: node và láng giềng 1 bước giữ nguyên, phần còn lại `opacity` thấp (và `pointer-events` vẫn bấm được); `Esc` thoát; trạng thái trong `graph-focus-state.ts` (mới) |
| **Trước và sau** | Một công tắc. "Sau" (mặc định): cạnh `removed` vẽ mờ, nét đứt, nhãn "−". "Trước": ẩn `added`, hiện `removed` như cạnh thường. Không có hai đồ thị cạnh nhau |
| **Luôn có danh sách** | `GraphViewToggle` chuyển sang `GraphListView` cùng dữ liệu, cùng bộ lọc lens, cùng node đang chọn. Cột: loại, nhãn, nhóm, rủi ro (icon và chữ), trạng thái, thay đổi, số quan hệ. Ảo hoá bằng `useVirtualizer` khi > 100 hàng. Là chế độ **mặc định** khi trình đọc màn hình bật hoặc khi `truncated` |
| **Tìm và nhảy** | Phím `/` (khi tiêu điểm không ở ô nhập) hoặc nút mở `GraphSearchPalette`; lọc theo `label`, `kind`, `group`; Enter chọn node, mở nhóm chứa nó, `fitView` tới đó và đặt tiêu điểm bàn phím |
| **Không mã hoá chỉ bằng màu** | Xem 2.5 |
| **Bàn phím** | Tiêu điểm vào canvas rồi `Tab` qua node theo thứ tự danh sách; `Arrow` nhảy sang láng giềng; `Enter` mở `GraphNodeSheet`; `Esc` thoát tập trung. Nhãn phím theo nền tảng bằng `ShortcutKeyCombo`; chỉ hiện chip phím khi đã làm thật |

### 2.5 Màu, hình và token

Nguyên tắc: màu chỉ là lớp thứ ba, sau chữ và hình.

| Mức | Chữ | Hình/icon (lucide, đề xuất) | Viền node |
|---|---|---|---|
| `low` | "Thấp" | `CircleCheck` | 1 px |
| `medium` | "Trung bình" | `TriangleAlert` | 1 px, chấm góc |
| `high` | "Cao" | `OctagonAlert` | 2 px |
| `critical` | "Nghiêm trọng" | `ShieldAlert` | 2 px đôi (`outline` + `border`) |
| `unknown` | "Chưa đánh giá" | `CircleHelp` | nét đứt |

Chữ không viết "an toàn". Tooltip ghi "rủi ro Thấp, đánh giá lúc `<assessedAt>`, dựa trên `<tool>`" (nghiên cứu mục 7).

**Token cần thêm (đề xuất, chưa có trong `main.css`).** Thêm vào cả `:root` và `.dark`, bind trong `@theme inline`, ghi vào STYLEGUIDE, theo mẫu `--status-success`:

| Token | Sáng / tối (tham chiếu biến Tailwind, không hex) |
|---|---|
| `--risk-low` | `var(--status-success)` |
| `--risk-medium` | `var(--color-amber-600)` / `var(--color-amber-400)` |
| `--risk-high` | `var(--color-orange-600)` / `var(--color-orange-400)` |
| `--risk-critical` | `var(--destructive)` |
| `--risk-*-background`, `--risk-*-border` | `color-mix(in srgb, var(--risk-*) 10%, transparent)` và `25%` |
| `--graph-edge-added`, `--graph-edge-removed` | `var(--status-success)`, `var(--destructive)` |

Phải phối hợp với CR-CV-050 mục 2.10 (đã đề xuất `--review-untested` hổ phách): hai nhóm token độc lập về nghĩa nhưng nên dùng cùng sắc để khỏi lệch. Chưa đo độ tương phản trên cả hai chủ đề; kiểm bằng công cụ trước khi gộp.

### 2.6 Bố cục tự động (phụ thuộc mới, cần duyệt)

`GraphCanvas` nhận `layout: LayoutEngine` (`(nodes, edges, options) => Promise<Positions>`), mặc định là bố cục tầng (wave) nội bộ như `buildDAGLayout`, để lens `flow`, `plan`, `execution` không cần thư viện. Lens `architecture`, `contract`, `data`, `impact` cần cụm và cạnh chéo nên cần thư viện.

| Tiêu chí | `elkjs` | `dagre` (`@dagrejs/dagre`) |
|---|---|---|
| Node lồng nhau (cụm, gom nhóm) | Hỗ trợ (compound graph, `hierarchyHandling`) | Hạn chế, phải tự xử lý |
| Định tuyến cạnh, cổng | Có | Chỉ điểm giữa |
| Kích thước bundle | Lớn (bản `elk.bundled`; đo ở bước 1 triển khai) | Nhỏ hơn nhiều (chưa đo) |
| Chạy trong Web Worker | Có (`elk-worker`) | Không sẵn, tự bọc |
| Giấy phép | EPL-2.0, cần pháp chế xem | MIT |
| Bảo trì | Có phát hành gần đây (chưa kiểm chứng ngày) | Bản gốc kém hoạt động, fork `@dagrejs` (chưa kiểm chứng) |
| Tính xác định | Có với cùng đầu vào | Có |

**Đề xuất:** `elkjs`, nạp lười trong Worker, chỉ khi lens cần cụm; dùng `dagre` làm phương án lùi nếu giấy phép EPL-2.0 hoặc kích thước không qua duyệt. Đây là **phụ thuộc mới, cần người duyệt** (AGENTS.md; README v7 O5). Bước 1 của triển khai là đo bundle và thời gian bố cục trên dữ liệu thật, rồi mới chốt.

### 2.7 Chuyển `TaskDAGView` sang token

Làm độc lập, có thể vào trước:

1. Thay `STATUS_COLORS` bằng ánh xạ trạng thái sang lớp token: `done` dùng `--status-success` (+ `-background`, `-border`); `in_progress` dùng `--primary`; `blocked` dùng `--destructive`; `review` dùng `--ai-action-accent` hoặc `--chart-3` (chọn khi duyệt); `cancelled`, `todo` dùng `--muted`/`--muted-foreground`/`--border`.
2. Bỏ `style={{}}` hex; node dùng class Tailwind `bg-*`/`border-*` hoặc `style` với `var(--…)`. Cạnh dùng `var(--border)`; chữ phụ `text-muted-foreground`.
3. Thêm `colorMode` của xyflow theo chủ đề hiện tại.
4. Giữ nguyên `data-testid`, `onConnect`, `addDependency`. Thêm kèm icon trạng thái trong node để không chỉ dựa màu.

Bước 2 (tuỳ chọn, sau): để `TaskDAGView` render qua `GraphCanvas` lens `plan` với `connectable`. Chưa cam kết vì `onConnect` ghi qua `task.addEdge`.

### 2.8 Hiệu năng và giới hạn

| Hạng mục | Đề xuất (chưa đo) |
|---|---|
| Node nạp tối đa | 2.000; vượt thì backend cắt, `truncated=true`, UI chuyển danh sách và hiện "Hiển thị 2.000/`totalNodes`" |
| Node vẽ cùng lúc | tối đa khoảng 50 sau gom; `onlyRenderVisibleElements` bật khi > 200 |
| Bố cục | trong Worker, không chặn luồng giao diện; kết quả cache theo `(digest, lens, groupOpen)` |
| Component node và tải | `memo` theo `id`, `risk`, `status`, `change`, chọn; `GraphCanvas`, `elkjs` nạp lười; cache tối đa 3 payload mỗi Request |

### 2.9 Trạng thái rỗng, đang tải, lỗi

| Tình huống | UI |
|---|---|
| Đang tải | `GraphSkeleton`; hoãn hiện 200 ms (độ trễ SSH), nút vô hiệu ngay |
| Chưa có đánh giá | "Chưa có đánh giá tác động cho phương án này" kèm nút "Chạy đánh giá" nếu có quyền (kênh do CR-REQ-030) |
| Lens không có node | "Không có `<loại>` bị ảnh hưởng" (không ghi "an toàn") |
| `stale=true` | Dải "Index phân tích đã cũ, kết quả chưa đánh giá được đầy đủ" và mọi `risk` hiện `unknown` hoặc tăng nhãn bất định |
| `truncated=true` | Dải "Chỉ hiện N/M node" và mặc định danh sách |
| Lỗi mạng, `forbidden`, `unsupported` | mạng: banner "Thử lại" giữ payload cũ mờ; `unsupported`: ẩn nút "Xem đồ thị", không lỗi đỏ (như CR-REQ-018); `forbidden`: "Bạn không có quyền xem đánh giá" |
| Thiếu dev server | "Chưa có dev server kết nối" kèm nút kết nối (nghiên cứu mục 7) |

### 2.10 SSH, độ trễ, chủ đề, màn hẹp

- Mọi gọi qua `callRequestRpc` (CR-REQ-018) với `getActiveRuntimeTarget`; không giả định chạy cục bộ. Bố cục chạy phía client để không thêm vòng khứ hồi.
- Chế độ sáng và tối: chỉ token, `colorMode` xyflow theo chủ đề; xyflow với `var()` trong SVG chưa kiểm chứng (cùng điểm với CR-CV-055).
- Màn hẹp và ứng dụng di động: mặc định chỉ danh sách; canvas chỉ để xem. Chưa kiểm tra.

### 2.11 Kênh dữ liệu (chờ CR-REQ-016 cập nhật)

Đề xuất `impact.graph {requestId, subjectType: 'solution_option'|'plan'|'task', subjectId, lens, base?}` trả `GraphPayload`. Tên cuối do CR-REQ-030 và CR-REQ-016 chốt; hook `useGraphLens` dùng hằng ở `request-rpc-methods.ts` để đổi một chỗ.

### 2.12 i18n

Tiền tố `auto.components.graph.` (lens, chuyển chế độ, trước/sau, chú giải, tìm kiếm, `RiskLevel.*`, trạng thái rỗng/lỗi), đủ `en, es, ja, ko, zh` và test phủ khoá.

## 3. Quyết định thiết kế

- Một canvas, lens là cấu hình; ba lens (`flow`, `plan`, `execution`) dựng phía client, bốn lens còn lại cần CR-REQ-030.
- `risk: 'unknown'` là giá trị riêng, không bao giờ vẽ như `low` (luật "không đo được thì ghi bất định").
- Danh sách là chế độ ngang hàng, không phải phụ lục.
- Bố cục qua giao diện `LayoutEngine` để v7 (CR-CV-055) giữ bố cục tự viết nếu không duyệt thư viện; `GraphCanvas` không bắt v7 dùng `elkjs`.
- Chuyển màu `TaskDAGView` tách riêng, vào trước, không phụ thuộc thư viện bố cục.

## 4. Tiêu chí chấp nhận

- [ ] `parseGraphPayload` chịu `kind`, `status` lạ và cạnh mồ côi; `risk` thiếu thành `unknown`.
- [ ] 7 chip lens; chip thiếu dữ liệu bị tắt kèm lý do; lens mặc định là lens rủi ro cao nhất.
- [ ] Đồ thị > 50 node hiện tối đa 50 và nhóm "+N" mở được; node `added`/`removed`/rủi ro cao luôn thuộc phần thấy.
- [ ] Ba mức thu phóng đổi cấp hiển thị (service, module, symbol).
- [ ] Bấm node làm mờ phần còn lại; `Esc` thoát.
- [ ] Công tắc Trước/Sau đổi hiển thị cạnh `added`/`removed` trên cùng một đồ thị, cạnh có nhãn "+" "−".
- [ ] Mọi đồ thị có chế độ danh sách cùng dữ liệu; > 100 hàng được ảo hoá; chọn đồng bộ hai chiều.
- [ ] `/` mở tìm kiếm `cmdk`, Enter nhảy tới node (mở nhóm chứa nó nếu cần).
- [ ] Mọi mức rủi ro có chữ và icon; ảnh chụp thang xám vẫn phân biệt được 5 mức.
- [ ] `main.css` có `--risk-*` ở `:root` và `.dark`, STYLEGUIDE cập nhật; không hex trong `components/graph/` và `TaskDAGView.tsx` (`rg '#[0-9a-fA-F]{3,6}'` rỗng).
- [ ] `TaskDAGView` giữ các `data-testid` và `onConnect`; test hiện có xanh.
- [ ] `unsupported` ẩn nút; `truncated`, `stale` có dải cảnh báo.
- [ ] Chuyển sáng/tối không cần tải lại; `prefers-reduced-motion` tắt hoạt ảnh.
- [ ] Bundle: `elkjs` chỉ nạp khi vào lens cần cụm (kiểm bằng phân tích bundle).

## 5. Kiểm thử

- Unit: `parseGraphPayload`, `graph-grouping` (ưu tiên node, cắt 50, nhóm "+N"), `graph-zoom-levels`, `graph-focus-state`, `riskPresentation` (5 mức đủ chữ, icon, token), ánh xạ trước/sau, lens registry.
- Component: `GraphPanel` (đổi lens, công tắc, danh sách đồng bộ), `GraphListView` (ảo hoá), `GraphSearchPalette`, `GraphNodeCard` (không chỉ màu), `GraphGroupNode`; mock `@xyflow/react` như `TaskDAGView.test.tsx`.
- Cập nhật `TaskDAGView.test.tsx` kiểm không còn style hex.
- Hiệu năng (chạy tay, ghi số đo): 50, 500, 2.000 node; thời gian bố cục `elkjs` và `dagre`; bundle trước và sau.
- Chưa chạy; tất cả là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Hiệu năng xyflow với hàng trăm node và repo cỡ nghìn symbol chưa thử.
- `elkjs`: kích thước, giấy phép EPL-2.0, chạy trong Worker của Electron/Vite chưa kiểm chứng. Trái README v7 O5; cần duyệt.
- Token rủi ro mới chưa đo tương phản trên cả hai chủ đề; chưa kiểm xyflow với `var()` trong SVG.
- Dữ liệu `impact` thiếu cạnh (tác động xuyên gRPC, WS, outbox có thể bị đánh giá thấp, nghiên cứu mục 9) khiến đồ thị trông "sạch" sai; do đó mới có `stale` và `unknown`.
- Ngưỡng 50 và 2.000 node, các mức zoom là đề xuất; tránh hai bộ token `--review-*` và `--risk-*` trôi lệch.

## 7. Câu hỏi mở

1. Chọn `elkjs` hay `dagre`; ai duyệt phụ thuộc và giấy phép EPL-2.0?
2. CR-REQ-030 trả `GraphPayload` trực tiếp hay một cấu trúc khác để frontend chuyển đổi? Tên kênh cuối?
3. Lens `execution` nhận sự kiện task theo kênh nào (CR-REQ-013, 016)?
4. `TaskDAGView` có nên chuyển hẳn sang `GraphCanvas` (bước 2) hay giữ riêng vì chức năng thêm phụ thuộc?
5. Hợp nhất `--review-*` (v7) và `--risk-*` hay giữ hai nhóm?

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-018 | Không bắt buộc sửa. Ghi `shared/graph-types.ts` là file mới của CR này; nếu CR-REQ-016 chốt kênh `impact.graph`, thêm hằng vào `request-rpc-methods.ts` |
| CR-REQ-019 | `RequestDetailHeader` thêm nút "Xem đồ thị" mở `GraphPanel` trong `Sheet`; `RequestStageTimeline` có thể là lens `flow` (giữ bản hiện tại, thêm liên kết) |
| CR-REQ-020 | `SolutionOptionCard` nhúng `GraphMini`; `SolutionComparisonTable` thêm hàng rủi ro (chi tiết ở CR-REQ-036) |
| CR-REQ-021 | `PlanSummaryHeader` thêm công tắc "Cây hoặc Đồ thị" (lens `plan`); mục 2.5 về lọc `plan`/`phase` giữ nguyên; `TaskDAGView` đổi màu theo 2.7 |
| CR-REQ-022, 023 | Không đổi |
| CR-REQ-016 (backend) | Chờ cập nhật: thêm kênh `impact.graph` |
| STYLEGUIDE, `main.css` | Thêm token `--risk-*`, `--graph-edge-*` |

## 9. Tham chiếu

- `/opt/repos/orca/docs/research/receive-request/frontend-visualization-and-ux.md`, `impact-assessment-and-risk-scoring.md`
- `/opt/repos/orca/frontend/src/renderer/src/components/task/{TaskDAGView,TaskGraph}.tsx`, `components/task/__tests__/TaskDAGView.test.tsx`, `components/workflow/DAGPreview.tsx`
- `/opt/repos/orca/frontend/package.json`; `components/ui/{command,table,toggle-group,sheet}.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/assets/main.css` (`:root` dòng 126, `.dark` dòng 216, `@theme inline` dòng 43)
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- `/opt/repos/orca/docs/crs/v7/README.md` (O5), `review-frontend/CR-CV-050` (mục 2.10), `CR-CV-055`
- Mới: `components/graph/*`, `shared/graph-types.ts`, `shared/graph-wire-parsers.ts`

# CR-CV-054 — Lens Cấu trúc: treemap SVG tự viết và cây thư mục ảo hoá

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-054 |
| **Tên** | Lens Cấu trúc: treemap SVG tự viết (thuật toán squarified, không thư viện mới) kèm cây thư mục ảo hoá (`@tanstack/react-virtual`), kích thước theo số symbol hoặc số dòng, màu theo khu vực hoặc ngôn ngữ, lớp phủ thay đổi, tải con theo yêu cầu |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050, 051, 053 (chú giải và mã hoá lớp phủ, điều hướng sang diff); backend: CR-CV-040, và RPC `GetStructure` (CR-CV-021/020; `ModuleGraph`) |
| **Mở khoá** | (độc lập; CR-CV-037 có thể thêm lớp phủ phụ thuộc/vòng ở lens này sau) |
| **Tác động** | `frontend/src/renderer/src/components/review-map/` (file mới), `review-lens-registry.ts` (thêm mục `structure`), `i18n/locales/*.json`; không đổi file hiện có ngoài đăng ký lens |

---

## 1. Bối cảnh và vấn đề

Cần một cách nhìn nhanh "thay đổi nằm ở đâu trong cây mã" mà không phải đọc danh sách file: treemap theo kích thước kèm cây thư mục để duyệt chính xác ([10 §5](../../../research/view-code/10-frontend-review-ux.md): "Treemap/sunburst bằng SVG tự viết (không có thư viện sẵn); cây thư mục ảo hoá (`react-virtual`); kích thước = số symbol; màu = khu vực"; [08 §2](../../../research/view-code/08-views-and-review-models.md)).

Hiện trạng đã đọc (2026-10-05):

- **Không có thư viện treemap** trong `frontend/package.json` (không `d3-*`, `elkjs`, `dagre`); README v7 O5 quyết **không thêm** thư viện ở MVP. Không có mã treemap nào trong `renderer/src` (đã rà).
- `@tanstack/react-virtual` 3.13 đã dùng ở `components/editor/CsvViewer.tsx` (`useVirtualizer({count, getScrollElement, estimateSize, overscan, getItemKey})`), `sidebar/WorktreeList.tsx`, `right-sidebar/SearchResultsPane.tsx`, `editor/CombinedDiffViewer.tsx`.
- **Dữ liệu**: `ModuleGraph {nodes: ModuleNode[], edges: ModuleEdge[]}` với `ModuleNode {id(path), kind:'folder'|'file', language, symbolCount, loc?, cluster}` (05 §2.2). Kênh `codeIntel.structure` (README 3.7). Quy mô tham chiếu: repo Orca có 15 759 file (README mục 1), nên **không thể** tải toàn bộ; mọi truy vấn phải có `path`/`depth` và giới hạn (README mục 6 "Không bao giờ trả cả graph", ≤ 1 500 nút).
- **Khu vực** (`frontend`, `backend-go`, `agent`, `desktop`, `mobile`) có trong 05 §2.1 là thuộc tính của cụm (`ClusterNode.area`), **không** có trong `ModuleNode`; phải suy từ tiền tố đường dẫn ở UI.
- Token màu danh mục `--review-area-1..6` do CR-CV-050 2.10 thêm; mã hoá lớp phủ và `overlaySvgProps(flags)` do CR-CV-053 2.5 cung cấp.
- Tiền lệ trợ năng: `CsvViewer` và `WorktreeList` không có mẫu cây `role="tree"`; cần viết theo WAI-ARIA Tree View.

## 2. Giải pháp đề xuất

### 2.1 Cây component và file (`components/review-map/`, mới)

```
StructureLens                       (ReviewLensProps)
├─ StructureToolbar                 (Kích thước: Symbol | Dòng · Màu: Khu vực | Ngôn ngữ · breadcrumb · ReviewOverlayLegend)
├─ ResizablePanelGroup (vertical)
│    ├─ StructureTreemap            (SVG; 2.3)
│    └─ StructureTree               (ảo hoá, role="tree"; 2.4)
├─ StructureEmptyState / StructureErrorState / StructureLoadingStage
└─ StructureFileDetail              (nội dung drawer khi chọn file; 2.5)
structure-treemap-layout.ts  structure-area-model.ts  structure-changed-index.ts
structure-tree-model.ts      useStructureTree.ts (hooks/)
```

```
┌ Kích thước: [Symbol|Dòng]   Màu: [Khu vực|Ngôn ngữ]   backend-go › services › infra-fleet-service ─┐
│ ┌──────────────────────────┬──────────┬────────┐                                                   │
│ │ internal/usecase  ●3     │ adapter  │ domain │   ● = có file đã đổi (số trong chấm)               │
│ │  (viền đậm = có đổi)     │          │        │   viền nét đứt = có symbol chưa test               │
│ ├────────────┬─────────────┴──────────┴────────┤                                                   │
│ │ proto      │ cmd                              │   chú giải: ▣ khu vực… ▣ đã đổi ┄ chưa test        │
│ └────────────┴──────────────────────────────────┘                                                   │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ▾ internal                                                              1 204 symbol  ●3          │
│   ▾ usecase                                                               312 symbol  ●3          │
│       ☐ relay.go                                                           41 symbol  ●            │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Dưới 560 px bề rộng, `ToggleGroup` "Treemap | Cây" chọn một trong hai thay vì chia dọc.

### 2.2 Dữ liệu, tải theo yêu cầu (`hooks/useStructureTree.ts`, `structure-tree-model.ts`)

- Gốc: `codeIntel.structure {path: '', depth: 2}`. Mở một thư mục chưa tải con (từ cây hoặc bấm vào ô treemap): `codeIntel.structure {path, depth: 1}`. Kết quả ghi vào `codeIntelResultsByWorktree` qua `useCodeIntelQuery`/`writeCodeIntelResult` (CR-CV-050), khoá `structure|<scopeKey>|<path>|<depth>`; bộ nhớ `Map<path, ModuleNode[]>` cục bộ của hook dựng cây.
- **Đồng thời tối đa 2 truy vấn**, loại trùng theo `path`, huỷ khi gỡ lens. Tổng nút đã tải bị chặn ở 5 000: vượt thì thôi tải thêm và hiện dòng "Đã đạt giới hạn hiển thị; thu hẹp bằng cách mở một thư mục con". Mỗi phản hồi `truncated` hiển thị "{shown}/{total} mục con" ngay ở thư mục đó (cuối nhóm con trong cây, và chấm "…" trong ô treemap), không cắt im lặng.
- **Giá trị** của thư mục: `ModuleNode.symbolCount` của chính nó nếu backend trả **tổng của cây con** (cần xác nhận, 7.1); nếu không, tổng các con đã tải (đánh dấu "≥" khi con chưa tải đủ). Chuyển đơn vị sang dòng dùng `loc` khi **mọi** nút hiển thị có `loc`; nếu thiếu, lựa chọn "Dòng" bị vô hiệu kèm tooltip "Index không có số dòng".
- Nút giá trị 0 không vẽ ô (vô nghĩa về diện tích) nhưng vẫn có trong cây.
- `ModuleEdge` (IMPORTS) **không dùng** ở lens này.
- Đổi phạm vi review không đổi cây (cấu trúc không phụ thuộc phạm vi), nhưng cache dùng `scopeKey` của index (`headCommit` của `IndexStatus`) chứ không của phạm vi review, để đổi phạm vi không tải lại.

### 2.3 Treemap SVG (`structure-treemap-layout.ts`, `StructureTreemap.tsx`)

- **Thuật toán squarified** (Bruls, Huizing, van Wijk) viết thuần: `layoutSquarifiedTreemap(items: {id: string; value: number}[], rect: {x,y,w,h}, {padding}) → {id, x, y, w, h}[]`. Sắp giảm dần theo `value`, xác định (khoá phụ theo `id` để ổn định giữa lần vẽ). Tổng diện tích bằng diện tích khung (sai số số học). Một mức mỗi lần: các con của thư mục đang xem; ô thư mục đủ lớn (≥ 120×80 px) vẽ thêm tiêu đề, **không** lồng đệ quy nhiều mức (đơn giản, đọc được).
- **Giới hạn**: tối đa 400 ô. Ô nhỏ hơn 24×16 px (hoặc ngoài top 400 theo giá trị) gộp thành một ô "+N nhỏ" (không drill; bấm mở cây tại thư mục). Khung đo bằng `ResizeObserver`, bố cục tính lại theo `requestAnimationFrame` khi đổi kích thước; kết quả `useMemo` theo `(children, rect, metric)`.
- **Màu**: `deriveStructureArea(path)` (`structure-area-model.ts`, thuần): đoạn đầu của đường dẫn; riêng `backend-go/services/<tên>` lấy `backend-go/<tên>` để các service phân biệt. Gán `--review-area-1..5` theo thứ tự chữ cái của các khu vực **đang hiển thị ở gốc** (xác định, không băm), mọi thứ còn lại `--review-area-6` ("Khác"). "Màu theo Ngôn ngữ": top 5 ngôn ngữ theo tổng giá trị, còn lại "Khác". Ô tô bằng `style={{fill: 'color-mix(in srgb, var(--review-area-N) 28%, var(--card))'}}`; viền `var(--border)`. Chú giải khu vực là hàng nhãn **chữ** kèm swatch (không chỉ màu); tên khu vực luôn có trong tooltip và trong tiêu đề ô đủ lớn.
- **Lớp phủ** (CR-CV-053 2.5): `buildChangedPathIndex(changedFiles, uncoveredFiles, violationFiles)` (`structure-changed-index.ts`, thuần) dựng đếm theo tiền tố đường dẫn; ô file có cờ `changed` dùng `overlaySvgProps(['changed'])`; ô thư mục có hậu duệ đã đổi hiển thị chấm và số "●3" ở tiêu đề, viền `changed` nhưng mảnh hơn; `untested` thêm nét đứt, `violation` thêm icon. Cờ `affected` **không áp dụng** (không có dữ liệu cấp file); chú giải chỉ liệt kê cờ có dữ liệu. File bị xoá trong thay đổi không có trong index nên không có ô; hiển thị dòng "{n} file bị xoá không hiển thị ở cấu trúc" khi `changedFiles` có trạng thái xoá.
- **Tương tác**: bấm ô thư mục = đi vào (breadcrumb cập nhật, cây mở và cuộn tới thư mục); bấm ô file = chọn (drawer 2.5); bấm "+N nhỏ" = mở cây tại thư mục. Breadcrumb là `button` từng cấp, quay lại cấp trên. `Tooltip` khi di chuột: đường dẫn, số symbol, số dòng, số file đã đổi. `Esc` trong khung treemap lên một cấp.
- **Trợ năng**: treemap là thành phần trực quan (`role="img"` có `aria-label` tóm tắt "Treemap của {path}: {n} mục, {m} có thay đổi"); **cây** ở 2.4 là đường truy cập bàn phím và trình đọc màn hình đầy đủ cho cùng dữ liệu. Không đặt 400 điểm dừng Tab.
- `prefers-reduced-motion`: không có chuyển tiếp khi đi vào/ra (mặc định không có hoạt ảnh).

### 2.4 Cây thư mục ảo hoá (`StructureTree`, `structure-tree-model.ts`)

- Hàng phẳng từ cây đã tải theo tập `expandedPaths` (`flattenStructureRows(tree, expanded)`, thuần); `useVirtualizer` với `estimateSize` 28 px, `overscan` 12, `getItemKey` = `path`. Hàng đặc biệt: "Đang tải…", "Tải lại" khi lỗi, "{shown}/{total}" khi `truncated`.
- Hàng: thụt lề theo cấp, chevron, icon `Folder`/`File` (`lucide-react`), tên (cắt giữa), `symbolCount` căn phải (`tabular-nums`), chấm đã đổi và số. Hàng chọn theo mẫu STYLEGUIDE "List rows": mặc định trong suốt, hover `bg-accent`, hàng đang chọn `bg-accent` + `data-current="true"`.
- **WAI-ARIA Tree**: `role="tree"`/`treeitem`, `aria-level`, `aria-expanded`, `aria-setsize`, `aria-posinset`, `aria-selected`, tiêu điểm lăn. Phím: `↑/↓` chuyển hàng, `→` mở (hoặc vào con đầu), `←` đóng (hoặc lên cha), `Home/End`, `Enter` chọn (thư mục: đi vào trong treemap; file: chọn). Không hỗ trợ phím `*` (mở mọi anh em) để tránh tải ồ ạt. Bỏ qua khi `isEditableTarget`.
- Hai chiều với treemap: `structureFocusPath` (state cục bộ `useReducer`) là nguồn duy nhất; đi vào ở treemap mở và cuộn cây tới đó; chọn trong cây đổi `structureFocusPath` thành thư mục cha của mục chọn.
- Lọc theo chip (CR-CV-051): chip `files`/`symbols` làm cây **chỉ hiển thị nhánh có file đã đổi** (nhánh khác ẩn) và ô treemap không đổi làm mờ (`opacity-40`); chip `untested` tương tự theo file chứa symbol chưa test. Bỏ lọc trả lại đầy đủ.

### 2.5 Chi tiết file (`StructureFileDetail`, nội dung drawer qua `ReviewDrawerContent`)

Khi chọn file: đường dẫn (sao chép), ngôn ngữ, `symbolCount`, `loc`, cờ lớp phủ, danh sách **symbol đã đổi** trong file (lọc `ChangeOverlay.changedSymbols` theo `filePath`; bấm để `onSelectSymbol` và chuyển sang lens Ảnh hưởng), nút "Xem diff" (`openReviewDiffAtSymbol` của CR-CV-053 không kèm dòng, hoặc dòng đầu của symbol) và "Mở trong editor". Không tải mã nguồn ở đây. Chọn thư mục: đi vào, không mở drawer.

### 2.6 Trạng thái và lỗi

| Tình huống | Hiển thị |
|---|---|
| Đang tải gốc | `StructureLoadingStage` theo thang thời lượng (CR-CV-051) |
| Rỗng | "Index chưa có thông tin cấu trúc cho đường dẫn này" + nút "Lên một cấp" |
| Mở thư mục lỗi | Hàng lỗi trong cây kèm "Thử lại"; treemap giữ nguyên |
| `truncated` | Dòng "{shown}/{total} mục con" ở thư mục đó |
| Vượt 5 000 nút | Dòng giải thích (2.2) |
| Offline | Theo khung (CR-CV-051): giữ phần đã tải, đánh dấu cũ |

## 3. Quyết định thiết kế

- **Tự viết squarified**, một mức, ≤ 400 ô: đủ đọc, không thêm phụ thuộc (O5), dễ kiểm thử (hàm thuần).
- **Cây là đường truy cập chính**, treemap là trực quan: tránh hàng trăm điểm dừng bàn phím.
- **Tải theo thư mục** (độ sâu 2 rồi 1): repo 15 759 file không tải trọn được.
- **Màu khu vực gán theo thứ tự chữ cái của khu vực đang hiển thị**, không băm, để ổn định và giải thích được; luôn có nhãn chữ.
- **Cờ `affected` không dùng** ở lens này vì không có dữ liệu cấp file.
- Đổi phạm vi review không tải lại cấu trúc (cache theo commit của index).

## 4. Tiêu chí chấp nhận

- [ ] `layoutSquarifiedTreemap`: tổng diện tích ô bằng diện tích khung (sai số < 0,5%), các ô không chồng nhau, kết quả giống nhau với cùng đầu vào, mục giá trị 0 bị loại.
- [ ] Treemap vẽ ≤ 400 ô, ô nhỏ gộp "+N nhỏ"; khung đổi kích thước thì bố cục tính lại không giật (một lần mỗi khung hình).
- [ ] Màu theo khu vực xác định và có nhãn chữ; "Màu theo Ngôn ngữ" hoạt động; không có hex trong file mới (kiểm tự động bằng test quét chuỗi).
- [ ] Lớp phủ: file đã đổi có viền đậm + dấu; thư mục có hậu duệ đổi có chấm và số đúng; `untested` nét đứt; chú giải chỉ liệt kê cờ có dữ liệu.
- [ ] Bấm ô thư mục đi vào, breadcrumb và cây đồng bộ; bấm file chọn và mở drawer chi tiết file; "Xem diff" mở diff của file.
- [ ] Cây ảo hoá: 10 000 hàng chỉ vẽ cửa sổ nhìn thấy; phím mũi tên/Home/End/Enter đúng chuẩn Tree; `aria-*` đúng; mở thư mục lần đầu gọi `structure {path, depth:1}` một lần dù bấm nhanh nhiều lần.
- [ ] Đồng thời ≤ 2 truy vấn; vượt 5 000 nút thì dừng tải và hiện thông báo; `truncated` hiển thị "{shown}/{total}".
- [ ] "Kích thước: Dòng" bị vô hiệu khi thiếu `loc`; đổi giữa Symbol/Dòng bố cục lại.
- [ ] Chip `files` làm cây chỉ còn nhánh có file đổi; bỏ chip trả lại đầy đủ.
- [ ] Lỗi mở thư mục không phá phần đã hiển thị; có "Thử lại".
- [ ] Hoạt động ở Electron và web; chuỗi qua `translate()` đủ 5 locale; `prefers-reduced-motion` không có chuyển tiếp; không `max-lines` disable.

## 5. Kiểm thử

- `structure-treemap-layout.test.ts`: tổng diện tích, không chồng, tính xác định, tỷ lệ cạnh trung bình tốt hơn chia dải đơn giản trên bộ giá trị mẫu, giá trị 0, một phần tử, `padding`, 5 000 phần tử không ném và chạy dưới ngưỡng thời gian rộng (ví dụ 200 ms; ngưỡng chỉ để bắt hồi quy lớn).
- `structure-area-model.test.ts`: `backend-go/services/x`, khu vực lạ, thứ tự gán màu, ngôn ngữ top 5.
- `structure-changed-index.test.ts`: đếm theo tiền tố, đường dẫn Windows/POSIX chuẩn hoá (`normalizeRuntimePathSeparators`), file xoá.
- `structure-tree-model.test.ts`: làm phẳng, mở/đóng, hàng tải/lỗi/`truncated`, lọc theo chip.
- `hooks/useStructureTree.test.ts` (đồng hồ giả, `vi.mock` client): loại trùng, đồng thời ≤ 2, giới hạn 5 000, huỷ khi gỡ.
- `StructureTree.test.tsx` (`// @vitest-environment happy-dom`): phím, `aria-*`, ảo hoá (kiểm số hàng DOM nhỏ hơn tổng), tiêu điểm lăn.
- `StructureTreemap.test.tsx`: số ô, nhãn khu vực, chuyển đổi Symbol/Dòng, bấm ô thư mục/file.
- `StructureLens.test.tsx`: tích hợp với client giả: tải gốc, mở thư mục, drawer chi tiết file; `i18n/code-intel-locale-coverage.test.ts` mở rộng.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **`ModuleNode.symbolCount` của thư mục có phải tổng cây con không**, và `codeIntel.structure` có hỗ trợ `path`/`depth` không: chưa có định nghĩa (README 3.6 chỉ ghi `GetStructure`); nếu backend chỉ trả cả cây, giới hạn 1 500 nút làm lens không dùng được cho repo lớn.
- `color-mix(in srgb, …)` trong thuộc tính `fill` SVG: dựa trên việc `main.css` và nhiều component đã dùng `color-mix` ở HTML; chưa kiểm chứng với phiên bản Chromium của Electron đóng gói.
- Hiệu năng bố cục khi kéo thay đổi kích thước panel chưa đo.
- Thứ tự khu vực theo chữ cái làm một khu vực đổi màu khi có khu vực mới ở gốc; chấp nhận (nhãn chữ vẫn có).
- Đường dẫn Windows/WSL/SSH trong `ModuleNode.id` (README 3.4: tương đối gốc repo) cần chuẩn hoá khi so với `changedFiles`.
- `loc` có thể thiếu hoặc chỉ cho một công cụ (05 ghi `loc?`).

## 7. Câu hỏi mở

1. `codeIntel.structure` hỗ trợ `path`, `depth` và trả `symbolCount` tổng cho thư mục? Nếu không, CR-CV-020/021 cần thêm.
2. Có `area` trong `ModuleNode` (để khỏi suy từ đường dẫn) không?
3. Sau này có thêm lớp phủ phụ thuộc/vòng (CR-CV-037) lên lens này không? Chỗ cắm: `StructureToolbar`.
4. Có cần sunburst (research nhắc "treemap/sunburst") hay chỉ treemap? Đề xuất chỉ treemap.
5. File bị xoá không có trong index: có cần lấy từ diff (`gitBranchChangesByWorktree`) để vẽ ô "đã xoá" không?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O5, 3.2, 3.9, mục 6), `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§5), `08-views-and-review-models.md` (§2), `05-graph-schemas.md` (§2.1, §2.2)
- `/opt/repos/orca/guides/STYLEGUIDE.md` (List rows: hover, selected, current; Color mixing; Animation)
- `/opt/repos/orca/frontend/src/renderer/src/components/editor/CsvViewer.tsx` (mẫu `useVirtualizer`), `components/sidebar/WorktreeList.tsx`, `components/right-sidebar/SearchResultsPane.tsx`
- `/opt/repos/orca/frontend/src/shared/cross-platform-path.ts`, `renderer/src/lib/editable-target.ts`, `components/ui/{resizable,toggle-group,tooltip,collapsible}.tsx`
- CR-CV-050 (token `--review-area-*`, cache, client), CR-CV-051 (khung, bộ lọc chip), CR-CV-053 (mã hoá lớp phủ, điều hướng diff)
- Mới: `components/review-map/{StructureLens,StructureTreemap,StructureTree,StructureToolbar,StructureFileDetail}.tsx`, `structure-treemap-layout.ts`, `structure-area-model.ts`, `structure-changed-index.ts`, `structure-tree-model.ts`, `hooks/useStructureTree.ts`

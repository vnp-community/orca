# CR-CV-088 — Nền đồ hoạ cho kiểm soát chất lượng: kiểm kê, quyết định thư viện/bố cục/engine, token màu, truy cập được, hiệu năng, bộ component biểu đồ nền

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-088 |
| **Tên** | Nền đồ hoạ: kiểm kê dependency và `components/ui`, bảng quyết định có căn cứ (tự viết SVG so với thư viện biểu đồ, bố cục đồ thị, engine đồ thị lớn), token màu cho mức nghiêm trọng, quy tắc mã hoá không chỉ dựa vào màu, mô tả văn bản và bàn phím cho mỗi hình, ngân sách hiệu năng, bộ component nền `components/quality-charts/` |
| **Loại** | Feature (nền tảng) + quyết định kiến trúc |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-050 (cơ chế token `--review-*`, i18n, cấu trúc `components/review-map/`), CR-CV-053 mục 2.5 (`OVERLAY_ENCODING`, mã hoá lớp phủ đã/ảnh hưởng/chưa test; dùng lại, không định nghĩa lại); phối hợp CR-CV-071 (phương pháp đo, ngân sách). Không cần backend: làm được bằng fixture |
| **Mở khoá** | CR-CV-087 (thực hiện **sau** CR này); các lens CR-CV-054, 055, 057 có thể dùng lại `treemap-squarified-layout.ts`, `ChartFrame`, `ChartTextAlternative` |
| **Tác động** | `frontend/src/renderer/src/assets/main.css` (token `--quality-*`, `--quality-heat-*`), `components/quality-charts/` (mới), `shared/chart-text-summary-types.ts` (nếu cần), `test-support/quality-chart-fixtures.ts` (mới), `i18n/locales/*.json`, `i18n/code-intel-quality-locale-coverage.test.ts` (mới); **không** thêm dependency (xem 2.2) |

---

## 1. Bối cảnh và vấn đề

Nghiên cứu [11 §5](../../../research/view-code/11-additions-for-quality-control.md) (D1, D2, D4, D5, D7) yêu cầu quyết định nền đồ hoạ trước khi vẽ scorecard, bản đồ nhiệt, DSM, xu hướng, treemap phủ test và đồng hồ diff coverage. README v7 chốt O5 (không thêm thư viện bố cục/treemap ở MVP) và O12 (công cụ/phụ thuộc mới cần duyệt riêng). CR này phải đưa ra **quyết định có căn cứ**, không mặc nhiên thêm hay mặc nhiên không thêm.

### 1.1 Kiểm kê dependency đồ hoạ (đã đọc 2026-10-06)

| Hạng mục | Kết quả | Bằng chứng |
|---|---|---|
| Đồ thị node-edge | `@xyflow/react` ^12.11.2 (cài 12.11.2, MIT); dùng ở `components/workflow/DAGPreview.tsx`, `components/task/TaskDAGView.tsx` | `frontend/package.json:28`; `node_modules/.pnpm/@xyflow+react@12.11.2*/…/package.json` |
| Prop hiệu năng/trợ năng của xyflow | `onlyRenderVisibleElements`, `nodesFocusable`, `disableKeyboardA11y`, `ariaLabelConfig` **có trong kiểu** (`dist/esm/types/component-props.d.ts:331, 355, 570, 646`). **Chưa có nơi nào trong `frontend/src` dùng** `onlyRenderVisibleElements` hay `nodesFocusable` (grep rỗng) | đọc `.d.ts`, grep |
| Sơ đồ | `mermaid` ^11.15.0, tải lười ở `components/editor/MermaidBlock.tsx` | `package.json:113` |
| Editor | `monaco-editor` ^0.55.1, `@monaco-editor/react` ^4.7.0 | `package.json:55, 114` |
| Ảo hoá | `@tanstack/react-virtual` ^3.13.24 (dùng ở `CsvViewer`, `WorktreeList`, `SearchResultsPane`, `CombinedDiffViewer`, `source-control-virtual-file-list`) | `package.json:59`, grep `useVirtualizer` |
| Bố cục panel | `react-resizable-panels` ^4.12.2 | `package.json:40` |
| Icon | `lucide-react` ^0.577.0 | `package.json:111` |
| Kiểu/validate | `zod` ~4.4.3, `yaml` ^2.8.4 | `package.json:47-48` |
| Thư viện biểu đồ trực tiếp | **Không có**: grep `recharts|d3|visx|echarts|chart|elk|dagre|cytoscape|sigma` trong `frontend/package.json` chỉ khớp `@xyflow`, `vitest`, v.v.; không dòng nào là thư viện biểu đồ | `frontend/package.json` |
| Gói **có sẵn gián tiếp** trong lockfile (qua `mermaid`/`@xyflow`) | `d3-hierarchy@3.1.2`, `d3-scale@4.0.2`, `d3-shape@3.2.0` (và `1.3.7`), `d3-array`, `d3-zoom`, `d3-selection`, `cytoscape@3.33.3`, `dagre-d3-es@7.0.14`. **Không có** `elkjs`, `recharts`, `visx`. Ba gói d3 đầu: `license: ISC`, `type: module` (đã đọc `package.json` trong `node_modules/.pnpm`). Chưa kiểm chứng gói nào kéo từng gói (chỉ biết có mặt) | `pnpm-lock.yaml:5868, 5902, 5913, 5943, 5800` |
| Công cụ a11y tự động | Không có `axe-core`/`jest-axe` trong `node_modules/.pnpm` hay `package.json` | grep |
| Kiểm thử | Vitest ^4.1.5 (môi trường `node`, thêm `// @vitest-environment happy-dom` cho component), `@testing-library/react` ^16, Playwright (có `tests/playwright.web.config.ts`) | `package.json` |

### 1.2 `components/ui/*` có gì, thiếu gì

Có (đã liệt kê thư mục): `accordion, badge, button-group, button, card, checkbox, collapsible, color-picker, command, context-menu, dialog, dropdown-menu, hover-card, input, label, popover, progress, resizable, scroll-area, select, separator, sheet, skeleton, slider, sonner, table, tabs, textarea, toggle-group, toggle, tooltip`.

**Thiếu** cho nhu cầu của series chất lượng: biểu đồ (chart), cây có `role="tree"`, lưới dữ liệu (data-grid; `table.tsx` chỉ là bảng tĩnh bọc `<table>`), timeline, khung thông báo (`alert`: không có file; các thông báo hiện là JSX tại chỗ), công tắc (`switch`), thẻ chỉ số (stat). `progress.tsx` có hai hạn chế đã đọc: (a) chỉ nhận `value`, không có trạng thái không xác định (`percent = null` của `quality.progress`, README v7 mục 8 điểm 6 và 3.10), (b) chỉ báo luôn có `transition-all duration-300 ease-out`, không có biến thể `motion-reduce`.

### 1.3 Token màu hiện có (`assets/main.css`) so với nhu cầu

Đã đọc `@theme inline` (:43-122), `:root` (:126-214), `.dark` (:216-302):

| Nhu cầu | Có sẵn | Đánh giá |
|---|---|---|
| Lỗi | `--destructive` (sáng `#e40014`, tối `#ff6568`) | Dùng được (vai trò "error states" theo STYLEGUIDE) |
| Đạt | `--status-success` (+`-background`, `-border`; sáng `#15803d`, tối `#86efac`) | Dùng được |
| **Cảnh báo** | **Không có token.** `--annotation-highlight` (`#f59e0b`/`#fbbf24`) là vùng đánh dấu ghi chú; STYLEGUIDE cấm dùng token trạng thái git cho việc khác. `main.css:1987-1989` dùng `var(--warning, #f59e0b)` nhưng `--warning` **không được định nghĩa ở đâu** (kiểm trong `:root`/`.dark`) nên luôn rơi về hex dự phòng | Thiếu |
| **Thông tin** | Không có | Thiếu |
| **Chưa rõ (unknown)** | `--muted-foreground` | Dùng được |
| Đã đổi / bị ảnh hưởng / chưa test / vi phạm | Chưa có; CR-CV-050 2.10 **đề xuất** `--review-changed|affected|untested|violation` và `--review-area-1..6` | Dùng lại, không tạo token trùng |
| Bảng màu biểu đồ | `--chart-1..5` đều là sắc xanh dương (`blue-300..800`), **giá trị giống nhau ở sáng và tối** (:158-162, :242-246); dùng ở `WorkspaceSpaceManagerPanel.tsx:88-93` và `assets/terminal.css:223` | Không phù hợp làm thang theo mức độ (không đổi theo chủ đề) |
| Thang cường độ (nhiệt) | Không có | Thiếu |

### 1.4 Đồ hoạ tương tự đã có trong repo (sibling, theo STYLEGUIDE UX rule 2)

- **Treemap có sẵn**: `components/status-bar/workspace-space-layout.ts` (`buildTreemapLayout`, chia đôi cân bằng, toạ độ theo phần trăm) và `WorkspaceSpaceManagerPanel.tsx:770-925` (ô là `<button>` định vị tuyệt đối, `aria-label` có tên và dung lượng, `title`, màu bằng `color-mix(in srgb, var(--chart-N) …, var(--card))`). CR-CV-054 mục 1 viết "Không có mã treemap nào trong `renderer/src` (đã rà)": **không đúng** (xem 7.1); thuật toán chia đôi cho tỉ lệ cạnh kém hơn squarified mà 054 chọn, nên không dùng lại làm nền chung, nhưng phải kế thừa cách đặt `aria-label` và `color-mix` của nó.
- **Vòng tiến độ SVG**: `components/setup-guide/SetupGuideProgressRing.tsx` (76 dòng, `aria-label` có "x/y", `Tooltip`) là mẫu cho gauge nhỏ.
- **`hooks/usePrefersReducedMotion.ts`** đã có (đọc `matchMedia('(prefers-reduced-motion: reduce)')` và lắng nghe thay đổi).
- Không có test chụp ảnh hay kiểm tương phản tự động trong repo.

### 1.5 Điều các CR cùng nhóm đã quyết (không lặp lại)

CR-CV-054: treemap squarified viết tay một mức ≤ 400 ô, `role="img"` + cây là đường truy cập. CR-CV-055/057: bố cục tầng viết tay, tất định. CR-CV-053 2.5: bảng mã hoá lớp phủ duy nhất. CR-CV-050 2.10: token `--review-*`. CR này **kế thừa** các quyết định đó và mở rộng cho các hình chất lượng.

## 2. Giải pháp đề xuất

### 2.1 Danh sách hình cần vẽ và độ phức tạp

Độ phức tạp: S = dưới 150 dòng logic, M = 150 đến 400, L = trên 400 hoặc có thuật toán không tầm thường. Chủ sở hữu hình: CR này làm **primitive**; CR-CV-087 ghép dữ liệu thật.

| # | Hình | Dữ liệu (nguồn ở CR-CV-087) | Cách vẽ | Độ phức tạp | Ngân sách phần tử |
|---|---|---|---|---|---|
| F1 | **Scorecard** cổng | `QualityGate` (+ `QualityRun`) | Không phải biểu đồ: thẻ `ui/card` + `GateVerdictBadge` + hàng lý do + `StackedSeverityBar` | S | ≤ 30 hàng lý do |
| F2 | Thanh chồng theo mức nghiêm trọng (`StackedSeverityBar`) | `QualityRun.summary {error,warning,info}` | HTML flex hoặc SVG, mỗi đoạn có số và biểu tượng | S | 3 đoạn |
| F3 | Sparkline xu hướng | chuỗi số theo lượt | SVG `<polyline>` + chấm cuối + nhãn số | S | ≤ 50 điểm |
| F4 | **Xu hướng theo lượt** (`TrendLineChart`) | `quality_trend_points` (qua CR-CV-087) | SVG: trục X là **lượt** (hạng mục rời, không phải thời gian liên tục), đường theo từng chuỗi, đánh dấu đổi kết luận cổng | M | ≤ 50 điểm × ≤ 4 chuỗi |
| F5 | **Gauge diff coverage** (`DiffCoverageGauge`) | `{covered, total}`, ngưỡng | Thanh "bullet" (giá trị + vạch ngưỡng) thay vì cung tròn (lý do ở bảng 2.2-D) | S | 1 |
| F6 | **Treemap phủ test/độ phức tạp** (`MetricTreemap`) | cây thư mục/file với `size` và `intensity` | Squarified, **một mức**, ≤ 400 ô; dùng `treemap-squarified-layout.ts` | M | ≤ 400 ô |
| F7 | **Bản đồ nhiệt hotspot** (`HotspotHeatmap`) | top N file × tín hiệu (churn, độ phức tạp, số phát hiện, thiếu phủ) | Lưới: hàng là file, cột là tín hiệu; ô tô theo thang 5 bậc kèm **số** | M | ≤ 40 hàng × ≤ 6 cột |
| F8 | **DSM** (`DependencyMatrix`) | đồ thị cụm/module rút gọn (CR-CV-087 nêu nguồn) | Lưới N×N, hàng/cột sắp theo thứ tự tô-pô của thành phần liên thông mạnh; ô lấp là SVG `<rect>`; ô nằm trên đường chéo dưới/trên cho biết vòng | L | ≤ 60×60 (ô lấp thường < 700) |
| F9 | Chú giải (`ChartLegend`) | bảng mã hoá | HTML | S | — |

Hai hình **không** thuộc CR này: đồ thị Ảnh hưởng/C4/ERD (CR-CV-053/055/057, xyflow) và treemap cấu trúc (CR-CV-054). Chúng dùng lại `ChartFrame`/`ChartTextAlternative` nếu muốn, không bắt buộc.

### 2.2 Bảng quyết định

**A. Thư viện biểu đồ: tự viết SVG hay thêm thư viện**

Tiêu chí (trọng số theo thứ tự ưu tiên): (1) kích thước bundle, (2) giấy phép, (3) hỗ trợ React 19, (4) chạy trong Electron renderer và web (SSR không bắt buộc: cả hai target đều là SPA, kiểm bằng grep `ReactDOMServer` ở `renderer/src` chưa làm), (5) truy cập được (bàn phím, `aria`, bảng thay thế), (6) tô màu bằng biến CSS/chủ đề, (7) chi phí duy trì.

| Phương án | Bundle | Giấy phép | React 19 | Truy cập được và theo chủ đề | Duy trì | Trạng thái bằng chứng |
|---|---|---|---|---|---|---|
| **A1. Tự viết SVG/HTML + hàm thuần** (đề xuất) | 0 dependency; mã của ta (ước tính vài trăm dòng logic tổng cộng, **chưa đo**) | n/a | n/a (React thuần) | Toàn quyền: `aria`, `color-mix`, `var(--…)` | Ta tự giữ (toạ độ, tick, ghi nhãn) | Tiền lệ trong repo: `SetupGuideProgressRing`, treemap `WorkspaceSpaceManagerPanel`; hình ở 2.1 đều là hình học đơn giản |
| **A2. Chỉ module toán d3** (`d3-scale`, `d3-shape`, `d3-hierarchy`) làm dependency trực tiếp, vẽ vẫn bằng React | Có sẵn trong lockfile (xem 1.1) nên không thêm gói lạ; bundle sau tree-shake **chưa đo** (ESM, `module` field có) | ISC (đã đọc) | Không liên quan: không phụ thuộc React | Như A1 | Giảm mã tự viết cho thang đo, tick đẹp, nội suy đường, treemap | Phải xin duyệt O12 vì khai báo dependency trực tiếp |
| **A3. Thư viện biểu đồ tích hợp React** (ứng viên: `recharts`, `visx`, `echarts`, `nivo`) | **Chưa kiểm chứng**; không có trong lockfile (grep rỗng cho `recharts`, `visx`; `echarts` chưa grep riêng) | **Chưa kiểm chứng** từng gói | **Chưa kiểm chứng** từng phiên bản | Phải kiểm tra từng gói: `aria`, bảng thay thế, tô bằng `var()` | Phụ thuộc bên ngoài, nâng cấp React 19+ | Chưa có bằng chứng; không đánh giá tiếp trừ khi A1/A2 bị loại |

*Khuyến nghị (theo O5, O12):* **A1 cho toàn bộ MVP**, **không thêm dependency**. Lý do: mọi hình ở 2.1 là hình học đơn giản, các nhu cầu "khó" (tick đẹp, nội suy) không có ở hình nào cần (trục X là hạng mục rời; một thang tuyến tính; đường gấp khúc, không cong). **Điều kiện xem lại sang A2** (ghi vào PR khi xảy ra, không làm trước): (a) hàm thang/tick tự viết vượt ~120 dòng hoặc có lỗi tick lặp lại, (b) một hình cần trục thời gian liên tục hoặc nội suy cong, (c) CR-CV-054 đã chọn squarified tự viết và tự viết lại thành hai bản, tức lặp mã. Khi đó đề nghị duyệt riêng **chỉ** `d3-scale` + `d3-shape` (+ `d3-hierarchy` nếu thay squarified). A3 chỉ xem xét nếu có yêu cầu tương tác (brush, zoom) mà A1/A2 không đáp ứng; khi đó phải làm bảng bằng chứng thật cho từng ứng viên trước.

**B. Bố cục tự động cho đồ thị**

CR này không có hình node-edge cần bố cục; mục này chốt ngưỡng cho các lens khác và cho DSM.

| Số nút | Cách trình bày | Ghi chú |
|---|---|---|
| ≤ ~100 | Bố cục tầng viết tay (CR-CV-055/057) | Ngưỡng của nghiên cứu 11 D2; **chưa đo** trên dữ liệu Orca |
| ~100 đến ~300 | Bố cục tầng + chế độ thu gọn ("chỉ phần bị đổi + liền kề" như CR-CV-057 2.6) | Giảm nút hiển thị thay vì đổi thuật toán |
| trên ~300 (tới trần 1 500 của README mục 6) | **Danh sách** hoặc **DSM** (ma trận không có "cắt nhau") | DSM là cách trình bày chính của CR này cho phụ thuộc dày |
| `elkjs`, `dagre` | **Không thêm** ở MVP | `elkjs` không có trong lockfile; `dagre` thuần không có (chỉ có `dagre-d3-es@7.0.14` là phụ thuộc nội bộ của `mermaid`, không phải API ổn định của ta). Giấy phép, kích thước, tương thích Electron/web của hai gói: **chưa kiểm chứng** |

*Thí nghiệm quyết định sau (không thuộc CR này):* chạy bố cục tầng của 055/057 trên 50, 100, 300 nút thật của Orca, đo thời gian bố cục và số cạnh cắt nhau; đề nghị duyệt `elkjs`/`dagre` chỉ khi vượt ngưỡng đã ghi ở 2.6 (xem 7.3).

**C. Engine đồ thị lớn**

| Phương án | Khi nào | Bằng chứng |
|---|---|---|
| **C1. xyflow + `onlyRenderVisibleElements` + danh sách thay thế** (đề xuất, đang dùng ở CR-CV-053) | ≤ 1 500 nút (trần hợp đồng, README mục 6) | Prop tồn tại trong kiểu (1.1); **hiệu năng thật chưa đo**, và chưa có đoạn mã nào trong repo bật nó |
| C2. Canvas/WebGL (`cytoscape` đã có trong lockfile qua `mermaid`; `sigma` không có) | Chỉ khi C1 vượt ngân sách 2.6 ở 1 500 nút thật | Chưa kiểm chứng; sẽ mất `aria` theo nút, cần cây thay thế bắt buộc |

*Khuyến nghị:* C1; C2 cần duyệt O12 và thí nghiệm đo bằng cùng harness của 2.6.

**D. Gauge**: nghiên cứu D3 nhắc "đồng hồ"; chọn **bullet bar** (thanh + vạch ngưỡng + số) thay cung tròn vì (i) đọc giá trị chính xác hơn, (ii) vạch ngưỡng là yêu cầu của cổng, (iii) dễ gán `role="meter"` hoặc `role="img"`, (iv) cùng vòng tiến độ nhỏ (`SetupGuideProgressRing`) vẫn dùng cho chip.

### 2.3 Token màu (sửa `assets/main.css`)

Quy tắc STYLEGUIDE: thêm vào **cả** `:root` và `.dark`, bind trong `@theme inline`, không hex mới. Giá trị tham chiếu biến Tailwind như `--ai-action-accent: var(--color-violet-500)` đã làm (`main.css:154, 238`); `tailwindcss@4.2.4/theme.css` có đủ các biến dùng bên dưới (đã đọc).

**Mức nghiêm trọng và kết luận** (đặt sau `--annotation-highlight`):

| Token | Vai trò | Sáng | Tối |
|---|---|---|---|
| `--quality-error` | `severity:error`, `verdict:fail` | `var(--destructive)` | `var(--destructive)` |
| `--quality-warning` | `severity:warning`, `verdict:warn` | `var(--color-amber-700)` | `var(--color-amber-400)` |
| `--quality-info` | `severity:info` | `var(--color-sky-700)` | `var(--color-sky-400)` |
| `--quality-pass` | `verdict:pass`, `result:pass` | `var(--status-success)` | `var(--status-success)` |
| `--quality-unknown` | `verdict:unknown`, chưa chạy | `var(--muted-foreground)` | `var(--muted-foreground)` |
| `--quality-{error,warning,info,pass}-background` | nền nhạt | `color-mix(in srgb, var(--quality-X) 10%, transparent)` | như sáng |
| `--quality-{error,warning,info,pass}-border` | viền | `color-mix(in srgb, var(--quality-X) 25%, transparent)` | như sáng |

Bind: `--color-quality-error: var(--quality-error)` … theo mẫu `--color-status-success*` (`main.css:101-103`) để dùng lớp `text-quality-error`, `bg-quality-error-background`, `border-quality-error-border`. Đặt tên theo vai trò ngữ nghĩa, không theo màu.

**Thang cường độ** (bản đồ nhiệt, treemap, DSM; **không dùng hue mới**, tránh va chạm với cảnh báo/untested): `--quality-heat-1..5 = color-mix(in srgb, var(--foreground) {8, 22, 40, 62, 85}%, var(--card))`. Mẫu `color-mix` theo `foreground` đã có ở `main.css` (nhiều chỗ, ví dụ :3153-3155) và STYLEGUIDE "Color mixing". Quy ước đọc **thống nhất**: *đậm hơn = cần chú ý hơn* (hotspot: điểm cao; phủ test: tỷ lệ chưa phủ cao; phụ thuộc: số cạnh nhiều). Chữ trên ô: bậc 1 đến 3 dùng `--foreground`, bậc 4 đến 5 dùng `--background` (số đo bên dưới).

**Lớp phủ** (đã đổi, bị ảnh hưởng, chưa test, vi phạm): dùng nguyên `--review-*` của CR-CV-050 và `OVERLAY_ENCODING` của CR-CV-053; CR này **không** thêm token.

**Tương phản (tính tay bằng script ngày 2026-10-06 theo công thức WCAG 2.x từ giá trị `oklch` trong `theme.css` và hex trong `main.css`; chưa đối chiếu công cụ ngoài, cần kiểm lại bằng test ở mục 5)**:

| Token | Sáng/`card` `#fff` | Sáng/`muted` `#f5f5f5` | Tối/`card` `#171717` | Tối/`background` `#0a0a0a` | Tối/`muted` `#262626` |
|---|---|---|---|---|---|
| `--quality-error` | 4,87 | **4,47** | 6,24 | 6,89 | 5,27 |
| `--quality-warning` | 5,05 | 4,63 | 10,44 | 11,53 | 8,81 |
| `--quality-info` | 5,85 | 5,37 | 8,21 | 9,07 | 6,93 |
| `--quality-pass` | 5,02 | 4,60 | 12,77 | 14,10 | 10,78 |
| `--quality-unknown` | 4,74 | **4,35** | 6,94 | 7,66 | 5,86 |

Ngưỡng áp dụng: **chữ** màu mức nghiêm trọng chỉ đặt trên `card`/`background` (≥ 4,5:1); trên `muted` hoặc nền nhạt (tint) thì chữ dùng `--foreground`/`--muted-foreground`, màu mức nghiêm trọng chỉ cho biểu tượng, viền, hình (≥ 3:1). Hai ô in đậm (<4,5 trên `muted`) là lý do của quy tắc này. Với `--review-*` của CR-CV-050 (đã tính cùng cách): `--review-untested` sáng (`amber-600`) chỉ **3,19** trên `card` và **2,93** trên `muted` (dưới 3:1 cho đồ hoạ trên `muted`); `--review-affected` sáng (`teal-600`) 3,66/3,36. Đề nghị CR-CV-050/053 đổi `--review-untested` sáng sang `amber-700` hoặc chỉ dùng nó trên `card`; ghi ở 7.2.

Thang cường độ (chữ trên ô, `color-mix` trong không gian sRGB như CSS): sáng, chữ `foreground` trên bậc 1/2/3 = 16,67 / 11,97 / 7,30; chữ `background` trên bậc 4/5 = 5,65 / 13,44. Tối, chữ `foreground` trên bậc 1/2/3 = 13,91 / 8,63 / 4,62; chữ `background` trên bậc 4/5 = 7,92 / 13,88. Tất cả ≥ 4,5.

**Cách kiểm (bắt buộc, thành test):** `quality-token-contrast.test.ts` đọc `main.css` và `tailwindcss/theme.css`, giải biến (`var()`, `color-mix` srgb, `oklch` → sRGB), tính tỉ lệ và khẳng định bảng cặp (token, nền) với ngưỡng 4,5 (chữ) hoặc 3 (đồ hoạ). Đây là hàm thuần (~60 dòng), không cần công cụ mới. Kiểm tay bổ sung ở sáng và tối cho `Monaco` ở CR-CV-087.

### 2.4 Mã hoá không chỉ dựa vào màu

Bảng nguồn duy nhất `severity-encoding.ts` (mẫu `OVERLAY_ENCODING` ở CR-CV-053 2.5; chú giải **dựng từ chính bảng**):

| Giá trị | Nhãn (i18n) | Token | Biểu tượng `lucide-react` | Hình SVG trong biểu đồ | Nét |
|---|---|---|---|---|---|
| `error` | Lỗi | `--quality-error` | `OctagonX` | bát giác đặc | nét liền 2 px |
| `warning` | Cảnh báo | `--quality-warning` | `TriangleAlert` | tam giác | nét gạch `4 2` |
| `info` | Thông tin | `--quality-info` | `Info` | tròn rỗng | nét chấm `1 2` |
| `pass` | Đạt | `--quality-pass` | `CircleCheck` | tròn có dấu tick | liền 1 px |
| `unknown` | Chưa rõ | `--quality-unknown` | `CircleHelp` | tròn nét đứt dài | `2 3` |

Ba nguyên tắc: (1) mỗi điểm dữ liệu quan trọng có **nhãn chữ hoặc số** (không chỉ hình/màu); (2) `unknown` luôn có biểu tượng và chữ "Chưa rõ", không bao giờ dùng biểu tượng "đạt" (STYLEGUIDE "UI copy must not overclaim"); (3) thang cường độ luôn kèm **số** trong ô hoặc trong tooltip và trong bảng thay thế. Va chạm sắc cam/hổ phách giữa `--quality-warning` (tam giác) và `--review-untested` (viền đứt) được giải bằng **dạng mã hoá khác nhau**: một là biểu tượng ở góc, một là kiểu viền của nút; không bao giờ để một dấu hiệu duy nhất mang hai nghĩa.

### 2.5 Truy cập được

- **Mô tả văn bản cho mỗi hình** (`chart-text-summary.ts`, hàm thuần, chuỗi qua `translate()`): `ChartFrame` bắt buộc nhận `summary` (một câu, thành `aria-label` của `role="img"`/`figure`), ví dụ "Số phát hiện qua 5 lượt: lỗi từ 12 xuống 3, cảnh báo từ 8 xuống 8". Câu chỉ nêu số đo, **không** kết luận ("cải thiện", "an toàn") vì thang đo không đủ để kết luận.
- **Bảng thay thế** (`ChartTextAlternative`): nút "Xem dạng bảng" (`ui/toggle`), hiển thị cùng dữ liệu bằng `ui/table`, có `<caption>`, tiêu đề cột, đơn vị. Đây là **đường truy cập chuẩn** cho trình đọc màn hình và bàn phím; khi chưa bật, bảng nằm trong lớp `sr-only` (không dùng `display:none`) để trình đọc màn hình vẫn tới được; bật nút thì hiển thị thật thay cho hình.
- **Bàn phím** (`useChartKeyboardNavigation.ts`): hình có ô tương tác (F6 treemap, F7 heatmap, F8 DSM) là **một điểm dừng Tab** dùng mẫu APG "grid" (`role="grid"`, `role="gridcell"`, tiêu điểm lăn `tabindex`): `←/→/↑/↓` chuyển ô, `Home/End` đầu/cuối hàng, `Ctrl+Home/End` đầu/cuối lưới (nhãn phím theo nền tảng khi hiển thị; không hiển thị chip nếu chưa cài), `Enter` kích hoạt (chọn/mở), `Esc` thoát về khung. Không đặt hàng trăm điểm dừng Tab (đã quyết ở CR-CV-054). F3/F4/F5 là `role="img"` + bảng thay thế (không tương tác từng điểm ở MVP).
- **Tiêu điểm nhìn thấy**: `focus-visible:ring-2 ring-ring` như `WorkspaceSpaceManagerPanel.tsx` ô treemap.
- **`prefers-reduced-motion`**: không hoạt ảnh vẽ vào/chuyển; `usePrefersReducedMotion` (đã có) điều khiển; hai nhược điểm của `progress.tsx` (1.2) xử lý ở CR-CV-087 bằng lớp `motion-reduce` và trạng thái không xác định, **không** sửa `progress.tsx` ở CR này.
- Tooltip hover chỉ là phụ; mọi thông tin trong tooltip cũng nằm trong bảng thay thế.
- xyflow (đồ thị khác): bật `nodesFocusable` và cung cấp `ariaLabelConfig` (kiểu có sẵn, 1.1) là việc của CR-CV-053/055/057; CR này chỉ ghi nhận.

### 2.6 Hiệu năng

**Ngân sách (đề xuất, chưa có số đo; hiệu chỉnh một lần sau đo đầu tiên theo mẫu CR-CV-071 D1):**

| Hạng mục | Ngân sách | Cách đảm bảo |
|---|---|---|
| Số phần tử SVG mỗi hình | F6 ≤ 400 ô; F7 ≤ 240 ô; F8 ≤ 700 ô lấp (lưới nền là một `<path>`/`pattern`); F4 ≤ 200 | Cắt và gộp "+N" kèm thông báo; không cắt im lặng |
| Thời gian tính bố cục (hàm thuần) | squarified 400 ô ≤ 16 ms; thứ tự DSM 150 nút ≤ 50 ms trên máy phát triển | Test thời gian rộng để bắt hồi quy lớn (mẫu CR-CV-054 mục 5) |
| Vẽ hình lần đầu sau khi có dữ liệu | ≤ 100 ms ở F1–F5, ≤ 250 ms ở F6–F8 (không tính mạng) | `performance.measure` trong test e2e web |
| Bộ nhớ | Dữ liệu một hình ≤ 1 MB trong store (ước tính, ví dụ 5 000 phát hiện × ~200 B ≈ 1 MB, chưa đo) | Giới hạn nạp ở CR-CV-087 |
| Pan/zoom đồ thị xyflow (C1) | ≥ 30 fps ở 1 500 nút | Chỉ đo, không thuộc hình của CR này; ngưỡng kích hoạt thí nghiệm C2 |
| Không kéo thư viện nặng vào chunk khởi động | `quality-charts` tải lười cùng lens (đăng ký lens của CR-CV-051 dùng `React.lazy`) | Không import `quality-charts` từ module khởi động |

**Kỹ thuật:** (1) **ảo hoá** không áp dụng cho SVG nhỏ; áp dụng cho danh sách phát hiện (CR-CV-087) bằng `@tanstack/react-virtual`; (2) **vẽ lười**: `useLazyChartMount` (IntersectionObserver) hoãn F7/F8/F6 tới khi cuộn vào tầm nhìn, giữ chỗ bằng `minHeight` của `ChartFrame` để không nhảy bố cục (STYLEGUIDE "pre-reserve space"); (3) `memo` và `useMemo` theo dữ liệu; (4) đo kích thước bằng `ResizeObserver` (`useChartSize`), gộp theo `requestAnimationFrame`; (5) không tạo `ResizeObserver` cho mỗi ô.

**Đo (phối hợp CR-CV-071):** (a) test thời gian cho hàm thuần (Vitest, ngưỡng rộng như CR-CV-054); (b) một spec Playwright web (`tests/playwright.web.config.ts`) nạp fixture cỡ Orca qua `code-intel-fake-backend.ts`, dùng `performance.mark/measure` đặt trong `ChartFrame` (chỉ khi `import.meta.env.DEV` hoặc cờ test) và xuất số đo; (c) số đo thứ nhất ghi vào PR đầu theo mẫu "cột đo được" của CR-CV-071 2.1. CR-CV-071 hiện chỉ nêu ngân sách phía backend (độ trễ RPC, kích thước payload, RSS agent): ngân sách vẽ phía frontend ở bảng trên **chưa có chỗ chứa**; đề nghị thêm khoá `frontend` vào `codeintel-budgets.json` hoặc một tệp dữ liệu riêng (7.4). Độ trễ SSH 50–200 ms: các hình không đo vì dữ liệu đã nằm trong store; trạng thái tải của dữ liệu theo `usePerceivedLoadingStage` (CR-CV-051).

### 2.7 Component nền (`frontend/src/renderer/src/components/quality-charts/`, mới)

Tên theo khái niệm, không `helpers/utils/common`; không `max-lines` disable.

```
components/quality-charts/
├─ ChartFrame.tsx                  khung: tiêu đề, mô tả, trạng thái, chú giải, bảng thay thế, giữ chỗ
├─ ChartLegend.tsx                 dựng từ bảng mã hoá
├─ ChartTextAlternative.tsx        bảng dữ liệu thay thế (ui/table)
├─ ChartHoverCard.tsx              thẻ hover định vị bằng toạ độ khung (không nuốt tiêu điểm)
├─ SeverityBadge.tsx · SeverityGlyph.tsx (SVG) · GateVerdictBadge.tsx
├─ StackedSeverityBar.tsx · SparklineChart.tsx · TrendLineChart.tsx
├─ DiffCoverageGauge.tsx · MetricTreemap.tsx · HotspotHeatmap.tsx · DependencyMatrix.tsx
├─ severity-encoding.ts            bảng mã hoá (nguồn duy nhất, 2.4)
├─ heat-intensity-scale.ts         giá trị -> bậc 1..5 (phân vị cố định, thuần)
├─ chart-linear-scale.ts           thang tuyến tính + tick (thuần)
├─ treemap-squarified-layout.ts    squarified một mức (thuần; chủ sở hữu, xem 7.1)
├─ dependency-matrix-ordering.ts   thành phần liên thông mạnh (Tarjan) + thứ tự (thuần)
├─ chart-text-summary.ts           câu mô tả (thuần, i18n)
├─ useChartSize.ts · useLazyChartMount.ts · useChartKeyboardNavigation.ts
└─ __tests__/ ...
test-support/quality-chart-fixtures.ts   dữ liệu tổng hợp cỡ Orca (trần ngân sách)
```

Props chính:

```ts
type ChartTableData = { caption: string
  columns: { key: string; label: string; align?: 'start' | 'end' }[]
  rows: Record<string, string | number>[] }

type ChartFrameProps = {
  id: string                     // aria id, nhãn đo hiệu năng
  title: string
  description?: string           // cách đọc: "Trục dọc: ..."
  summary: string                // aria-label, từ chart-text-summary
  status: 'loading' | 'ready' | 'empty' | 'error' | 'stale'
  emptyReason?: React.ReactNode  // "Chưa có dữ liệu vì ..." (không "không có vấn đề")
  error?: { message: string; onRetry?: () => void }
  staleNote?: string             // dòng "Số liệu tại {commit}..."
  legend?: React.ReactNode
  table: ChartTableData          // bắt buộc: đường truy cập chuẩn
  minHeight: number              // giữ chỗ
  children: (size: { width: number; height: number }) => React.ReactNode
}
type SeverityBadgeProps = { severity: QualitySeverityOrUnknown; count?: number; withLabel?: boolean }
type MetricTreemapProps = { items: { id: string; label: string; size: number; intensity: number;
  overlay?: OverlayFlag[] }[]; sizeLabel: string; intensityLabel: string
  onSelect?: (id: string) => void; maxTiles?: number /* 400 */ }
type HotspotHeatmapProps = { rows: { id: string; label: string; values: (number | null)[] }[]
  columns: { key: string; label: string; unit?: string }[]; onSelectRow?: (id: string) => void }
type DependencyMatrixProps = { nodes: { id: string; label: string; group?: string }[]
  edges: { from: string; to: string; weight: number }[]; maxNodes?: number /* 60 */
  onSelectCell?: (from: string, to: string) => void }
type DiffCoverageGaugeProps = { covered: number; total: number; threshold?: number
  estimated?: boolean /* hiển thị "ước lượng" */ }
```

Wireframe ChartFrame và hai hình:

```
┌ Số phát hiện theo lượt ──────────────────── [Xem dạng bảng] ┐
│ Trục ngang: lượt agent (cũ → mới). Trục dọc: số phát hiện.    │
│   12 ■──┐                                                     │
│          ■──┐    ▲ cảnh báo (tam giác)   ■ lỗi (bát giác)     │
│   8  ▲──▲──▲──▲   ● thông tin (tròn)                          │
│   3             ■                                              │
│      L1  L2  L3  L4                                            │
│ Chú giải: ■ Lỗi  ▲ Cảnh báo  ● Thông tin   ⋯ Kết luận đổi     │
└ Dữ liệu theo lần chạy lúc 14:02 · HEAD a41c9e0 ──────────────┘

DSM (hàng = bị phụ thuộc bởi cột? quy ước ghi trong mô tả)
        A  B  C  D     ■ có phụ thuộc (đậm = nhiều cạnh)
     A  ·  ■  ·  ·     ▲ ô trên đường chéo = phụ thuộc ngược chiều tầng (vòng)
     B  ·  ·  ■  ▲
     C  ·  ·  ·  ■
     D  ·  ·  ·  ·
```

Quy ước đọc DSM (ghi vào `description`): hàng là nút nguồn, cột là nút đích; thứ tự theo thứ tự tô-pô của đồ thị thành phần liên thông mạnh, các nút trong cùng thành phần liên thông (vòng) gộp thành khối và được vẽ khung đậm. Ô phía dưới đường chéo (cạnh đi ngược thứ tự tô-pô) chỉ xảy ra trong khối vòng; dấu `▲` ở đó là ký hiệu hình dạng (không chỉ màu).

### 2.8 Trạng thái và lỗi chung của hình

| Trạng thái | Hiển thị |
|---|---|
| `loading` | `Skeleton` đúng kích thước `minHeight`; thang thời lượng do hook gọi (CR-CV-051) |
| `empty` | Giải thích **vì sao** trống ("Chưa có lần chạy nào", "Profile không thu thập coverage"), có hành động trực tiếp nếu có; **không** viết "không có vấn đề" |
| `error` | Persistent inline, nút "Thử lại"; không toast |
| `stale` | Hình vẫn vẽ + dòng `staleNote` (commit, thời điểm) |
| Cắt (`maxTiles`, `maxNodes`) | "Đang hiển thị X/Y; thu hẹp phạm vi" |

### 2.9 i18n, hai render target, SSH

Khoá `auto.components.qualityCharts.<Thành phần>.<tên>` đủ `en, es, ja, ko, zh`; test `i18n/code-intel-quality-locale-coverage.test.ts` (mẫu `task-jira-link-locale-coverage.test.ts`); không gọi `translate()` ở cấp module (`i18n/no-top-level-translate.test.ts`). Component không gọi mạng và không dùng API riêng của Electron nên chạy ở web lẫn Electron. Không có hành động từ xa trong CR này.

## 3. Quyết định thiết kế

- **Tự viết SVG, không thêm dependency** (A1) với điều kiện xem lại tường minh sang A2; ghi vào đây để người duyệt O12 thấy bảng so sánh và có thể quyết khác.
- **Một quy ước nhiệt duy nhất** (đậm = cần chú ý hơn), không hue mới: tránh va chạm với `--quality-warning` và `--review-untested` và an toàn cho người khiếm sắc.
- **Bảng thay thế bắt buộc** cho mọi hình (prop `table` không tuỳ chọn): trợ năng không phải việc làm sau.
- **Mô tả văn bản chỉ nêu số đo**, không kết luận.
- **Mã hoá từ bảng nguồn duy nhất** (`severity-encoding.ts`), chú giải dựng từ cùng bảng.
- **Bullet thay cung tròn** cho diff coverage.
- **Thang tuyến tính, hạng mục rời cho trục lượt**: bỏ nhu cầu tick thời gian và nội suy cong.
- **`treemap-squarified-layout.ts` đặt ở đây** vì CR này làm trước 054 trong thứ tự P0 của nhóm chất lượng; 054 nhập lại thay vì viết bản thứ hai (7.1).
- **Không sửa `progress.tsx`, `main.css` ngoài token mới** ở CR này; sửa `progress.tsx` nếu cần thì ở CR-CV-087.

## 4. Tiêu chí chấp nhận

- [ ] Không có dependency mới trong `frontend/package.json` và `pnpm-lock.yaml` (diff chỉ chạm mã nguồn và token), trừ khi người duyệt O12 chọn A2 và ghi quyết định vào PR.
- [ ] Bảng quyết định A/B/C ở 2.2 có trong PR (bản sao hoặc liên kết) kèm kết luận của người duyệt; mọi ô "chưa kiểm chứng" còn lại được ghi lại.
- [ ] `main.css` có `--quality-error|warning|info|pass|unknown` (+ `-background`, `-border` cho bốn mức đầu) và `--quality-heat-1..5` ở `:root` **và** `.dark`, bind trong `@theme inline`; không hex mới.
- [ ] `quality-token-contrast.test.ts` xanh: chữ ≥ 4,5:1 trên `card`/`background` ở sáng và tối cho các cặp ở 2.3; đồ hoạ ≥ 3:1; chữ trên ô nhiệt ≥ 4,5:1 theo quy tắc bậc.
- [ ] `severity-encoding.ts` là nguồn duy nhất; `ChartLegend` và `SeverityBadge` dựng từ nó; test chứng minh không thể lệch.
- [ ] Mỗi hình F1–F8 có: `role` đúng, `aria-label` từ `chart-text-summary`, `ChartTextAlternative`, hoạt động ở sáng và tối, không dùng màu làm dấu hiệu duy nhất.
- [ ] F6/F7/F8 là một điểm dừng Tab và điều hướng bằng phím theo 2.5; test mô phỏng phím.
- [ ] `prefers-reduced-motion`: không chuyển tiếp/hoạt ảnh nào trong `quality-charts` (test giả `matchMedia`).
- [ ] Các hàm thuần: tổng diện tích squarified bằng diện tích khung (sai số < 0,5%), không chồng ô, tất định; thứ tự DSM đúng trên đồ thị có vòng, DAG và đồ thị rỗng; thang/tick không lặp.
- [ ] Vượt `maxTiles`/`maxNodes` thì gộp và hiện "X/Y", không cắt im lặng.
- [ ] Không file nào đặt tên `helpers/utils/common`; không `max-lines` disable; không dùng `components/code-review/*`; chuỗi `translate()` đủ 5 locale; không hex trong TS/TSX mới (test quét).
- [ ] Số đo đầu tiên của ngân sách 2.6 (hàm thuần và vẽ lần đầu với fixture cỡ trần) được ghi vào PR.

## 5. Kiểm thử

Vitest, môi trường `node` cho hàm thuần; `// @vitest-environment happy-dom` + Testing Library cho component. Chạy `pnpm --dir frontend test`. **Chưa chạy bất kỳ test nào; danh sách là kế hoạch.**

- `quality-token-contrast.test.ts` (2.3), `quality-token-parity.test.ts` (mỗi token có ở `:root`, `.dark` và `@theme inline`; không hex trong file mới).
- `treemap-squarified-layout.test.ts`, `dependency-matrix-ordering.test.ts`, `chart-linear-scale.test.ts`, `heat-intensity-scale.test.ts`, `chart-text-summary.test.ts` (câu không chứa từ kết luận; mã lạ không ném).
- Component: `ChartFrame` (đủ năm trạng thái, giữ chỗ `minHeight`, `table` bắt buộc), `ChartTextAlternative`, `SeverityBadge` (nhãn chữ luôn có), `HotspotHeatmap`/`DependencyMatrix`/`MetricTreemap` (một điểm dừng Tab, phím mũi tên, `aria-rowindex/colindex`, cắt và "X/Y"), `DiffCoverageGauge` (`estimated` hiển thị "ước lượng"; `total = 0` không chia cho 0, hiển thị "Chưa có dữ liệu").
- Thời gian: squarified 400 ô, DSM 150 nút (ngưỡng rộng).
- Playwright web: nạp `quality-chart-fixtures.ts` cỡ trần, đo `performance.measure` (kế hoạch, phụ thuộc hạ tầng e2e web; chưa xác nhận spec chạy được trong CI).
- Kiểm tay: sáng/tối, hiển thị trên Electron và web, bàn phím, trình đọc màn hình (chưa có công cụ tự động).

## 6. Rủi ro và điểm chưa kiểm chứng

- **Mọi số bundle, giấy phép, hỗ trợ React 19 của `recharts/visx/echarts/elkjs/dagre`** chưa kiểm chứng; chỉ ba gói d3 đã xác nhận ISC/ESM/có mặt trong lockfile. Không nên coi A3 là đã loại hoàn toàn.
- **Số tương phản** là tính tay; công thức chuyển `oklch` → sRGB và `color-mix` cần được test đảm bảo (xem 5); sai số làm tròn ±0,01.
- `--quality-*` dùng `var(--color-amber-700)`: chưa chắc Tailwind 4 luôn phát biến `--color-*` khi chưa có lớp tiện ích nào dùng màu đó. Tiền lệ `--ai-action-accent: var(--color-violet-500)` và `--terminal-pane-locate: var(--color-blue-600)` hoạt động, nhưng chưa kiểm bằng bản dựng. Dự phòng: định nghĩa trực tiếp dưới dạng hex trong `main.css` (cho phép vì là nơi định nghĩa token).
- **Va chạm hổ phách** (`--quality-warning` và `--review-untested`) giảm bằng dạng mã hoá khác nhau nhưng không loại bỏ; cần người dùng thử.
- **Thứ tự DSM** (Tarjan + tô-pô) có thể cho thứ tự kém đọc với đồ thị gần đầy; thử trên dữ liệu thật.
- **Ngân sách chưa có số đo**; ngưỡng 400/240/700 ô là kế thừa (054) hoặc đề xuất.
- **Treemap lặp**: có ba triển khai nếu không thống nhất (`workspace-space-layout.ts`, 054, CR này). Xem 7.1.
- Hai target Electron/web: chưa kiểm chứng `desktop/` có dùng chung `frontend/src/renderer` (điều CR-CV-050 mục 6 cũng nêu).
- Không có công cụ a11y tự động trong repo; kiểm tra truy cập ở đây phần lớn là test cấu trúc (`role`, `aria`) và kiểm tay.

## 7. Câu hỏi mở

1. **Ai sở hữu thuật toán treemap?** Đề xuất: `components/quality-charts/treemap-squarified-layout.ts` (CR này), CR-CV-054 nhập lại và bỏ `structure-treemap-layout.ts` (hoặc giữ tên đó làm lớp mỏng). Cần cập nhật CR-CV-054 mục 1 (xác nhận treemap sẵn có ở `status-bar/workspace-space-layout.ts`) và mục 2.3.
2. Đổi `--review-untested` sáng sang `amber-700`, hay chỉ cho dùng trên `card`? (CR-CV-050 2.10).
3. Ngưỡng để xin duyệt `elkjs`/`dagre` (O5): số nút, số cạnh cắt nhau, thời gian bố cục; chốt khi có thí nghiệm ở 2.2-B.
4. Có thêm khoá `frontend` vào `codeintel-budgets.json` của CR-CV-071 hay một tệp ngân sách riêng cho vẽ hình?
5. Người duyệt O12 có chấp nhận A2 (`d3-scale`, `d3-shape`, `d3-hierarchy`) ngay từ đầu để bớt mã tự viết, vì chúng đã nằm trong lockfile?
6. Nên chuyển phần chung (`ChartFrame`, `ChartTextAlternative`, thang màu) thành primitive ở `components/ui/` sau khi dùng ở hai nơi? Hiện đặt ở `quality-charts/` để không động vào `ui/` khi chưa có nhu cầu thứ hai.
7. Có cần chế độ tương phản cao (`prefers-contrast`)? `main.css` hiện không có truy vấn `prefers-contrast` (grep rỗng).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (O5, O9, O12, mục 3.10, mục 6, mục 8 điểm 13)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (§5 D1–D7), `10-frontend-review-ux.md` (§5, §9)
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md` (2.10 token), `CR-CV-053-impact-lens-and-symbol-detail.md` (2.5 `OVERLAY_ENCODING`), `CR-CV-054-structure-lens.md` (2.3 squarified), `CR-CV-055-architecture-c4-lens.md`, `CR-CV-057-erd-lens.md`, `CR-CV-051-review-workspace-shell.md` (`usePerceivedLoadingStage`)
- `/opt/repos/orca/docs/crs/v7/quality-rollout/CR-CV-071-performance-budgets-metrics-tracing.md`
- `/opt/repos/orca/guides/STYLEGUIDE.md` (Color roles, Color mixing, UX rule 1 và 2, Animation, "UI copy must not overclaim")
- `/opt/repos/orca/frontend/package.json`, `/opt/repos/orca/pnpm-lock.yaml` (:5800, :5868, :5902, :5913, :5943)
- `/opt/repos/orca/frontend/src/renderer/src/assets/main.css` (:43-122, :126-214, :216-302, :1987-1989), `components/ui/{progress,table,badge}.tsx`, `hooks/usePrefersReducedMotion.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/status-bar/{WorkspaceSpaceManagerPanel.tsx,workspace-space-layout.ts}`, `components/setup-guide/SetupGuideProgressRing.tsx`
- `/opt/repos/orca/node_modules/.pnpm/tailwindcss@4.2.4/node_modules/tailwindcss/theme.css`, `node_modules/.pnpm/@xyflow+react@12.11.2*/node_modules/@xyflow/react/dist/esm/types/component-props.d.ts`
- Mới: `components/quality-charts/*`, `test-support/quality-chart-fixtures.ts`, `i18n/code-intel-quality-locale-coverage.test.ts`

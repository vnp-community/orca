# FE-CV-SOL-088: Nền đồ hoạ chất lượng (token, mã hoá không chỉ dựa vào màu, primitive biểu đồ tự viết SVG)

> 📋 Proposed. Chưa triển khai. Viết ngày 2026-10-06 từ khảo sát code `frontend/src`; chưa chạy test, build hay ứng dụng. Các con số tương phản đã **tính lại độc lập bằng script đọc-chỉ** (không phải công cụ a11y), khớp bảng của CR.

**CR:** [CR-CV-088](../../../../../../docs/crs/v7/quality-visualization/CR-CV-088-graphics-foundation-and-chart-primitives.md)
**Area:** frontend (`frontend/src/renderer/src/assets/main.css`, `components/quality-charts/`, `test-support/`, `i18n/`)
**Thứ tự:** làm **trước** FE-CV-SOL-087-* (README feature: 088 trước 087). Không cần backend: chỉ dùng fixture.
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md) (không dùng store ở solution này), [v5/08-editor-and-files](../../../../tdd/v5/08-editor-and-files.md) (chỉ để đối chiếu: solution này không chạm Monaco).

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

**Hợp đồng áp dụng** (trích, không chép lại kiểu):

| Mục hợp đồng | Áp dụng vào solution này |
|---|---|
| `CONTRACT-codeintel-ui-api.md` §4.7 quy tắc hiển thị bắt buộc | `unknown` luôn "Chưa đủ dữ liệu để kết luận" với biểu tượng khác `pass`; cấm "an toàn", "đã đáp ứng"; không điểm số đơn; `CoverageReport.source:'estimated'` hiển thị khác `measured` (gauge có prop `source`) |
| `CONTRACT-codeintel-ui-api.md` §4.7 `QualityTrendPoint.metrics` | "khoá vắng = không có số, không phải 0" → `TrendLineChart` nhận `value: number \| null` và **ngắt đường** tại null (PQ-33) |
| `CONTRACT-codeintel-ui-api.md` §4.7 `QualityProfile.definition.coverage` (`diffCoverageWarnBelow`, `diffCoverageFailBelow`) | `DiffCoverageGauge` có **hai** vạch ngưỡng tuỳ chọn (xem "Lệch" #3) |
| `CONTRACT-codeintel-ui-api.md` §1 U9 | Mọi chuỗi tự do từ backend (nhãn hàng, tên file) chỉ render văn bản thuần trong biểu đồ và bảng thay thế |
| PQ-32 | Giá trị enum chữ thường (`error`, `pass`); `unknown` là giá trị hợp lệ của encoding |
| PQ-33, PQ-34 | Hình dạng dữ liệu thật do 087 ánh xạ; 088 chỉ định nghĩa props thuần, không import kiểu từ hợp đồng |
| `CONTRACT-codeintel-proto-and-data-map.md` §8.1/8.3 | Tên file theo khái niệm; mọi chuỗi qua `translate()` |
| §9 O-9 (`d3-*` thuộc danh sách cần duyệt O12) | Quyết định A1 "tự viết", không thêm dependency |

**Lệch giữa CR và hợp đồng** (hợp đồng thắng):

| # | CR-088 ghi | Hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `DiffCoverageGauge` nhận `{covered, total, threshold?, estimated?}` | `CoverageReport.diff = {changedExecutable, covered, uncovered, diffCoverage\|null, reason?, partial, excludedFiles[]}` và profile có **hai** ngưỡng (`WarnBelow`, `FailBelow`) | Props: `{covered, total, thresholds?: {warnBelow?: number\|null; failBelow?: number\|null}, source: 'measured'\|'estimated', partial?: boolean}`; phần trăm **tính từ `covered/total`**, không tin trường `diffCoverage` (đơn vị chưa nêu trong hợp đồng; xem câu hỏi mở 4) |
| 2 | Nhãn lượt `turnId` | `QualityTrendPoint.turnKey` | Trục X nhận `label` đã dựng sẵn; ánh xạ khoá lượt thuộc 087 |
| 3 | Mức nghiêm trọng 3 giá trị + `unknown` | `Finding.severity` cũng `error\|warning\|info`; `GateResult` có `warn` | `severity-encoding.ts` định nghĩa **hai** bảng: `SEVERITY_ENCODING` (`error/warning/info/unknown`) và `VERDICT_ENCODING` (`pass/warn/fail/unknown`), `warn` dùng chung token `--quality-warning` nhưng nhãn khác ("Cảnh báo" so với "Có cảnh báo") |
| 4 | CR-088 ghi `useLazyChartMount` | Không có yêu cầu hợp đồng | Giữ |

**Phụ thuộc chéo khu vực:**

| Cần | Từ | Ghi chú |
|---|---|---|
| Cơ chế token `--review-*`, bảng `OVERLAY_ENCODING` | `FE-CV-SOL-050-*` (CR-050 2.10), `FE-CV-SOL-053-impact-lens-and-symbol-detail` | 088 **dùng lại**, không định nghĩa lại; nếu chưa có thì lớp phủ `overlay` của `MetricTreemap` là prop thuần, không import |
| Registry lens lazy (`React.lazy`) | `FE-CV-SOL-051-review-workspace-shell` | `quality-charts` không import từ module khởi động |
| Số đo ngân sách vẽ hình | CR-071 (`BE-CV-SOL-071-metrics-tracing-and-budgets` chỉ có ngân sách backend) | Chỗ chứa ngân sách frontend chưa có (câu hỏi mở 5) |
| Fake backend G4 | `FE-CV-SOL-050-*` (`code-intel-fake-backend.ts`) | 088 **không** cần fake backend; chỉ `test-support/quality-chart-fixtures.ts` |
| Backend/agent | **Không có** | 088 không gọi kênh nào |

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc trong phiên này (đường dẫn thật): `frontend/package.json`, `frontend/src/renderer/src/assets/main.css` (:96-106, :126-160, :206, :216-246, :1987-1989), `components/ui/progress.tsx`, `hooks/usePrefersReducedMotion.ts`, `components/status-bar/workspace-space-layout.ts` (`TreemapInput`, `TreemapRect`, `splitBalanced`), `WorkspaceSpaceManagerPanel.tsx` (:89-93 `color-mix(... var(--chart-N) ...)`, :917 `aria-label` ô), `components/setup-guide/SetupGuideProgressRing.tsx` (76 dòng), thư mục `components/ui/`, `i18n/i18n.ts` (`translate(key, fallback, options)`), `i18n/task-jira-link-locale-coverage.test.ts` (mẫu phủ khoá "read-by-name"), `config/vitest.config.ts` (`environment: 'node'`, `include: src/**/*.test.ts(x)`), `node_modules/.pnpm/tailwindcss@4.2.4/node_modules/tailwindcss/theme.css`, `/opt/repos/orca/guides/STYLEGUIDE.md` (Color roles, Color mixing, UX rules, rubric).

**Xác nhận:**
- `components/quality-charts/` **chưa tồn tại**; `main.css` **không** có `--quality-*`; `--warning` không được định nghĩa nhưng được dùng ở `main.css:1987-1989` (`var(--warning, #f59e0b)`).
- `--chart-1..5` đều `var(--color-blue-300..800)` ở cả `:root` (:158-162) và `.dark` (:242-246): không đổi theo chủ đề.
- `frontend/package.json` không có thư viện biểu đồ trực tiếp (grep `recharts|d3|visx|echarts` không khớp); `@tanstack/react-virtual` ^3.13.24, `zustand` ^5.0.13, `monaco-editor` ^0.55.1, `happy-dom` ^20.9.0, `@testing-library/react` ^16.3.2 có. `tests/playwright.web.config.ts` tồn tại.
- `components/ui/` có `table, toggle, toggle-group, card, badge, skeleton, collapsible, tooltip, hover-card`; **không có** `alert`, `switch`, chart.
- `progress.tsx`: chỉ nhận `value`, indicator `transition-all duration-300 ease-out` cố định (đã đọc, 26 dòng).
- `usePrefersReducedMotion` trả boolean và lắng nghe `change`.
- Test đọc file nguồn bằng `readFileSync(join(process.cwd(), 'src/renderer/src/...'))` đã có tiền lệ (`app-startup-routing.test.ts`), nên test token đọc `main.css` được; `process.cwd()` là `frontend/` khi chạy `pnpm --filter orca-frontend test`.

**Tính lại độc lập (script đọc-chỉ, công thức WCAG 2.x; OKLCH → sRGB tuyến tính qua ma trận Oklab):** bảng tương phản token của CR-088 2.3 khớp ở cả 25 ô (ví dụ `--quality-error` sáng/`card` 4,87, sáng/`muted` **4,47**; `--quality-unknown` sáng/`muted` **4,35**; tối `--quality-warning`/`card` 10,44) và thang nhiệt khớp (sáng bậc 3 chữ `foreground` 7,30; tối bậc 3 chữ `foreground` **4,62**, biên chỉ 0,12 trên 4,5). Vì tối bậc 3 sát ngưỡng, test phải dùng ngưỡng 4,5 chính xác và phải cảnh báo nếu số làm tròn đổi. **Chưa kiểm chứng** bằng công cụ ngoài; `color-mix` giả định nội suy sRGB không tiền nhân alpha.

**Correction relative to CR-088:**

| # | CR ghi | Thực tế đã đọc / hợp đồng | Quyết định |
|---|---|---|---|
| 1 | `CR-CV-054 mục 1: không có treemap` sai | `workspace-space-layout.ts` export `buildTreemapLayout(items: TreemapInput[]): TreemapRect[]` (:117, chia đôi cân bằng bằng `splitBalanced`); ô là `<button>` `aria-label` có tên và dung lượng | Đúng như CR; solution viết squarified **mới** (tỉ lệ cạnh tốt hơn); kế thừa cách đặt `aria-label`/`color-mix`. Không refactor `workspace-space-layout.ts` |
| 2 | Token `--quality-warning` dùng `var(--color-amber-700)` | `theme.css` có `--color-amber-700: oklch(55.5% 0.163 48.998)`, `--color-sky-700: oklch(50% 0.134 242.749)`, `--color-amber-400`, `--color-sky-400` (đã đọc) | Dùng được; nhưng chưa kiểm Tailwind 4 có phát biến `--color-*` khi không có utility dùng (rủi ro 6.1); dự phòng định nghĩa hex trong `main.css` (nơi định nghĩa token được phép) |
| 3 | Bảng quyết định A/B/C nằm trong CR | Cần ghi lại ở PR | Task 088-09 sinh tệp ghi chú quyết định trong mô tả PR (không tạo file `.md` ngoài yêu cầu) |
| 4 | Mô tả `usePerceivedLoadingStage` của CR-051 | Chưa tồn tại (CR-051 chưa làm) | `ChartFrame` nhận `status` từ hook gọi; không import hook |

## 2. Giải pháp

### 2.1 Quyết định thư viện (ghi lại để người duyệt O12 quyết lại)

| Phương án | Quyết định | Bằng chứng đã đọc |
|---|---|---|
| A1 tự viết SVG/HTML + hàm thuần | **Chọn** | Hình ở F1-F9 đều hình học đơn giản; tiền lệ `SetupGuideProgressRing`, treemap `WorkspaceSpaceManagerPanel`; không dependency mới (O5, O12; `CONTRACT` §9 O-9) |
| A2 chỉ `d3-scale`/`d3-shape`/`d3-hierarchy` | Không làm; điều kiện xem lại: (a) tick/scale tự viết vượt ~120 dòng hoặc lỗi lặp, (b) cần trục thời gian liên tục hoặc đường cong, (c) 054 và 088 lặp mã treemap | ISC, ESM, có trong lockfile (CR đã đọc `pnpm-lock.yaml:5868, 5902, 5913`); bundle sau tree-shake **chưa đo** |
| A3 thư viện biểu đồ React | Không đánh giá | Không có trong lockfile; giấy phép/React 19/bundle **chưa kiểm chứng** |
| elkjs/dagre; canvas/WebGL | Không thêm | Không có nút-cạnh trong 088; DSM thay danh sách khi > ~300 nút |

### 2.2 Cây file (mới trừ khi ghi khác)

```
frontend/src/renderer/src/assets/main.css                       (sửa) token --quality-* và --quality-heat-1..5, bind @theme inline
frontend/src/renderer/src/components/quality-charts/
  severity-encoding.ts                  bảng mã hoá duy nhất (mức + kết luận)
  SeverityGlyph.tsx  SeverityBadge.tsx  GateVerdictBadge.tsx  ChartLegend.tsx
  chart-linear-scale.ts                 thang tuyến tính + tick (thuần)
  heat-intensity-scale.ts               giá trị -> bậc 1..5 (thuần)
  chart-text-summary.ts                 câu mô tả số đo (thuần, i18n)
  treemap-squarified-layout.ts          squarified một mức (thuần)
  dependency-matrix-ordering.ts         Tarjan SCC + thứ tự (thuần)
  ChartFrame.tsx  ChartTextAlternative.tsx  ChartHoverCard.tsx
  useChartSize.ts  useLazyChartMount.ts  useChartKeyboardNavigation.ts
  StackedSeverityBar.tsx  SparklineChart.tsx  TrendLineChart.tsx  DiffCoverageGauge.tsx
  MetricTreemap.tsx  HotspotHeatmap.tsx  DependencyMatrix.tsx
  __tests__/*.test.ts(x)
frontend/src/renderer/src/test-support/quality-chart-fixtures.ts  dữ liệu tổng hợp cỡ trần ngân sách
frontend/src/renderer/src/i18n/code-intel-quality-locale-coverage.test.ts  phủ khoá 5 locale (dùng chung với 087)
frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json    (sửa) khoá auto.components.qualityCharts.*
```

Tất cả tên theo khái niệm; không `helpers/utils/common`; không `max-lines` disable (`AGENTS.md`). Thư mục `renderer/src/test-support/` đã tồn tại (đã liệt kê); nội dung hiện có chưa đọc.

### 2.3 Token (`main.css`)

Thêm vào **cả** `:root` và `.dark` và `@theme inline` (mẫu `--color-status-success*` ở :101-103):

| Token | Sáng | Tối |
|---|---|---|
| `--quality-error` | `var(--destructive)` | `var(--destructive)` |
| `--quality-warning` | `var(--color-amber-700)` | `var(--color-amber-400)` |
| `--quality-info` | `var(--color-sky-700)` | `var(--color-sky-400)` |
| `--quality-pass` | `var(--status-success)` | `var(--status-success)` |
| `--quality-unknown` | `var(--muted-foreground)` | `var(--muted-foreground)` |
| `--quality-{error,warning,info,pass}-background` | `color-mix(in srgb, var(--quality-X) 10%, transparent)` | như sáng |
| `--quality-{error,warning,info,pass}-border` | `color-mix(in srgb, var(--quality-X) 25%, transparent)` | như sáng |
| `--quality-heat-1..5` | `color-mix(in srgb, var(--foreground) {8,22,40,62,85}%, var(--card))` | như sáng |

Quy tắc dùng (test hoá): chữ màu mức nghiêm trọng chỉ trên `card`/`background` (≥ 4,5:1); trên `muted` hoặc nền tint, chữ dùng `--foreground`/`--muted-foreground`, màu mức chỉ cho biểu tượng, viền, hình (≥ 3:1). Ô nhiệt luôn có viền `--border` vì bậc 1 chỉ cách `card` khoảng 1,1:1 (tính lại: chữ/ô bậc 1 sáng 16,67 nhưng ô/`card` không phải ngưỡng chữ). Chữ trong ô bậc 1-3 dùng `--foreground`, bậc 4-5 dùng `--background`.

**Không** sửa `--review-*` hay `--chart-*`. **Không** thêm `--warning`.

### 2.4 Bảng mã hoá (nguồn duy nhất)

```ts
// severity-encoding.ts
export type QualitySeverityLevel = 'error' | 'warning' | 'info' | 'unknown'
export type QualityVerdictLevel = 'pass' | 'warn' | 'fail' | 'unknown'
export type EncodingEntry = {
  token: string              // tên token CSS, ví dụ '--quality-error'
  textClass: string          // 'text-quality-error'
  icon: LucideIcon           // OctagonX | TriangleAlert | Info | CircleCheck | CircleHelp
  shape: 'octagon' | 'triangle' | 'circle-open' | 'circle-check' | 'circle-dashed'
  strokeDash: string | null  // null | '4 2' | '1 2' | '2 3'
  labelKey: string; labelFallback: string
}
export const SEVERITY_ENCODING: Record<QualitySeverityLevel, EncodingEntry>
export const VERDICT_ENCODING: Record<QualityVerdictLevel, EncodingEntry>
export function toSeverityLevel(raw: string): QualitySeverityLevel   // enum lạ -> 'unknown'
export function toVerdictLevel(raw: string): QualityVerdictLevel     // enum lạ -> 'unknown'
```

`unknown` dùng `CircleHelp`, `circle-dashed`, nhãn "Chưa rõ"; **không bao giờ** dùng `CircleCheck`. `warn` dùng `TriangleAlert`. Test "không thể lệch": mọi `icon` khác nhau giữa `pass` và `unknown`; mọi cặp `{shape, strokeDash}` khác nhau.

### 2.5 Props và chữ ký

```ts
type ChartTableData = { caption: string
  columns: { key: string; label: string; align?: 'start' | 'end' }[]
  rows: Record<string, string | number | null>[] }          // null hiển thị "—", không 0

type ChartFrameProps = {
  id: string; title: string; description?: string
  summary: string                         // aria-label từ chart-text-summary
  status: 'loading' | 'ready' | 'empty' | 'error' | 'stale'
  emptyReason?: React.ReactNode           // luôn giải thích VÌ SAO trống
  error?: { message: string; onRetry?: () => void }
  staleNote?: string
  legend?: React.ReactNode
  table: ChartTableData                   // bắt buộc
  minHeight: number
  children: (size: { width: number; height: number }) => React.ReactNode
}
type LinearSeriesPoint = { label: string; value: number | null; marker?: QualityVerdictLevel }
type TrendLineChartProps = { series: { id: QualitySeverityLevel | string; label: string; points: LinearSeriesPoint[] }[]
  xLabels: string[]; maxPoints?: number /* 50 */ ; onSelectPoint?: (index: number) => void }
type DiffCoverageGaugeProps = { covered: number; total: number
  thresholds?: { warnBelow?: number | null; failBelow?: number | null }   // phần trăm 0..100
  source: 'measured' | 'estimated'; partial?: boolean }
type MetricTreemapProps = { items: { id: string; label: string; size: number; intensity: number; overlay?: string[] }[]
  sizeLabel: string; intensityLabel: string; onSelect?: (id: string) => void; maxTiles?: number /* 400 */ }
type HotspotHeatmapProps = { rows: { id: string; label: string; values: (number | null)[] }[]
  columns: { key: string; label: string; unit?: string }[]; onSelectRow?: (id: string) => void; maxRows?: number /* 40 */ }
type DependencyMatrixProps = { nodes: { id: string; label: string; group?: string }[]
  edges: { from: string; to: string; weight: number }[]; maxNodes?: number /* 60 */
  onSelectCell?: (from: string, to: string) => void }
```

Hàm thuần:

```ts
export function squarify(items: { id: string; size: number }[], bounds: { x: number; y: number; width: number; height: number }): { id: string; x: number; y: number; width: number; height: number }[]
export function orderByStrongComponents(nodes: string[], edges: { from: string; to: string }[]): { order: string[]; blocks: string[][] }  // blocks: nhóm vòng (SCC > 1)
export function bucketIntensity(value: number, domain: { min: number; max: number }): 1 | 2 | 3 | 4 | 5
export function describeSeriesRange(label: string, points: (number | null)[]): string   // chỉ nêu số đo, không "cải thiện"
```

### 2.6 Wireframe `ChartFrame` và ô lưới

```
┌ Số phát hiện theo lượt ──────────────────── [Xem dạng bảng] ┐
│ Trục ngang: lượt (cũ → mới). Trục dọc: số phát hiện.          │
│   ⊗ 12 ──┐        ▲ cảnh báo (tam giác)  ⊗ lỗi (bát giác)     │
│          ⊗──┐     ○ thông tin (tròn rỗng)                      │
│   ▲ 8 ──▲──▲──▲                                                │
│      L1  L2  L3  L4          ⋯ kết luận đổi (đánh dấu ◆)       │
│ Chú giải dựng từ SEVERITY_ENCODING                              │
└ Số liệu tại HEAD a41c9e0 · 14:02 ───────────────────────────┘

HotspotHeatmap (role="grid", một điểm dừng Tab)
 file                 churn   findings  uncovered
 src/a/b.ts           ▓ 18    ▒  4      ░  12%     (số luôn hiển thị trong ô)
 src/a/c.ts           ░  3    —         ▒  41%     (— = không có số, không phải 0)
```

Lưới nền DSM là một `<path>`/`pattern`; ô lấp là `<rect>`; vòng đánh dấu bằng hình `▲` và khung đậm (không chỉ màu).

### 2.7 Truy cập được

- `ChartFrame` render `figure` + `figcaption`; vùng vẽ `role="img"` (F3/F4/F5) hoặc `role="grid"` (F6-F8) với `aria-label={summary}`; bảng thay thế luôn có trong DOM (`sr-only` khi chưa bật nút `ui/toggle` "Xem dạng bảng", `display` thật khi bật). Không `display:none` để trình đọc màn hình vẫn tới.
- `useChartKeyboardNavigation`: một điểm dừng Tab, tiêu điểm lăn `tabindex`, mũi tên, `Home/End`, `Ctrl+Home/End` (nhãn theo nền tảng khi hiển thị; không hiển thị chip nếu chưa cài), `Enter`, `Esc`; `aria-rowindex/colindex`. `focus-visible:ring-2 ring-ring`.
- Reduced motion: không transition/animation trong `quality-charts`; `usePrefersReducedMotion` chỉ dùng nếu có hoạt ảnh (mặc định không có); test giả `matchMedia`.
- Tooltip chỉ phụ; nội dung tooltip cũng có trong bảng thay thế. `ChartHoverCard` không nuốt tiêu điểm.
- Không có công cụ a11y tự động trong repo (README feature điểm 14): kiểm tra bằng test cấu trúc (`role`, `aria-*`, thứ tự Tab) và kiểm tay.

### 2.8 Hiệu năng

Ngân sách (đề xuất, chưa đo): F6 ≤ 400 ô; F7 ≤ 40×6=240 ô; F8 ≤ 60×60 với ≤ 700 ô lấp; F4 ≤ 50 điểm × ≤ 4 chuỗi; squarified 400 ô ≤ 16 ms; DSM 150 nút ≤ 50 ms; vẽ lần đầu F1-F5 ≤ 100 ms, F6-F8 ≤ 250 ms. Vượt trần thì gộp "+N"/"X/Y" kèm thông báo, không cắt im lặng. `useLazyChartMount` (IntersectionObserver) cho F6-F8, giữ chỗ bằng `minHeight`; `useChartSize` dùng một `ResizeObserver` cho cả khung, gộp `requestAnimationFrame`; không `ResizeObserver` mỗi ô. Danh sách dài (phát hiện) ảo hoá ở 087, không ở đây.

## 3. Quyết định thiết kế

- Tự viết SVG, không dependency; ghi điều kiện xem lại A2.
- Một quy ước nhiệt (đậm = cần chú ý hơn), không hue mới, luôn kèm số.
- `table` bắt buộc trong `ChartFrameProps` (không tuỳ chọn): a11y không phải việc làm sau.
- `summary` chỉ nêu số đo, không kết luận.
- Gauge là bullet bar, hai vạch ngưỡng, `source:'estimated'` hiển thị "Ước lượng" cạnh số.
- `value: null` ngắt đường và hiển thị "—" trong bảng (PQ-33), không bao giờ vẽ 0.
- `treemap-squarified-layout.ts` đặt ở `quality-charts/` để FE-CV-SOL-054 nhập lại.
- Không sửa `progress.tsx`: 087 dùng spinner tĩnh khi `percent === null`.

## 4. Tiêu chí chấp nhận

- [ ] `package.json`/`pnpm-lock.yaml` không đổi (trừ khi người duyệt O12 chọn A2 và ghi quyết định).
- [ ] `main.css` có đủ token ở `:root`, `.dark`, `@theme inline`; không hex mới trong TS/TSX; test parity xanh.
- [ ] `quality-token-contrast.test.ts` xanh: chữ ≥ 4,5 (trên `card`/`background`), đồ hoạ ≥ 3, chữ trên ô nhiệt ≥ 4,5 theo bậc; test **fail** nếu đưa chữ `--quality-error` sáng lên `muted` (ví dụ 4,47).
- [ ] `severity-encoding.ts` là nguồn duy nhất; `ChartLegend`, `SeverityBadge`, `GateVerdictBadge` dựng từ nó; `unknown` không dùng biểu tượng `pass`.
- [ ] Mỗi hình F1-F8 có `role`, `aria-label` từ `chart-text-summary`, bảng thay thế, hoạt động sáng/tối, không màu làm dấu hiệu duy nhất.
- [ ] F6/F7/F8 là một điểm dừng Tab, điều hướng phím theo 2.7.
- [ ] Hàm thuần: tổng diện tích squarified = diện tích khung (sai số < 0,5%), không chồng ô, tất định; DSM đúng trên DAG, đồ thị có vòng, đồ thị rỗng, tự vòng; thang/tick không lặp.
- [ ] Vượt `maxTiles`/`maxNodes`/`maxRows` thì hiện "Đang hiển thị X/Y".
- [ ] `DiffCoverageGauge`: `total = 0` hiển thị "Chưa có dữ liệu" (không chia 0, không 0%); `estimated` hiển thị "Ước lượng".
- [ ] Không file nào đặt tên `helpers/utils/common`; không `max-lines` disable; không dùng `components/code-review/*`; mọi chuỗi qua `translate()` đủ 5 locale.
- [ ] Số đo đầu tiên của ngân sách 2.8 ghi vào PR.

## 5. Kiểm thử

Vitest (môi trường `node` cho hàm thuần; `// @vitest-environment happy-dom` + Testing Library cho component). Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts`. **Chưa chạy.**

- `quality-token-contrast.test.ts`, `quality-token-parity.test.ts` (đọc `main.css` và `theme.css` bằng `readFileSync(join(process.cwd(), ...))`).
- `treemap-squarified-layout.test.ts`, `dependency-matrix-ordering.test.ts`, `chart-linear-scale.test.ts`, `heat-intensity-scale.test.ts`, `chart-text-summary.test.ts` (câu không chứa "an toàn", "sạch", "cải thiện"; mã lạ không ném).
- Component: `ChartFrame` (năm trạng thái, `minHeight`, `table` bắt buộc), `ChartTextAlternative`, `SeverityBadge` (nhãn chữ luôn có), `TrendLineChart` (null ngắt đường), `DiffCoverageGauge`, ba hình lưới (một Tab stop, phím mũi tên, `aria-rowindex/colindex`, cắt "X/Y").
- Thời gian: squarified 400 ô, DSM 150 nút (ngưỡng rộng).
- Playwright web (`tests/playwright.web.config.ts`): kế hoạch, phụ thuộc hạ tầng; chưa xác nhận spec chạy trong CI.
- Quét "không hex": test đọc các file mới và khẳng định không khớp `#[0-9a-fA-F]{3,8}` ngoài `main.css`.

## 6. Rủi ro và điểm chưa kiểm chứng

1. Tailwind 4 có thể không phát `--color-amber-700` khi không có utility dùng nó; dự phòng hex trong `main.css`. Chưa kiểm bằng bản dựng.
2. Số tương phản: tính lại cùng công thức nhưng chưa qua công cụ ngoài; tối bậc 3 biên 0,12.
3. Va chạm hổ phách giữa `--quality-warning` và `--review-untested`: giảm bằng dạng mã hoá khác; cần thử người dùng. CR-088 7.2 đề nghị đổi `--review-untested` sáng sang `amber-700` (thuộc CR-050/053).
4. Thứ tự DSM (Tarjan + tô-pô) kém đọc với đồ thị gần đầy; thử trên dữ liệu thật.
5. Ngưỡng ngân sách chưa có số đo; không có `ResizeObserver` mẫu trong `quality-charts` để so.
6. Chưa kiểm chứng: `desktop/` có dùng chung `frontend/src/renderer` (CR-050 mục 6 cũng nêu).

## 7. Câu hỏi mở

1. Treemap: chủ sở hữu thuật toán là 088; cần CR-054/FE-CV-SOL-054 nhập lại và sửa mục 1 của CR-054 (đã có treemap chia đôi ở `status-bar`).
2. Đổi `--review-untested` sáng sang `amber-700` hay giới hạn dùng trên `card` (CR-050 2.10).
3. Ngưỡng xin duyệt `elkjs`/`dagre`: chốt sau thí nghiệm bố cục (ngoài solution).
4. **Hợp đồng thiếu:** đơn vị của `CoverageReport.diff.diffCoverage`, `totals.pct`, `files[].pct`, `QualityTrendPoint.metrics.diffCoverage` và `QualityProfile...diffCoverageWarnBelow` (tỉ lệ 0..1 hay phần trăm 0..100) không được nêu. 088 chỉ nhận số đã chuẩn hoá thành phần trăm 0..100; việc ép đơn vị ở 087-15.
5. Khoá `frontend` trong `codeintel-budgets.json` của CR-071 hay tệp ngân sách riêng.
6. Chuyển `ChartFrame`/`ChartTextAlternative` thành primitive `components/ui/` sau khi có nơi dùng thứ hai.
7. Chế độ tương phản cao (`prefers-contrast`): `main.css` không có truy vấn này.

## 8. Tasks

Xem [../tasks/README.md](../tasks/README.md): FE-CV-TASK-088-01 đến 088-09.

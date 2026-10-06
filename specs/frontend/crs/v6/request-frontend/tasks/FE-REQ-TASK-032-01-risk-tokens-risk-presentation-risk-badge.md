# FE-REQ-TASK-032-01: Token màu rủi ro, `riskPresentation` và `RiskBadge`

**From Solution:** [FE-REQ-SOL-032](../solutions/FE-REQ-SOL-032-graph-canvas-and-lenses.md) mục 2.6
**Priority:** P0
**Area:** frontend / design tokens / graph
**File:** `frontend/src/renderer/src/assets/main.css` (sửa); `guides/STYLEGUIDE.md` (sửa); `frontend/src/renderer/src/components/graph/risk-presentation.ts`, `RiskBadge.tsx` (mới); test cùng tên; `frontend/src/shared/graph-types.ts` chỉ import kiểu `GraphRisk` (task 032-03 tạo; nếu làm trước thì khai `type GraphRisk` tạm trong `risk-presentation.ts` rồi chuyển)
**Depends on:** không (có thể làm ngay)
**Status:** [ ] TODO

## Context

- `main.css` (đã đọc): `@theme inline` bắt đầu dòng 43; `:root` dòng 126; `.dark` dòng 216. Mẫu token: `--status-success` (dòng 155, hex) với `--status-success-background` và `--status-success-border` bằng `color-mix(in srgb, var(--status-success) 10%/25%, transparent)` (dòng 156 đến 157); bind trong `@theme inline` dòng 101 đến 103 (`--color-status-success`, `-background`, `-border`).
- Hiện **không có** `--risk-*`, `--graph-edge-*` (grep `--risk|--graph-edge|--review` trong `main.css` rỗng). `--destructive` `#e40014` sáng và `#ff6568` tối (dòng 148, 232).
- STYLEGUIDE dòng 24: thêm token mới phải ở `main.css` cả `:root` và `.dark`, expose trong `@theme inline`, rồi mới dùng. Dòng 7: "color is reserved for state" (cần ghi `--risk-*` là trạng thái được phép). Dòng 296: `destructive` chỉ cho mất dữ liệu.
- `--ai-action-accent` dùng `var(--color-violet-*)` (dòng 154, 238) nên biến màu Tailwind `--color-*` có trong theme; **chưa kiểm chứng** riêng `--color-amber-600|400` và `--color-orange-600|400`. Bước 1 dưới đây xác minh.
- v7 CR-CV-050 mục 2.10 (đề xuất `--review-*`) chưa có trong `main.css`; chỉ phối hợp về sắc, không gộp tên.

## Việc cần làm

1. Xác minh theme: chạy `rg -n "color-amber|color-orange" frontend/src/renderer/src/assets/main.css`; nếu không có, mở `frontend/src/renderer/src/assets/` tìm file `@import "tailwindcss"` và xác nhận Tailwind v4 phát biến `--color-amber-600` (kiểm bằng cách build `pnpm --filter orca-frontend build` rồi grep CSS đầu ra). Nếu biến không phát, khai báo hex dự phòng trong `:root` và `.dark` (đây là chỗ DUY NHẤT được hex, ghi chú lý do).
2. `main.css` `:root`: thêm `--risk-low: var(--status-success)`, `--risk-medium: var(--color-amber-600)`, `--risk-high: var(--color-orange-600)`, `--risk-critical: var(--destructive)`; với mỗi mức thêm `--risk-<mức>-background: color-mix(in srgb, var(--risk-<mức>) 10%, transparent)` và `--risk-<mức>-border: color-mix(in srgb, var(--risk-<mức>) 25%, transparent)`; thêm `--graph-edge-added: var(--status-success)`, `--graph-edge-removed: var(--destructive)`.
3. `.dark`: ghi đè `--risk-medium: var(--color-amber-400)`, `--risk-high: var(--color-orange-400)` (`low` và `critical` kế thừa qua `--status-success`, `--destructive` đã đổi theo `.dark`; nhưng khai lại các dòng `-background`/`-border` không cần vì `color-mix` đọc biến hiện hành).
4. `@theme inline`: bind `--color-risk-low|medium|high|critical`, `--color-risk-<mức>-background`, `--color-risk-<mức>-border`, `--color-graph-edge-added`, `--color-graph-edge-removed` theo mẫu dòng 101 đến 103, để dùng `text-risk-high`, `bg-risk-high-background`, `border-risk-high-border`.
5. `risk-presentation.ts` (thuần): `export function riskPresentation(level: GraphRisk): { labelKey: string; Icon: LucideIcon; borderWidthClass: 'border' | 'border-2'; doubleRing: boolean; dashed: boolean; textClass: string; bgClass: string; borderClass: string }`. Bảng cố định: `low` `CircleCheck`, `medium` `TriangleAlert`, `high` `OctagonAlert`, `critical` `ShieldAlert` (`doubleRing: true`), `unknown` `CircleHelp` (`dashed: true`, dùng `text-muted-foreground`, KHÔNG dùng token `--risk-*`). `labelKey` = `auto.components.graph.RiskLevel.<mức>`. Giá trị lạ trả về của `unknown` (không bao giờ của `low`).
6. `RiskBadge.tsx`: props `{ level: GraphRisk; assessedAt?: string | null; tool?: string | null; size?: 'sm' | 'md'; showLabel?: boolean }`. Render `Badge` (`components/ui/badge.tsx`) chứa `Icon` (`aria-hidden`) + chữ nhãn (`useTranslation`), `Tooltip` (`components/ui/tooltip.tsx`) với khoá `auto.components.graph.RiskTooltip` nội suy `{{level}}`, `{{assessedAt}}`, `{{tool}}`; thiếu `assessedAt` thì dùng khoá `RiskTooltipUnassessed`. `showLabel=false` vẫn có `aria-label` bằng nhãn. Không viết "an toàn" ở bất kỳ khoá nào.
7. Thêm khoá `auto.components.graph.RiskLevel.{low,medium,high,critical,unknown}`, `RiskTooltip`, `RiskTooltipUnassessed` vào 5 locale (`en.json`, `es.json`, `ja.json`, `ko.json`, `zh.json`); en: Low, Medium, High, Critical, Not assessed. Bản dịch khác en được người dịch duyệt (không để trống).
8. `guides/STYLEGUIDE.md`: thêm mục "Risk tokens" (bảng token, quy tắc "chữ trước, hình thứ hai, màu thứ ba", `unknown` không dùng màu rủi ro, không dùng `--git-decoration-*`), và sửa câu dòng 7 để liệt kê rủi ro là trạng thái dùng màu.

## Bảng tham chiếu nhanh

| Hạng mục | Giá trị |
|---|---|
| Token mới | `--risk-low`, `--risk-medium`, `--risk-high`, `--risk-critical`, mỗi cái kèm `-background` (10%) và `-border` (25%); `--graph-edge-added`, `--graph-edge-removed` |
| Bind Tailwind | `--color-risk-*`, `--color-graph-edge-*` trong `@theme inline` |
| Icon (lucide-react) | `CircleCheck`, `TriangleAlert`, `OctagonAlert`, `ShieldAlert`, `CircleHelp` |
| Khoá i18n | `auto.components.graph.RiskLevel.{low,medium,high,critical,unknown}`, `auto.components.graph.RiskTooltip`, `auto.components.graph.RiskTooltipUnassessed` |
| Kênh WS | không (task thuần giao diện) |
| Phím tắt | không |

Trạng thái giao diện của `RiskBadge`: mức hợp lệ hiển thị chữ + icon; `level` thiếu hoặc lạ hiển thị "Chưa đánh giá" (icon `CircleHelp`, nét đứt); không có trạng thái đang tải hay lỗi vì component chỉ nhận dữ liệu đã có.

## Trình tự làm gợi ý

1. Viết `risk-presentation.test.ts` trước (5 mức, `unknown` không giống `low`).
2. Thêm token vào `main.css` (`:root`, `.dark`, `@theme inline`), build để xác nhận biến `--color-amber-*` tồn tại.
3. Viết `risk-presentation.ts`, rồi `RiskBadge.tsx` và test.
4. Thêm khoá i18n 5 locale và cập nhật `STYLEGUIDE.md`.
5. Chạy test và kiểm tĩnh `rg` không hex; chụp ảnh thang xám đính vào PR.

## Kiểm thử

- `risk-presentation.test.ts`: với mỗi trong 5 mức có `labelKey`, `Icon` khác nhau, `unknown` có `dashed`, `critical` có `doubleRing`; giá trị ngoài miền (ép kiểu) trả như `unknown`; không có chuỗi `#` nào trong kết quả.
- `RiskBadge.test.tsx` (`// @vitest-environment happy-dom`, mẫu `TaskDAGView.test.tsx`): 5 mức đều hiển thị chữ (không chỉ icon); `showLabel=false` có `aria-label`; tooltip có `tool` và `assessedAt` khi truyền; không có phần tử nào tên "safe".
- Kiểm tĩnh: `rg -n '#[0-9a-fA-F]{3,6}' frontend/src/renderer/src/components/graph` rỗng; `rg -n -- '--risk-' frontend/src/renderer/src/assets/main.css` có đủ ở `:root`, `.dark` (medium, high), `@theme inline`.
- Thang xám: chụp ảnh `RiskBadge` 5 mức (tay, ghi vào PR) và xác nhận phân biệt được không cần màu. Đo tương phản `--risk-medium` và `--risk-high` trên nền sáng và tối bằng công cụ kiểm tương phản (chưa đo; ghi số vào PR).
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/graph/risk-presentation src/renderer/src/components/graph/RiskBadge`.

## Tiêu chí hoàn thành

- [ ] `main.css` có `--risk-low|medium|high|critical` (+ `-background`, `-border`) và `--graph-edge-added|removed` ở `:root` và `.dark`, bind trong `@theme inline`.
- [ ] `STYLEGUIDE.md` mô tả token và quy tắc "rủi ro không chỉ bằng màu".
- [ ] `riskPresentation` phân biệt đủ 5 mức bằng chữ, icon và kiểu viền; `unknown` không bao giờ trả kiểu của `low`.
- [ ] `RiskBadge` chạy ở 5 mức, có tooltip "đánh giá lúc, dựa trên".
- [ ] Khoá i18n có đủ 5 locale; test phủ khoá (task 032-08 gom) hoặc test cục bộ ở file này xanh.
- [ ] Không hex trong `components/graph/`.

## Rủi ro và lưu ý

- Hổ phách và cam gần nhau trong chủ đề tối; nếu hai mức khó tách, dựa vào icon và độ dày viền (đã có trong thiết kế), không tăng thêm màu.
- Phối hợp với v7 CR-CV-050: nếu `--review-untested` được thêm trước, tham chiếu cùng `--color-amber-*`; không tạo hai sắc hổ phách khác nhau.
- `--risk-low` dùng `--status-success`: nghĩa "đã đạt" trong nơi khác của app; chữ "Thấp" và icon `CircleCheck` giữ nghĩa là mức rủi ro, không phải "đã xác minh an toàn".
- `main.css` rất lớn (hơn 3.400 dòng, ratchet `config/max-lines-baseline.txt` có thể áp dụng): chỉ thêm khối token vào các vùng sẵn có, không thêm `max-lines` disable.

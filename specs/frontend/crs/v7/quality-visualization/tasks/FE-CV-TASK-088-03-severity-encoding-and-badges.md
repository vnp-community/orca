# FE-CV-TASK-088-03: Bảng mã hoá mức nghiêm trọng, glyph, badge, chú giải

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.4
**Priority:** P0
**Area:** frontend / components
**File:** `frontend/src/renderer/src/components/quality-charts/severity-encoding.ts`, `SeverityGlyph.tsx`, `SeverityBadge.tsx`, `GateVerdictBadge.tsx`, `ChartLegend.tsx` (đều mới) và `__tests__/severity-encoding.test.ts`, `SeverityBadge.test.tsx`
**Depends on:** FE-CV-TASK-088-01
**Status:** [x] DONE

## Context

- Hợp đồng `CONTRACT-codeintel-ui-api.md` §4.7: `unknown` hiển thị "Chưa đủ dữ liệu để kết luận" (kết luận) hoặc "Chưa rõ" (mức), biểu tượng khác `pass`, không dùng "an toàn", "đã đáp ứng". PQ-32: enum chữ thường. `GateResult = 'pass'|'warn'|'fail'|'unknown'`; mức = `error|warning|info`.
- Mẫu tham chiếu: `OVERLAY_ENCODING` của FE-CV-SOL-053 (nếu đã có: cùng cấu trúc; nếu chưa, không import).
- Icon `lucide-react` ^0.577.0 (đã có): `OctagonX`, `TriangleAlert`, `Info`, `CircleCheck`, `CircleHelp`. **Chưa kiểm chứng** tên export tồn tại ở bản này; kiểm trong task (dùng `import * as` hoặc tra `node_modules/lucide-react/dist/lucide-react.d.ts`).
- STYLEGUIDE: biểu tượng icon đơn giản đã dùng gần đó; không emoji.

## Việc cần làm

1. `severity-encoding.ts` theo chữ ký SOL-088 2.4: `SEVERITY_ENCODING`, `VERDICT_ENCODING`, `toSeverityLevel`, `toVerdictLevel` (enum lạ → `'unknown'`, không ném). Nhãn qua `translate()` **lúc gọi** (hàm `labelOf(entry)`), không ở cấp module (`i18n/no-top-level-translate.test.ts`).
2. `SeverityGlyph.tsx`: SVG theo `shape` (bát giác đặc, tam giác, tròn rỗng, tròn có tick, tròn nét đứt dài), `aria-hidden`, kích thước theo `size`; dùng `currentColor` + lớp `text-quality-*`.
3. `SeverityBadge.tsx`: `{severity, count?, withLabel?}`; luôn có **nhãn chữ** hoặc `aria-label` đầy đủ khi `withLabel=false`; `count` là số nguyên không âm; dùng `ui/badge` (không tạo biến thể mới nếu `badge.tsx` đủ).
4. `GateVerdictBadge.tsx`: `{verdict, stale?}`; `unknown` → "Chưa đủ dữ liệu để kết luận" (phiên bản dài) hoặc "Chưa rõ" (compact); `stale` thêm chữ "cũ" (không chỉ đổi màu).
5. `ChartLegend.tsx`: dựng từ `SEVERITY_ENCODING`/`VERDICT_ENCODING` (nhận danh sách khoá cần hiển thị), liệt kê glyph + nhãn + nét; không bảng chú giải riêng.
6. Khoá i18n: `auto.components.qualityCharts.severity.{error,warning,info,unknown}`, `...verdict.{pass,warn,fail,unknown}`, `...verdict.unknownLong`, `...stale`. (Thêm vào 5 locale ở 088-09 hoặc ngay đây nếu làm trước.)

## Kiểm thử

- `severity-encoding.test.ts`: mọi cặp `{shape, strokeDash}` khác nhau; `pass` và `unknown` khác `icon`; `toSeverityLevel('critical')` = `'unknown'`; không ném với `undefined`/số.
- `SeverityBadge.test.tsx` (happy-dom): luôn có nhãn chữ; `unknown` không chứa `CircleCheck` (kiểm theo `data-icon`/`data-shape` do component đặt); copy `unknown` không chứa "an toàn", "sạch", "đã đáp ứng".
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/quality-charts/__tests__/severity-encoding src/renderer/src/components/quality-charts/__tests__/SeverityBadge`.

## Tiêu chí hoàn thành

- [ ] Một nguồn duy nhất; `ChartLegend` và badge không có bản sao bảng.
- [ ] Không màu làm dấu hiệu duy nhất (glyph + nhãn).
- [ ] Không hex; không `max-lines` disable.

## Rủi ro

- Tên icon `lucide-react` có thể khác bản cài; thay bằng icon tương đương gần nhất và cập nhật bảng SOL-088 2.4.
- Va chạm hổ phách với `--review-untested`: bảo đảm hình dạng (tam giác ở góc) khác kiểu viền của overlay (xem SOL-088 6.3).

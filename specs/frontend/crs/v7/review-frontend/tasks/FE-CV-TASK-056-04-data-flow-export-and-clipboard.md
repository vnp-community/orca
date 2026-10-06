# FE-CV-TASK-056-04: Sao chép Mermaid và xuất `.mmd`/`.svg`

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 4.3
**Priority:** P1
**Area:** frontend / review-map
**File:** `components/review-map/data-flow-export.ts` (mới), test
**Depends on:** không
**Status:** [ ] TODO

## Context

- `window.api.ui.writeClipboardText` (`api-types.ts:3216`); mẫu tải tệp `components/settings/mcp/mcp-audit-csv.ts` (`URL.createObjectURL(new Blob(...))` + `<a download>`, tên không `:`); `lib/markdown-review-note-copy.ts` (mẫu clipboard).

## Việc cần làm

1. `copyMermaidSource(source)`; `downloadMermaidSource(source, name)`; `downloadSvgFromContainer(container, name)` lấy `outerHTML` của `svg` trong vùng bọc (đã qua DOMPurify); vô hiệu khi chưa vẽ/lỗi.
2. `dataFlowExportFilename(label, ext, now)`: `dataflow-<slug>-<YYYYMMDD-HHmm>.<ext>`, slug `[a-z0-9-]` ≤ 40, không `:`.

## Kiểm thử

- Tên tệp; nội dung; DOM giả; không `<script>` trong SVG xuất; clipboard mock.

## Tiêu chí hoàn thành

- [ ] Test xanh.

## Rủi ro

- `<a download>` trong Electron đóng gói chưa kiểm ngoài tiền lệ CSV.

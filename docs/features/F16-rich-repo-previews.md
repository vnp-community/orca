# F16 — Rich Repo Previews

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F16 |
| **Tên** | Rich Repo Previews |
| **Ưu tiên** | P2 — Could Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.10 (Rich Repo Previews) |
| **Tham chiếu URD** | UR-062 |
| **Tham chiếu SRS** | FR-8.3 |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Preview tài liệu và media trực tiếp trong Orca mà không cần mở ứng dụng ngoài — Markdown, PDF, ảnh, và Mermaid diagrams đều được render inline.

---

## Tính năng chi tiết

### Markdown Preview
- Render GitHub Flavored Markdown (GFM)
- Tables, code blocks, task lists
- Math equations (KaTeX)
- Mermaid diagrams
- Image embedding
- Table of Contents panel

### Image Viewer
- Display ảnh đúng kích thước
- Zoom in/out
- Pan khi ảnh lớn hơn viewport
- Supported: PNG, JPG, GIF, SVG, WebP

### PDF Viewer
- Render PDF inline
- Page navigation
- Zoom control
- Powered by `pdfjs-dist`

### Diagram Support (trong Markdown)
- Mermaid: flowcharts, sequence diagrams, Gantt charts
- Được render tự động trong Markdown preview

---

## Tiêu chí chấp nhận

- [ ] Markdown render đúng GFM trong < 500ms
- [ ] PDF mở và render đúng
- [ ] Ảnh hiển thị đúng kích thước và tỷ lệ
- [ ] Mermaid diagram render chính xác

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Markdown** | `react-markdown`, `remark-gfm`, `rehype-highlight` |
| **PDF** | `pdfjs-dist` v5.7.284 |
| **Math** | `katex`, `remark-math`, `rehype-katex` |
| **Mermaid** | `mermaid` v11.15.0 |
| **Rich markdown** | `src/shared/rich-markdown-context-menu.ts` |
| **TOC panel width** | `src/shared/markdown-toc-panel-width.ts` |

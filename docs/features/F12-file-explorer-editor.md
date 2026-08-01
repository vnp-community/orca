# F12 — File Explorer & Editor

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F12 |
| **Tên** | File Explorer & Editor |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.10 (File Explorer + Editor) |
| **Tham chiếu URD** | UR-060, UR-061 |
| **Tham chiếu SRS** | FR-8.1, FR-8.2, FR-8.3 |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

VSCode-style editor và file explorer tích hợp trong Orca, với autosave và khả năng drag files / images thẳng vào agent prompt.

---

## Vấn đề cần giải quyết

Developer cần chỉnh sửa file trực tiếp trong Orca mà không cần mở editor riêng. Ngoài ra, việc chia sẻ file với agent thường đòi hỏi copy/paste nội dung — kéo thả file vào prompt đơn giản hơn nhiều.

---

## Tính năng chi tiết

### Monaco Editor

- **Engine**: Monaco Editor (VSCode engine)
- Syntax highlighting cho 50+ ngôn ngữ phổ biến
- **Autosave**: tự động lưu sau 1 giây idle
- Multi-cursor editing (Ctrl+click)
- Find and replace (hỗ trợ regex)
- Breadcrumb navigation
- Code folding
- Go to definition (khi có language server)

### File Explorer

- Tree view của file system theo worktree
- **Git status indicators**: modified (M), added (A), deleted (D), untracked (?)
- File/folder: create, rename, delete, move
- Context menu: right-click operations
- Collapse/expand directories
- File type icons

### Drag File to Agent Prompt

**Supported file types:**
- Text files: đọc content, đính kèm inline trong prompt
- Images: encode base64, đính kèm dưới dạng image block
- Folders: list file tree
- PDF: trích xuất text (nếu có thể)

**Limits:**
- Max file size: 10MB per file
- Max total per prompt: 50MB

### Rich Repo Previews

- **Markdown**: render với full GFM (tables, code blocks, math)
- **Images**: display đúng kích thước
- **PDF**: render inline
- **Mermaid diagrams**: render trong Markdown

---

## Luồng người dùng

```
[Chỉnh sửa file]
1. Click file trong File Explorer
2. File mở trong editor tab
3. Chỉnh sửa → tự động lưu sau 1 giây
4. Git status badge update ngay lập tức

[Drag file vào agent]
5. Kéo "schema.sql" từ File Explorer
6. Thả vào agent chat box
7. Orca đính kèm content vào prompt
8. Người dùng thêm "Giải thích schema này"
9. Agent đọc schema và giải thích

[Drag image]
10. Kéo "screenshot.png" từ desktop
11. Thả vào agent chat
12. Image được đính kèm dưới dạng base64
13. Agent xem ảnh và phân tích
```

---

## Tiêu chí chấp nhận

- [ ] Editor mở file trong < 500ms
- [ ] Autosave hoạt động sau 1 giây idle
- [ ] Git status cập nhật ngay sau khi lưu
- [ ] Drag text file vào prompt đính kèm content đúng
- [ ] Drag image vào prompt agent nhận được ảnh
- [ ] Markdown preview render đúng với GFM

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Editor** | `@monaco-editor/react` v4.7.0, `monaco-editor` v0.55.1 |
| **File drop** | `src/shared/native-file-drop.ts` |
| **Editor save events** | `src/shared/editor-save-events.ts` |
| **Image extensions** | `src/shared/image-file-extensions.ts` |
| **Markdown renderer** | `react-markdown`, `remark-gfm`, `rehype-highlight` |
| **PDF viewer** | `pdfjs-dist` v5.7.284 |
| **Math rendering** | `katex`, `remark-math`, `rehype-katex` |
| **Mermaid** | `mermaid` v11.15.0 |

---

## Metrics

| KPI | Target |
|----|-------|
| File open time | < 500ms |
| Autosave delay | 1 giây |
| Max drag-drop file size | 10MB |

# F08 — Annotate AI Diffs

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F08 |
| **Tên** | Annotate AI Diffs |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.7 |
| **Tham chiếu URD** | UR-041 |
| **Tham chiếu SRS** | FR-6.2 |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Xem diff của code mà AI agent tạo ra, thêm comment trực tiếp trên từng dòng, và gửi toàn bộ feedback về agent để sửa — tất cả mà không cần rời Orca.

---

## Vấn đề cần giải quyết

Sau khi AI agent tạo code, developer cần review và phản hồi. Hiện tại phải copy/paste output từ terminal, viết feedback bằng lời (dễ mơ hồ, thiếu context dòng code cụ thể), rồi paste lại vào agent. Annotate AI Diffs cho phép comment trực tiếp trên diff với context đầy đủ.

---

## Tính năng chi tiết

### Diff Viewer

- **Unified diff** và **split diff** view
- Syntax highlighting theo ngôn ngữ (50+ ngôn ngữ)
- **Large diff handling**: render giới hạn, collapse hunks lớn
- Hunk navigation (scroll tới thay đổi tiếp theo)
- File tree với badge số lượng thay đổi
- **Binary diff**: thông báo rõ ràng cho file binary

### Inline Annotation

- Click vào bất kỳ dòng code nào → mở comment box
- Comment có thể được gắn vào:
  - Dòng cụ thể (single line)
  - Range of lines (multi-line selection)
- Hỗ trợ markdown trong comment
- Review nhiều comment trước khi gửi

### Feedback to Agent

- Tập hợp tất cả comments thành structured prompt
- Format gửi về agent bao gồm:
  - File path
  - Line number(s)
  - Original code (context)
  - Comment text
- Gửi về agent terminal (inject vào PTY)
- Agent xử lý và cập nhật code

### Review Flow

- **Mark as reviewed**: track dòng đã review
- **Commit after review**: tạo commit sau khi review xong
- Integration với GitHub PR review (gửi comments lên GitHub)

---

## Luồng người dùng

```
[Xem agent diff]
1. Agent hoàn thành task
2. Click "Review Changes" → diff viewer mở
3. Browse các file thay đổi

[Annotate]
4. Thấy dòng "if (user.age > 18)" thiếu null check
5. Click vào dòng → textbox xuất hiện
6. Nhập: "Cần check user !== null trước"
7. Tiếp tục review, thêm comment ở các dòng khác

[Gửi feedback]
8. Click "Send to Agent"
9. Orca format: "File: auth.ts, Line 42: Cần check user !== null trước..."
10. Agent nhận, sửa code, tạo diff mới
11. Diff viewer refresh với thay đổi mới

[Commit]
12. Hài lòng với kết quả → click "Commit"
13. AI generate commit message
14. Review và submit
```

---

## Tiêu chí chấp nhận

- [ ] Click vào dòng diff để thêm comment (< 1 click)
- [ ] Comment được gửi về agent với đầy đủ context (file, line, code)
- [ ] Agent nhận và điều chỉnh code dựa trên comment
- [ ] Diff viewer hiển thị đúng với large files (> 1000 dòng thay đổi)
- [ ] Syntax highlighting hoạt động cho TypeScript, Python, Go, Rust

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Diff format** | `src/shared/diff-comments-format.ts` |
| **Large diff limit** | `src/shared/large-diff-render-limit.ts` |
| **Review steps** | `src/shared/review-steps.ts` |
| **Source control AI** | `src/shared/source-control-ai.ts` (~46K bytes) |
| **Hosted review** | `src/shared/hosted-review.ts` |
| **PR review lines** | `src/main/github/pr-review-comment-lines.ts` |

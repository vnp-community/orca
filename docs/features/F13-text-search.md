# F13 — Text Search

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F13 |
| **Tên** | Text Search |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.10 (Text Search) |
| **Tham chiếu SRS** | FR-8 |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Tìm kiếm text toàn bộ workspace — bao gồm tất cả worktrees và files — với hỗ trợ regex, case sensitivity, và file filter.

---

## Tính năng chi tiết

### Search Options
- **Literal text search**: tìm chính xác
- **Regex search**: hỗ trợ regular expressions
- **Case sensitive / insensitive**
- **Whole word matching**
- **Include/Exclude patterns**: glob patterns cho file filter

### Scope
- Tìm trong tất cả worktrees đang open
- Tìm trong worktree cụ thể
- Tìm trong file đang mở (Ctrl+F)
- Exclude: node_modules, .git, build outputs (mặc định)

### Results
- Group theo file
- Show match count per file và total
- Click để navigate tới match
- Preview context (2 dòng trước/sau)
- Highlight match trong editor khi navigate

### Replace
- Search & replace trong file
- Search & replace across all files (với confirm dialog)

---

## Tiêu chí chấp nhận

- [ ] Search hoàn thành trong < 2 giây với 10K files
- [ ] Regex search hoạt động chính xác
- [ ] Replace across files có confirm dialog trước khi thực hiện
- [ ] Kết quả cập nhật real-time khi gõ

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Search engine** | `src/shared/text-search.ts` (~17K bytes) |
| **Search match count** | `src/shared/search-match-count.ts` |

---

## Metrics

| KPI | Target |
|----|-------|
| Search time (10K files) | < 2 giây |
| First result time | < 200ms |

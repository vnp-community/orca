# F10 — Quick Open

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F10 |
| **Tên** | Quick Open |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.10 (Quick Open) |
| **Tham chiếu URD** | UR-062 |
| **Tham chiếu SRS** | FR-10 |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Universal search và command palette cho phép tìm kiếm worktrees, files, agents, commands, và repo context theo fuzzy matching — tất cả trong một ô tìm kiếm, không cần rời bàn phím.

---

## Vấn đề cần giải quyết

Với nhiều worktrees, files, và agents chạy cùng lúc, việc navigate giữa chúng trở nên phức tạp. Developer cần cách nhanh nhất để tìm đúng worktree, mở đúng file, hoặc chạy đúng command mà không cần nhớ vị trí chính xác.

---

## Tính năng chi tiết

### Search Scope

| Loại | Ví dụ | Shortcut filter |
|------|-------|----------------|
| **Worktrees** | "fix-login-v2", "feature/auth" | `@` prefix |
| **Files** | "user.ts", "auth/service.ts" | Mặc định |
| **Agents** | "Claude Code (running)", "Codex" | `>` prefix |
| **Commands** | "Create Worktree", "Toggle Terminal" | `>` prefix |
| **Repo symbols** | Functions, classes, interfaces | `#` prefix |

### Fuzzy Matching

- Fuzzy search với scoring (ưu tiên exact match → prefix → subsequence)
- Highlight matched characters trong result
- Sort theo relevance score + recency

### File Discovery

- **Readdir walk**: traverse filesystem của tất cả worktrees
- **Git-aware**: collapse `.git` directories, respect `.gitignore`
- **Incremental**: không block UI khi đang walk
- Cache kết quả, invalidate khi file system thay đổi

### Navigation

- `↑↓` để chọn result
- `Enter` để open
- `Esc` để đóng
- Preview panel cho file result (snippet xung quanh match)

---

## Luồng người dùng

```
1. Nhấn Cmd+K (macOS) / Ctrl+K (Windows/Linux)
2. Quick Open dialog xuất hiện

[Tìm file]
3. Gõ "useauth" → thấy "src/hooks/useAuth.ts"
4. Enter → mở file trong editor

[Tìm worktree]
5. Gõ "@fix" → thấy "fix-login-v2 (Claude Code running)"
6. Enter → switch sang worktree đó

[Chạy command]
7. Gõ ">new wor" → thấy "Create Worktree"
8. Enter → mở dialog tạo worktree
```

---

## Tiêu chí chấp nhận

- [ ] Quick Open mở trong < 50ms sau khi nhấn shortcut
- [ ] Kết quả xuất hiện trong < 100ms khi gõ
- [ ] File search không block UI
- [ ] Fuzzy matching tìm đúng kết quả với typo nhỏ
- [ ] Worktree, agent, command search hoạt động đúng

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Filter logic** | `src/shared/quick-open-filter.ts` |
| **File walk** | `src/shared/quick-open-readdir-walk.ts` |
| **Expansion paths** | `src/shared/quick-open-expansion-paths.ts` |
| **Git dir collapse** | `src/shared/quick-open-git-directory-collapse.ts` |
| **Directory validation** | `src/shared/quick-open-directory-validation.ts` |

---

## Metrics

| KPI | Target |
|----|-------|
| Open dialog | < 50ms |
| First results | < 100ms |
| File index size | Unlimited (lazy walk) |

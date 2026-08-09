# F01 — Parallel Worktrees

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F01 |
| **Tên** | Parallel Worktrees |
| **Ưu tiên** | P0 — Must Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.1 |
| **Tham chiếu URD** | UR-001, UR-002, UR-003 |
| **Tham chiếu SRS** | FR-1.1, FR-1.2, FR-1.3, FR-1.4 |
| **ADR References** | — |
| **HLD References** | C3.1, C4.1 |

---

## Mô tả

Tạo nhiều **git worktree độc lập**, mỗi worktree chạy một AI agent riêng với cùng prompt ban đầu. Người dùng có thể fan-out một prompt tới N agent, xem và so sánh kết quả, rồi merge worktree tốt nhất vào nhánh chính.

---

## Vấn đề cần giải quyết

Khi phát triển với AI agent, developers thường chỉ chạy một agent tại một thời điểm, không biết liệu cách tiếp cận của agent đó có phải là tốt nhất hay không. Việc thử nhiều cách song song trước đây đòi hỏi phải tự tạo nhiều branch, mở nhiều terminal, theo dõi thủ công — rất mất thời gian và dễ nhầm lẫn.

---

## Tính năng chi tiết

### Tạo Worktree
- Tạo worktree mới từ nhánh hoặc commit cụ thể
- Auto-generate tên worktree từ timestamp hoặc task name
- Mỗi worktree có terminal và file explorer riêng biệt hoàn toàn

### Fan-out Prompt
- Gửi cùng một prompt tới 1–10 agent cùng lúc
- Mỗi agent làm việc trong worktree cô lập, không ảnh hưởng nhau
- Theo dõi tiến trình từng agent trong thời gian thực

### So sánh và Merge
- Xem diff giữa các worktree để so sánh kết quả
- Chọn worktree thắng và merge vào nhánh chính
- Cleanup tự động hoặc thủ công các worktree không cần nữa

### Sparse Checkout Presets
- Giới hạn checkout của worktree mới xuống một tập thư mục con (Git sparse-checkout), hữu ích với monorepo lớn
- Lưu preset (tên + danh sách thư mục repo-relative) theo từng repo, chọn lại khi tạo worktree tiếp theo
- Validate path: chặn đường dẫn tuyệt đối và `..` để tránh checkout ngoài repo

### Remote Worktrees
- Hỗ trợ worktree trên SSH host (remote machine)
- Cùng trải nghiệm như worktree local

### Safety
- Cảnh báo trước khi xóa worktree có uncommitted changes
- Phục hồi graceful khi thư mục worktree bị xóa từ bên ngoài

---

## Luồng người dùng

```
1. Người dùng nhập prompt
2. Chọn "Fan-out" → chọn số lượng (ví dụ: 3)
3. Orca tạo 3 worktree mới (từ cùng base branch)
4. Mỗi worktree tự động khởi động agent và inject prompt
5. Người dùng theo dõi 3 agent chạy song song
6. So sánh kết quả → chọn worktree tốt nhất
7. Merge worktree đó vào nhánh chính
8. Cleanup 2 worktree còn lại
```

---

## Tiêu chí chấp nhận

- [ ] Người dùng tạo được worktree mới từ nhánh bất kỳ trong < 30 giây
- [ ] Có thể tạo 5+ worktree cùng lúc mà không giảm hiệu năng đáng kể
- [ ] Không có data corruption giữa các worktree độc lập
- [ ] Cảnh báo rõ ràng trước khi xóa worktree có uncommitted changes
- [ ] Worktree orphaned (thư mục bị xóa ngoài) hiển thị trạng thái "orphaned", không crash

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Git command** | `git worktree add <path> <base-ref>` |
| **Persistence** | SQLite: metadata (id, path, branch, created_at) |
| **Removal safety** | Check uncommitted changes, running processes trước khi xóa |
| **File** | `src/main/repo-worktrees.ts`, `src/shared/worktree-id.ts` |
| **Safety logic** | `src/main/worktree-removal-safety.ts` |
| **Recovery** | `src/main/local-worktree-removal-recovery.ts` |
| **Sparse checkout presets UI** | `src/renderer/src/components/sparse/SparseCheckoutPresetSelect.tsx`, `SparseCheckoutPresetDraftForm.tsx` |
| **Sparse checkout validation** | `src/main/ipc/sparse-checkout-directories.ts` |

---

## Phụ thuộc

- Git 2.25+ (baseline cho `git worktree` operations)
- `GitCapabilityCache` cho version-specific behavior

---

## Metrics

| KPI | Target |
|----|-------|
| Worktree creation time | < 30 giây |
| Max concurrent worktrees | ≥ 10 |
| Data isolation | 100% (zero cross-contamination) |

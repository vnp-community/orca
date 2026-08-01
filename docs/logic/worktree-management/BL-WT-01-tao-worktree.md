# BL-WT-01 — Tạo Worktree

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-WT-01 |
| **Tên** | Tạo Worktree |
| **Nhóm** | Worktree Management |
| **Actors** | Alex (Senior Dev), Maya (Tech Lead), Carlos (Remote Dev) |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F01 Parallel Worktrees |
| **SRS** | FR-1.1 |

---

## Mô tả nghiệp vụ

Nghiệp vụ cho phép người dùng tạo một git worktree mới — một bản sao làm việc độc lập của repository — để AI agent có thể làm việc trong môi trường cô lập mà không ảnh hưởng đến nhánh chính hay các worktree khác.

---

## Tiền điều kiện (Preconditions)

- Repository git đã được mở trong Orca
- Git 2.25+ đã được cài đặt
- Người dùng đã đăng nhập và có quyền truy cập repo
- Disk còn đủ không gian (> 100MB)

---

## Luồng chính (Main Flow)

```
1. Người dùng click "New Worktree" trong sidebar
2. Hệ thống hiển thị dialog:
   - Chọn base branch (default: main)
   - Nhập tên worktree (optional, default: auto-generate)
   - Chọn agent type (optional)
3. Người dùng xác nhận
4. Hệ thống thực thi:
   a. Validate đường dẫn đích không xung đột
   b. Chạy: git worktree add <path> <base-ref>
   c. Tạo database record (id, path, branch, created_at)
   d. Khởi tạo terminal PTY trong worktree
   e. Thêm worktree card vào sidebar
5. Worktree sẵn sàng, terminal active
```

---

## Luồng thay thế

**[A1] Tên worktree đã tồn tại:**
- Hệ thống hiển thị lỗi "Path already exists"
- Gợi ý tên thay thế
- Người dùng chọn tên khác hoặc hủy

**[A2] Base branch không tồn tại:**
- Hệ thống báo lỗi: "Branch 'xyz' not found"
- Gợi ý danh sách branches có sẵn

**[A3] Disk không đủ:**
- Hệ thống cảnh báo dung lượng disk
- Gợi ý xóa worktrees cũ

---

## Hậu điều kiện (Postconditions)

- Thư mục worktree được tạo trên filesystem
- Git worktree được đăng ký (`git worktree list` hiển thị)
- Database record được tạo
- Sidebar hiển thị worktree card mới
- Event `worktree:created` được phát

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-WT-01 | Tên worktree không được chứa ký tự đặc biệt (chỉ a-z, 0-9, -, _) |
| BR-WT-02 | Đường dẫn worktree phải nằm ngoài thư mục `.git` |
| BR-WT-03 | Không được tạo worktree trùng path với worktree đang tồn tại |
| BR-WT-04 | Maximum 20 worktrees đồng thời trên cùng repository |

---

## Dữ liệu

**Input:**
```typescript
{
  baseRef: string;        // branch name hoặc commit SHA
  name?: string;          // optional, auto-generated nếu không có
  path?: string;          // optional, default: ../repo-<name>
  agentType?: AgentType;  // optional, mặc định theo project config
}
```

**Output:**
```typescript
{
  id: WorktreeId;         // UUID
  path: string;           // absolute path trên filesystem
  branch: string;         // tên branch
  baseRef: string;        // base branch/SHA
  createdAt: Date;
  status: 'ready';
}
```

---

## SLO (Service Level Objective)

| Metric | Target |
|--------|--------|
| Thời gian tạo worktree | < 30 giây |
| Success rate | > 99% |
| Rollback khi fail | < 5 giây |

# BL-TM-01 — Tạo PTY Session

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-TM-01 |
| **Tên** | Tạo PTY Session |
| **Nhóm** | Terminal Management |
| **Actors** | Alex, Maya, Carlos |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F02 Terminal Splits |
| **SRS** | FR-3.1 |

---

## Mô tả nghiệp vụ

Tạo pseudo-terminal (PTY) session để chạy shell hoặc agent process — cung cấp môi trường terminal đầy đủ với I/O pipes, signal handling, và terminal dimensions.

---

## Luồng chính

```
1. Trigger từ: tạo worktree, mở terminal mới, hoặc khởi động agent
2. Hệ thống:
   a. Detect platform (macOS/Linux: POSIX PTY, Windows: ConPTY)
   b. Create PTY với kích thước ban đầu (cols x rows từ UI)
   c. Spawn shell process trong PTY:
      - macOS/Linux: $SHELL hoặc /bin/bash
      - Windows: PowerShell hoặc cmd.exe
      - WSL: bash via git-bash bridge
   d. Attach xterm.js renderer
   e. Setup I/O pipes (data, resize, close events)
3. PTY sẵn sàng nhận input
```

---

## Hậu điều kiện

- PTY process đang chạy
- Terminal renderer attached và hiển thị
- Resize handler active
- Cleanup handler đăng ký

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-TM-01 | PTY phải được cleanup khi tab đóng — không zombie process |
| BR-TM-02 | PTY resize phải được propagate tới process ngay lập tức |
| BR-TM-03 | PTY phải hỗ trợ 256-color và true color |
| BR-TM-04 | Shell path phải được resolve từ $SHELL env, không hardcode |

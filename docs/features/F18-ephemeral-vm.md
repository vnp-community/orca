# F18 — Ephemeral VM

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F18 |
| **Tên** | Ephemeral VM |
| **Ưu tiên** | P2 — Could Have |
| **Trạng thái** | 🚧 Đang phát triển |
| **Tham chiếu PRD** | §3.10 (Ephemeral VM) |
| **ADR References** | — |
| **HLD References** | C3.1 |

---

## Mô tả

Chạy AI agent trong Virtual Machine tạm thời (ephemeral) — VM được tạo tự động, agent làm việc trong môi trường cô lập hoàn toàn, và VM bị xóa sau khi xong.

---

## Vấn đề cần giải quyết

Một số tác vụ cần môi trường sạch hoàn toàn (không có state từ session trước), hoặc cần thực thi code không an toàn trong sandbox. Ephemeral VM cung cấp môi trường cô lập cho từng task.

---

## Tính năng chi tiết

### VM Lifecycle
- **Create**: tạo VM theo recipe (OS, packages, environment)
- **Run**: agent làm việc trong VM
- **Snapshot**: lưu trạng thái VM (optional)
- **Destroy**: xóa VM sau khi xong

### Recipe System
- Recipe định nghĩa: OS image, packages, env vars, setup scripts
- YAML-based recipe format
- Recipe validation và doctor (check conflicts)

### Runtime Options
- **SSH-based**: VM được access qua SSH
- **Container**: Docker/OCI container (alternative)

### Integration với Worktrees
- Ephemeral VM worktree: worktree được mount trong VM
- Kết quả được copy ra sau khi VM xong
- Port forwarding từ VM về local

---

## Tiêu chí chấp nhận

- [ ] VM được tạo từ recipe trong < 60 giây
- [ ] Agent chạy trong VM giống như local
- [ ] VM bị destroy tự động sau khi task xong
- [ ] Recipe validation phát hiện conflicts trước khi tạo VM

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Recipe types** | `src/shared/ephemeral-vm-recipes.ts` |
| **Recipe runner** | `src/shared/ephemeral-vm-recipe-runner.ts` |
| **Recipe doctor** | `src/shared/ephemeral-vm-recipe-doctor.ts` |
| **Runtime store** | `src/shared/ephemeral-vm-runtime-store.ts` |
| **Runtime service** | `src/main/ephemeral-vm-runtime-service.ts` |
| **SSH integration** | `src/main/ephemeral-vm-runtime-ssh.ts` |
| **Setup terminal** | `src/shared/ephemeral-setup-terminal-worktree-id.ts` |

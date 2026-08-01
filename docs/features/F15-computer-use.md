# F15 — Computer Use

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F15 |
| **Tên** | Computer Use |
| **Ưu tiên** | P2 — Could Have |
| **Trạng thái** | 🚧 Đang phát triển |
| **Tham chiếu PRD** | §3.10 (Computer Use) |
| **Tham chiếu SRS** | FR-9.2 (CLI integration) |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Cho phép AI agent điều khiển desktop UI thực tế — click, type, scroll, và tương tác với bất kỳ ứng dụng nào — khi workflow cần real interaction với GUI.

---

## Vấn đề cần giải quyết

Một số workflow cần tương tác với ứng dụng không có API (legacy apps, desktop tools). Hiện tại developer phải tự làm thủ công. Computer Use cho phép agent thực hiện thay thế.

---

## Tính năng chi tiết

### Screenshot-based Control
- Agent chụp screenshot của màn hình
- Agent phân tích screenshot và quyết định action
- Agent gửi action (click, type, scroll) về Orca
- Orca thực thi action qua native APIs

### Supported Actions
- **Click**: click tọa độ (x, y) hoặc element description
- **Type**: gõ text vào focused element
- **Scroll**: scroll theo hướng và lượng
- **Key**: gửi keyboard shortcut
- **Screenshot**: chụp màn hình hiện tại

### Safety
- Yêu cầu explicit user approval để bật Computer Use
- Chỉ hoạt động khi user đã grant quyền
- Log tất cả actions để audit

### Integration với CLI
- `orca click <selector>` — CLI interface cho computer use
- `orca fill <selector> <value>` — fill form field
- `orca snapshot` — chụp màn hình

### Platform Support
- macOS: Accessibility API, screen capture
- Linux: X11/Wayland utilities
- Windows: Win32 APIs

---

## Tiêu chí chấp nhận

- [ ] Agent có thể click vào element bằng coordinate
- [ ] Agent có thể type text vào input field
- [ ] Screenshot được chụp đúng vùng màn hình
- [ ] User phải approve trước khi Computer Use hoạt động
- [ ] Tất cả actions được log đầy đủ

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Computer main** | `src/main/computer/` |
| **Key spec** | `src/shared/computer-use-key-spec.ts` |
| **Runtime types** | `src/shared/computer-use-runtime-types.ts` |
| **Error recovery** | `src/shared/computer-use-error-recovery.ts` |
| **Permissions** | `src/shared/computer-use-permissions-types.ts` |
| **macOS binary** | `build:computer-macos` |

# F02 — Terminal Splits

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F02 |
| **Tên** | Terminal Splits |
| **Ưu tiên** | P0 — Must Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.2 |
| **Tham chiếu URD** | UR-010, UR-011, UR-012 |
| **Tham chiếu SRS** | FR-3.1, FR-3.2, FR-3.3, FR-3.4 |
| **ADR References** | — |
| **HLD References** | C3.1 |

---

## Mô tả

Terminal tích hợp hiệu năng cao tương đương Ghostty, hỗ trợ chia màn hình vô hạn, scrollback buffer bền vững qua restart, và shell integration đầy đủ.

---

## Vấn đề cần giải quyết

Developers sử dụng Orca cần xem output của nhiều agent cùng lúc. Các terminal app riêng biệt (iTerm2, Windows Terminal) không tích hợp với workflow agent, không biết context của worktree, và không lưu được scrollback khi đóng app.

---

## Tính năng chi tiết

### Rendering
- **WebGL rendering** qua `@xterm/addon-webgl` cho hiệu năng GPU-accelerated
- Fallback về Canvas rendering nếu WebGL không khả dụng
- 256-color và true color (24-bit) support
- Unicode và emoji rendering chính xác
- **Ligature support** qua `@xterm/addon-ligatures`
- Clickable URL links (OSC hyperlinks)

### Split Layout
- Chia terminal theo chiều ngang (horizontal) và chiều dọc (vertical)
- Vô hạn số lần split
- Resize pane bằng cách kéo border
- Multi-tab terminal
- Mỗi split/tab chạy process riêng độc lập

### Scrollback Persistence
- Scrollback buffer lưu vào SQLite khi đóng app
- Khôi phục đầy đủ output từ session trước
- Cursor position và attributes được restore
- Buffer size có thể cấu hình (mặc định: unlimited theo config)

### Shell Integration (OSC 133)
- Detect OSC 133 A/B/C/D sequences
- Track command start/end, exit code
- Hiển thị exit code trong UI
- PowerShell auto-bootstrap script

### Keyboard Support
- Kitty keyboard protocol
- Modifier key detection (Shift, Ctrl, Alt, Meta)
- Platform-aware shortcut labels (⌘ macOS, Ctrl Windows/Linux)

---

## Tiêu chí chấp nhận

- [ ] Terminal khởi động trong < 1 giây
- [ ] Typing latency < 16ms (keydown → screen update)
- [ ] Không freeze khi output > 10,000 dòng/giây
- [ ] Scrollback restore chính xác sau khi restart
- [ ] Hỗ trợ bash, zsh, fish, PowerShell
- [ ] Copy/paste hoạt động bình thường

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **PTY library** | `node-pty` v1.1.0 |
| **Terminal library** | `@xterm/xterm` 6.1.0-beta |
| **WebGL addon** | `@xterm/addon-webgl` |
| **Serialize addon** | `@xterm/addon-serialize` |
| **Platform: macOS/Linux** | POSIX PTY |
| **Platform: Windows** | ConPTY (Windows 10+) |
| **Platform: WSL** | bash bridge via `git-bash.ts` |
| **Persistence** | `src/main/terminal-history.ts`, `src/main/terminal-scrollback-snapshots.ts` |

---

## Metrics

| KPI | Target |
|----|-------|
| Typing latency | < 16ms |
| Frame rate | ≥ 60fps |
| Scrollback limit | Configurable (default unlimited) |
| Startup time | < 1 giây |

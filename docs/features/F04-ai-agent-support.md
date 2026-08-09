# F04 — AI Agent Support

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F04 |
| **Tên** | AI Agent Support |
| **Ưu tiên** | P0 — Must Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.9 |
| **Tham chiếu URD** | UR-004, UR-005 |
| **Tham chiếu SRS** | FR-2.1, FR-2.2, FR-2.3, FR-2.4, FR-2.5 |
| **ADR References** | — |
| **HLD References** | C3.1, C3.8 |

---

## Mô tả

Orca hỗ trợ chạy **bất kỳ CLI agent nào** — "if it runs in a terminal, it runs in Orca." Với 30+ agent được tích hợp sâu (first-class support), người dùng có thể orchestrate nhiều agent từ nhiều nhà cung cấp khác nhau trong cùng một workspace.

---

## Vấn đề cần giải quyết

Thị trường AI agent đang phát triển nhanh chóng với hàng chục agent từ các nhà cung cấp khác nhau. Developer không muốn bị lock-in vào một agent, muốn thử và so sánh kết quả từ nhiều agent, và cần quản lý session, usage, rate limits của tất cả từ một nơi.

---

## Agents được tích hợp sâu (First-class)

| Agent | Nhà cung cấp | Đặc điểm tích hợp |
|-------|-------------|------------------|
| **Claude Code** | Anthropic | Session resume, usage tracking, sub-agent roster |
| **Codex** | OpenAI | Usage tracking, startup delivery |
| **GitHub Copilot** | GitHub | Auth integration |
| **Grok** | xAI | Session path lookup |
| **Cursor** | Cursor | Binary detection |
| **OpenCode** | OpenCode.ai | Usage tracking |
| **Gemini / Antigravity** | Google | Native integration |
| **Pi** | Pi.ai | Overlay UI settings |
| **Amp** | AmpCode | CLI support |
| **Devin** | Cognition | CLI integration |
| **Goose** | Block | CLI integration |
| **Cline** | Cline.bot | CLI support |
| **Continue** | Continue.dev | CLI support |
| **Qwen Code** | Alibaba | CLI support |
| **MiMo Code** | Xiaomi | CLI support |
| **Hermes Agent** | Nous Research | Startup query |
| **Kiro** | Kiro.dev | CLI integration |
| **Kimi Code** | Moonshot AI | Hook install qua `config.toml` (không có JSON settings, Orca quản lý bằng marker-delimited TOML block) |
| **MiniMax** | MiniMax | Session cookie riêng (mã hoá bằng `safeStorage`, lưu tại `~/.orca/minimax-session-cookie.enc`) |
| **OpenClaude** | OpenClaude | Dùng lại `ClaudeHookService` (Claude-compatible hook settings) |
| **Command Code** | Command Code | Hook install qua `~/.commandcode/settings.json`, tự khôi phục biến môi trường từ tiến trình cha khi CLI strip env |
| **Droid** | Factory | Hook install qua `~/.factory/settings.json`, hỗ trợ `PermissionRequest` riêng cho approval prompt |
| + 10 agent khác | Various | Generic CLI |

> **Native chat transcript reader** (`src/main/native-chat/`) không phải một provider — đây là hạ tầng đọc/parse transcript dùng chung cho Claude Code, Codex, và Grok (session file resolver, transcript decoders theo từng format, tail-follow watcher).

---

## Tính năng chi tiết

### Auto-Detection
- Tự động scan PATH cho các binary đã biết
- Version check cho mỗi binary phát hiện
- Hiển thị danh sách agent khả dụng trong UI

### Session Management
- **Session resume**: tiếp tục session đã có sau khi restart
  - Claude Code: `--resume <session-id>`
  - Codex: session file path
  - OpenCode: session recovery command
- Session ID lưu trong persistence store

### Usage Tracking & Rate Limits
- Theo dõi usage theo từng provider (tokens, requests)
- Hiển thị rate limit status và thời gian reset
- Badge cảnh báo khi gần rate limit
- **Account switcher**: hot-swap accounts khi bị rate limit

### Trust Presets (Permission Management)
- `minimal` — read-only, không chạy lệnh
- `standard` — read/write files, chạy lệnh an toàn
- `trusted` — full permissions
- Remote agent có trust presets riêng biệt

### Agent Hooks
- Hook vào lifecycle events của agent
- OSC parsing để detect agent status
- Process recognition (phân biệt agent process với process thường)
- Mỗi agent có `HookService` riêng (`getStatus` / `install` / `installRemote` / `remove`) ghi một managed script + block hook vào config của agent (JSON `settings.json` hoặc TOML tuỳ agent), đánh dấu bằng marker để không đụng vào hook do người dùng tự thêm
- `installRemote()` ghi cùng hook qua SFTP lên SSH host — cần thiết vì agent CLI (vd. Droid, Kimi) chạy trên Dev Server, không phải trên máy Orca Desktop
- Managed script POST kết quả hook về `http://127.0.0.1:<ORCA_AGENT_HOOK_PORT>/hook/<agent>` kèm token xác thực (`X-Orca-Agent-Hook-Token`)

### Agent Startup Configuration
- Agent-specific environment variables
- Custom startup commands
- Default agent per-project (via `orca.yaml`)
- Shell selection per-agent

---

## Luồng người dùng

```
1. Orca auto-detect Claude Code, Codex đã cài trên máy
2. Người dùng tạo worktree mới
3. Chọn agent (Claude Code) → Orca khởi động trong PTY
4. Agent chạy trong terminal, Orca parse status tự động
5. Khi rate limit → Orca thông báo → người dùng switch sang Codex
6. Session được resume sau khi restart app
```

---

## Tiêu chí chấp nhận

- [ ] Tự động phát hiện agent đã cài trong < 2 giây sau khi mở app
- [ ] Agent được khởi động trong < 5 giây khi tạo worktree
- [ ] Session resume hoạt động với Claude Code, Codex, OpenCode
- [ ] Usage tracking hiển thị đúng, reset đúng thời điểm
- [ ] Trust presets được áp dụng và không thể bypass

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Agent config** | `src/shared/tui-agent-config.ts` |
| **Detection** | `src/shared/agent-detection.ts` |
| **Process recognition** | `src/shared/agent-process-recognition.ts` |
| **Session resume** | `src/shared/agent-session-resume.ts` |
| **Trust presets** | `src/main/agent-trust-presets.ts` |
| **Claude usage** | `src/main/claude-usage/` |
| **Codex usage** | `src/main/codex-usage/` |
| **Hooks** | `src/main/hooks.ts` |
| **Agent awake** | `src/main/agent-awake-service.ts` |
| **Hook installer (dùng chung)** | `src/main/agent-hooks/installer-utils.ts`, `installer-utils-remote.ts` |
| **Kimi hooks** | `src/main/kimi/hook-service.ts`, `kimi-hook-config-toml.ts` |
| **MiniMax cookie store** | `src/main/minimax/minimax-cookie-store.ts` |
| **OpenClaude hooks** | `src/main/openclaude/hook-service.ts` (bọc `ClaudeHookService`) |
| **Command Code hooks** | `src/main/command-code/hook-service.ts`, `command-code-managed-script.ts` |
| **Droid hooks** | `src/main/droid/hook-service.ts` |
| **Transcript reader dùng chung** | `src/main/native-chat/` |

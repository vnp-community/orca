# Software Requirements Specification (SRS)

**Sản phẩm:** Orca — AI Orchestrator IDE  
**Phiên bản tài liệu:** 2.0  
**Ngày:** 2026-07-21 | **Cập nhật:** 2026-08-01  
**Phiên bản sản phẩm:** 5.0 (Profile Hierarchy + AI Provider + Workflow + Task Graph + Project Workspace + Full-Flow Tracing)  
**Chuẩn áp dụng:** IEEE 830-1998 (adapted)  
**Tài liệu liên quan:** PRD.md, URD.md  

---

## 1. Giới thiệu

### 1.1 Mục đích

Tài liệu SRS này mô tả các **yêu cầu phần mềm** — cả chức năng và phi chức năng — cho hệ thống Orca AI Orchestrator IDE. Tài liệu phục vụ cho:
- Nhóm phát triển để implement đúng yêu cầu
- QA để xây dựng test plan
- Stakeholders để xác nhận phạm vi hệ thống

### 1.2 Phạm vi hệ thống

Orca bao gồm các thành phần:

| Thành phần | Mô tả |
|-----------|---------|
| **Orca Desktop** | Ứng dụng Electron đa nền tảng (macOS/Windows/Linux) |
| **Orca Web Server** | Node.js server mode (không cần Electron), hỗ trợ multi-user qua HTTP/WebSocket |
| **Orca Mobile** | Companion app iOS/Android |
| **Orca CLI** | Command-line interface |
| **Orca Relay** | Binary relay cho SSH remote execution |
| **Orca Daemon** | Background service cho IPC |

### 1.3 Định nghĩa và thuật ngữ

| Thuật ngữ | Định nghĩa |
|-----------|-----------|
| **Worktree** | Git worktree độc lập, mỗi cái có working directory riêng |
| **Agent** | CLI-based AI coding assistant (Claude Code, Codex, v.v.) |
| **Session** | Một phiên làm việc của agent trong một worktree |
| **Auth Session** | Phiên đăng nhập của user (cookie `orca_session`, 8h TTL) |
| **UserId** | Unique ID của user trong Orca Server (UUID) |
| **ORCA_MULTI_USER** | Biến môi trường bật chế độ multi-user (=1 để kích hoạt) |
| **Relay** | Binary được deploy trên SSH host để truyền thông tin |
| **PTY** | Pseudo-terminal (terminal ảo) |
| **IPC** | Inter-Process Communication (giao tiếp liên tiến trình) |
| **OSC** | Operating System Command (escape sequence trong terminal) |
| **Fan-out** | Gửi cùng prompt tới nhiều agent cùng lúc |
| **Diff** | Sự khác biệt giữa hai phiên bản code |
| **Pairing** | Kết nối mobile app với desktop app |
| **DSN** | Data Source Name — cỗi kết nối database (`dialect://user:pass@host/db`) |
| **Fleet** | Tập hợp các dev servers được quản lý tập trung |
| **AgentToken** | Token xác thực Agent WebSocket connection |
| **DevServerId** | Unique ID của dev server đăng ký trong Orca Web Server |
| **WebCredentialStore** | Per-user AES-256-GCM credential storage trong Orca Server |
| **OrcaProfile** | Profile object gồm 6 sections (agent, editor, shell, mcp, security, envVars) |
| **ProfileResolver** | Component resolve profile bằng deep-merge Company←Dept←User |
| **AIProviderAccount** | Account đăng ký một AI provider trên một dev server cụ thể |
| **WorkflowTemplate** | Định nghĩa YAML của một workflow (có thể kế thừa từ parent template) |
| **WorkflowExecution** | Một lần chạy cụ thể của một workflow (có state, inputs, outputs) |
| **OrcaTask** | Task node trong Task Graph (có graph edges và grants) |
| **TaskGrant** | Phân quyền trên task (scope × permission) |
| **WorkspaceContext** | Central state của Project Workspace (project, relay, profile, gitStatus) |
| **RelayConnectionPool** | Singleton quản lý SSH relay connections (reuse per dev server) |
| **TraceEvent** | Structured event phát ra bởi span (id, flow, level, fields, ts, elapsedMs) |
| **TraceSpan** | Handle được tạo bởi `createTracer().start()` — gọi step/ok/fail |
| **Tracer** | Factory object tạo span cho một flow cụ thể |
| **TraceSink** | Callback nhận mọi TraceEvent — có thể đăng ký nhiều sink cùng lúc |
| **TraceFlow** | Tên định danh luồng theo format `subsystem:operation` (ví dụ: `devServer:browseDir`) |

### 1.4 Tổng quan tài liệu

Tài liệu được cấu trúc theo:
- Section 2: Mô tả hệ thống tổng thể
- Section 3: Yêu cầu chức năng chi tiết
- Section 4: Yêu cầu phi chức năng
- Section 5: Yêu cầu giao diện
- Section 6: Ràng buộc thiết kế

---

## 2. Mô tả hệ thống tổng thể

### 2.1 Góc nhìn hệ thống

```
┌─────────────────────────────────────────────────────────────┐
│                      Orca Desktop App                        │
│  ┌──────────┐  ┌─────────────┐  ┌───────────────────────┐  │
│  │ Renderer │  │ Main Process│  │      Preload          │  │
│  │ (React)  │←→│ (Node.js)   │←→│ (Context Bridge)      │  │
│  └──────────┘  └──────┬──────┘  └───────────────────────┘  │
│                        │                                     │
│              ┌─────────┼──────────┐                         │
│              ↓         ↓          ↓                         │
│         ┌────────┐ ┌───────┐ ┌──────────┐                  │
│         │ SQLite │ │  PTY  │ │   Git    │                   │
│         └────────┘ └───────┘ └──────────┘                  │
└──────────────────────────┬──────────────────────────────────┘
                           │ SSH / WebSocket
          ┌────────────────┴────────────────┐
          ↓                                 ↓
┌─────────────────┐               ┌──────────────────┐
│  Orca Relay     │               │  Orca Mobile     │
│  (Remote Host)  │               │  (iOS/Android)   │
└─────────────────┘               └──────────────────┘
```

### 2.2 Chức năng hệ thống

Orca thực hiện các chức năng chính:
1. **Quản lý git worktree** — tạo, xóa, theo dõi worktree
2. **Orchestrate AI agent** — khởi động, theo dõi, dừng agent
3. **Terminal emulation** — PTY management, rendering, persistence
4. **SSH remote execution** — kết nối, relay, port forwarding
5. **Source control integration** — GitHub, GitLab, Linear, v.v.
6. **Mobile companion** — pairing, notification, control
7. **Browser automation** — Design Mode, Computer Use

### 2.3 Đặc tính người dùng

Xem URD.md Section 2 (User Personas).

### 2.4 Ràng buộc chung

- Ứng dụng phải hoạt động offline (trừ tính năng cần AI API)
- Git binary phải được cài đặt riêng bởi người dùng
- AI agent CLI phải được cài đặt riêng bởi người dùng
- Node.js 24 là runtime bắt buộc

---

## 3. Yêu cầu chức năng

### FR-1: Quản lý Worktree

#### FR-1.1: Tạo Worktree

**Tham chiếu URD:** UR-001  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống cho phép tạo git worktree mới từ nhánh hoặc commit cụ thể.

**Đầu vào:**
- Base branch/commit SHA
- Tên worktree (tùy chọn, auto-generate từ timestamp hoặc task name)
- Đường dẫn đích (mặc định: thư mục song song với repo)

**Xử lý:**
1. Validate đường dẫn đích không xung đột
2. Chạy `git worktree add <path> <base-ref>`
3. Tạo database record với metadata (id, path, branch, created_at)
4. Khởi tạo workspace session
5. Tạo worktree card trong UI

**Đầu ra:**
- Worktree được tạo và hiển thị trong sidebar
- Terminal mới trong worktree sẵn sàng
- Sự kiện `worktree:created` được phát

**Điều kiện lỗi:**
- Nếu đường dẫn đích đã tồn tại → báo lỗi, hỏi người dùng
- Nếu base branch không tồn tại → báo lỗi rõ ràng
- Nếu disk không đủ chỗ → báo lỗi với gợi ý dọn dẹp

---

#### FR-1.2: Fan-out Prompt

**Tham chiếu URD:** UR-002  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống tạo N worktree đồng thời và gửi cùng prompt tới N agent.

**Đầu vào:**
- Prompt text
- Số lượng worktree (1-10)
- Agent type (mặc định: cấu hình của project)
- Base branch

**Xử lý:**
1. Tạo N worktree theo FR-1.1
2. Với mỗi worktree, khởi động agent theo FR-2.3
3. Inject prompt vào mỗi agent terminal
4. Theo dõi trạng thái mỗi agent độc lập

**Đầu ra:**
- N worktree được tạo và hiển thị
- N agent đang chạy với prompt giống nhau
- Giao diện so sánh các worktree

---

#### FR-1.3: Xóa Worktree (Safe Removal)

**Tham chiếu URD:** UR-001  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống xóa worktree an toàn, tránh mất dữ liệu.

**Safety checks trước khi xóa:**
1. Kiểm tra uncommitted changes (git status)
2. Kiểm tra untracked files
3. Kiểm tra agent có đang chạy không
4. Kiểm tra process nào đang dùng thư mục

**Điều kiện lỗi:**
- Nếu có uncommitted changes → hiện dialog cảnh báo với tùy chọn:
  - Discard and delete
  - Commit changes first
  - Cancel
- Nếu agent đang chạy → yêu cầu dừng agent trước

**Hậu xử lý:**
- Chạy `git worktree remove --force <path>`
- Xóa database record
- Cleanup terminal sessions

---

#### FR-1.4: Worktree Removal Recovery

**Ưu tiên:** Should Have  

**Mô tả:** Khi thư mục worktree bị xóa từ ngoài hệ thống (filesystem), Orca phải phục hồi gracefully.

**Xử lý:**
1. Phát hiện thư mục worktree bị missing
2. Hiển thị trạng thái "orphaned" trên worktree card
3. Cung cấp tùy chọn cleanup reference trong git
4. Không crash hay hang khi missing worktree

---

### FR-2: Quản lý AI Agent

#### FR-2.1: Phát hiện Agent

**Tham chiếu URD:** UR-004  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống tự động phát hiện các CLI agent đã cài đặt.

**Xử lý:**
1. Scan PATH cho các binary đã biết (claude, codex, gemini, cursor, v.v.)
2. Thực hiện version check cho mỗi binary phát hiện
3. Load agent configuration từ `src/shared/tui-agent-config.ts`
4. Hiển thị danh sách agent khả dụng trong UI

**Agent detection patterns** (từ `agent-detection.ts`):
- Phát hiện qua process name
- Phát hiện qua binary path lookup
- Phát hiện qua environment variables

---

#### FR-2.2: Khởi động Agent

**Tham chiếu URD:** UR-004  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống khởi động agent trong PTY của worktree tương ứng.

**Đầu vào:**
- Agent type (claude, codex, v.v.)
- Working directory (worktree path)
- Startup command (từ agent config)
- Environment variables
- Trust preset (permissions level)

**Xử lý:**
1. Resolve agent binary path
2. Áp dụng trust preset settings
3. Tạo PTY session
4. Spawn agent process trong PTY
5. Setup hooks (OSC parsing, status monitoring)
6. Monitor agent startup sequence

**Trạng thái agent:**
- `idle` — chưa có prompt
- `running` — đang xử lý
- `waiting` — chờ user input
- `completed` — xong task
- `error` — lỗi

---

#### FR-2.3: Session Resume

**Tham chiếu URD:** UR-004  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống cho phép tiếp tục session agent từ lần trước.

**Xử lý:**
1. Load session ID từ persistence store
2. Truyền resume flag/session ID vào agent startup command
3. Agent tự khôi phục context từ session ID

**Hỗ trợ theo agent:**
- Claude Code: `--resume <session-id>`
- Codex: session file path
- OpenCode: session recovery command

---

#### FR-2.4: Agent Trust Presets

**Tham chiếu URD:** UR-100  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống cung cấp profile quyền hạn cho agent.

**Preset levels:**
- `minimal` — read-only, không chạy lệnh
- `standard` — read/write files, chạy lệnh an toàn
- `trusted` — full permissions

**Cơ chế:**
- Preset được áp dụng qua environment variables
- Trust settings được lưu per-project
- Remote agent có trust presets riêng (`remote-agent-trust-presets.ts`)

---

#### FR-2.5: Agent Usage Tracking

**Tham chiếu URD:** UR-005  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống theo dõi usage của từng AI provider.

**Dữ liệu tracked:**
- Claude usage (từ `claude-usage` module)
- Codex usage (từ `codex-usage` module)
- OpenCode usage (từ `opencode-usage` module)
- Rate limit status và reset time

**Hiển thị:**
- Badge trên agent icon khi gần rate limit
- Panel usage trong settings
- Thời gian reset rate limit

---

### FR-3: Terminal Emulation

#### FR-3.1: PTY Management

**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống tạo và quản lý PTY sessions.

**Yêu cầu kỹ thuật:**
- Sử dụng `node-pty` để tạo pseudo-terminal
- Hỗ trợ resize (SIGWINCH) khi cửa sổ thay đổi kích thước
- Cleanup PTY process khi đóng tab
- Prevent zombie processes

**Platform specifics:**
- **macOS/Linux**: POSIX PTY
- **Windows**: ConPTY (Windows 10+)
- **WSL**: Sử dụng bash bridge (`git-bash.ts`)

---

#### FR-3.2: Terminal Rendering

**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống render terminal output với hiệu năng cao.

**Yêu cầu:**
- WebGL rendering qua `@xterm/addon-webgl`
- Fallback về Canvas rendering nếu WebGL không khả dụng
- Hỗ trợ 256-color và true color (24-bit)
- Unicode và emoji rendering đúng
- Ligature support qua `@xterm/addon-ligatures`
- OSC link ranges (clickable links)

**Hiệu năng:**
- Frame rate ≥ 60fps khi output bình thường
- Không freeze khi output > 10,000 dòng/giây
- Memory sử dụng: scrollback buffer tối đa cấu hình được

---

#### FR-3.3: Terminal Persistence

**Tham chiếu URD:** UR-012  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống lưu và khôi phục scrollback buffer.

**Lưu (serialize):**
- Sử dụng `@xterm/addon-serialize` để serialize terminal state
- Lưu vào SQLite database
- Snapshot được trigger theo sự kiện (close, idle timeout)

**Khôi phục (restore):**
- Load từ SQLite khi mở lại worktree
- Restore cursor position và attributes
- Restore scrollback lên tới configured limit

**Constraints:**
- Scrollback limit có thể cấu hình (mặc định: unlimited theo cấu hình)
- Serialize chỉ khi terminal đang idle

---

#### FR-3.4: Shell Integration (OSC 133)

**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống theo dõi command execution qua OSC 133 sequences.

**Xử lý:**
- Detect OSC 133 A/B/C/D sequences trong output
- Track command start/end
- Hiển thị exit code trong UI
- PowerShell bootstrap script tự động inject (`powershell-osc133-bootstrap.ts`)

---

### FR-4: SSH và Remote Development

#### FR-4.1: SSH Connection

**Tham chiếu URD:** UR-020  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống thiết lập và duy trì kết nối SSH.

**Yêu cầu:**
- Sử dụng `ssh2` library hoặc system SSH binary
- Hỗ trợ xác thực: SSH key, password, SSH agent, keyboard-interactive
- Đọc `~/.ssh/config` bao gồm Include directives
- Hỗ trợ SSH ProxyJump
- Channel multiplexing để tối ưu kết nối

**SSH Config parsing (`ssh-config-parser.ts`):**
- Parse `~/.ssh/config` đầy đủ bao gồm Host patterns
- Expand Include directives
- Resolve tilde trong paths

**Connection flow:**
1. Resolve host config từ SSH config
2. Negotiate auth method (key → agent → password)
3. Thiết lập control channel
4. Deploy relay binary nếu chưa có
5. Mở port forwarding channels

---

#### FR-4.2: SSH Relay

**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống deploy và quản lý Orca relay binary trên remote host.

**Deploy flow:**
1. Check relay version trên remote
2. Nếu outdated hoặc missing → upload binary qua SFTP
3. Verify binary integrity (hash check)
4. Start relay process

**Relay capabilities:**
- Forward terminal I/O
- File system operations
- Git commands
- Port scanning và forwarding

**Versioning:**
- Version mismatch detection (`ssh-relay-version-mismatch-error.ts`)
- Cross-version isolation tests
- Auto-upgrade khi local version mới hơn

---

#### FR-4.3: Auto-Reconnect

**Tham chiếu URD:** UR-021  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống tự động reconnect khi SSH connection bị gián đoạn.

**Xử lý:**
1. Phát hiện mất kết nối (socket close, keepalive timeout)
2. Hiển thị trạng thái "Reconnecting" trong UI
3. Thử reconnect với exponential backoff:
   - 1s → 2s → 4s → 8s → 16s → 30s (max)
4. Khi reconnected: restore terminal state, resume agent

**Agent continuity:**
- Agent process tiếp tục chạy trên remote trong khi mất kết nối
- Buffering output từ remote trong thời gian reconnect
- Flush buffer khi reconnected

---

#### FR-4.4: Port Forwarding

**Tham chiếu URD:** UR-022  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống forward port từ remote host về local.

**Loại forwarding:**
- **Local forward**: local port → remote service
- **Dynamic**: SOCKS proxy

**Auto-detection:**
- Scan cho ports mới mở trên remote (`ssh-port-scanner.ts`)
- Tự động forward khi phát hiện port mới
- Thông báo người dùng với local URL

**Localhost proxy:**
- Label-based routing cho multiple worktrees
- Proxy request tới đúng worktree dựa trên header

---

### FR-5: Mobile Companion

#### FR-5.1: Device Pairing

**Tham chiếu URD:** UR-030  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống kết nối mobile app với desktop app.

**Pairing flow:**
1. Desktop hiển thị QR code (encode: server URL + one-time token)
2. Mobile scan QR
3. Mobile gửi pairing request với token
4. Desktop verify token, tạo session key
5. Trao đổi session key (TweetNaCl key exchange)
6. Kết nối được thiết lập và encrypted

**Security:**
- E2E encryption (TweetNaCl)
- One-time pairing token
- Session key rotation
- Token expiry sau 5 phút

---

#### FR-5.2: Push Notifications

**Tham chiếu URD:** UR-031  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống gửi push notification tới mobile khi có sự kiện.

**Sự kiện trigger:**
- Agent hoàn thành task (idle state)
- Agent gặp lỗi
- Agent yêu cầu user input
- Worktree được tạo/xóa

**Delivery:**
- Thông qua WebSocket connection giữa desktop và mobile
- Fallback: local notification khi app foreground

---

#### FR-5.3: Remote Dispatch

**Tham chiếu URD:** UR-032  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống cho phép mobile gửi prompt tới agent đang chạy trên desktop.

**Flow:**
1. Mobile gửi dispatch request qua encrypted channel
2. Desktop nhận và validate request
3. Desktop inject prompt vào agent terminal
4. Agent xử lý prompt
5. Desktop gửi status update về mobile

---

### FR-6: Source Control Integration

#### FR-6.1: GitHub/GitLab Client

**Tham chiếu URD:** UR-040  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống tích hợp với GitHub/GitLab API để hiển thị PR, issues.

**GitHub features (`src/main/github/client.ts`):**
- Authentication (OAuth, PAT)
- List PRs với filter (open/closed/merged, assignee, label)
- PR detail: title, body, diff, checks, reviews
- Create PR với AI-generated description
- Comment trên PR
- Manage issues

**GitLab features (`src/main/gitlab/`):**
- Similar API surface với GitLab specifics
- Pipeline status
- MR (Merge Request) management

**Provider abstraction:**
- Interface chung cho GitHub và GitLab
- Gitea, Bitbucket, Azure DevOps via separate adapters
- Provider detection từ remote URL

---

#### FR-6.2: Annotate AI Diff

**Tham chiếu URD:** UR-041  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống hiển thị diff của agent changes và cho phép inline comment.

**Diff display:**
- Unified/split diff view
- Syntax highlighting per language
- Large diff handling (render limit theo `large-diff-render-limit.ts`)
- Hunk collapsing

**Annotation flow:**
1. Người dùng click vào dòng diff để mở comment box
2. Nhập comment text
3. Comment được format với context (file, line, original code)
4. Gửi về agent terminal

---

#### FR-6.3: AI Commit Message Generation

**Tham chiếu URD:** UR-043  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống generate commit message từ staged changes.

**Flow (`src/shared/commit-message-agent-spec.ts`):**
1. Collect staged changes (git diff --staged)
2. Collect context: recent commits, branch name
3. Build prompt cho AI model
4. Stream response và display
5. Người dùng có thể edit trước khi commit

**Prompt building:**
- Include file list và diff stats
- Include branch context
- Apply project commit convention (nếu có)

---

#### FR-6.4: Linear Integration

**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống tích hợp với Linear project management.

**Features:**
- List issues với filter (assignee, status, priority)
- Issue detail với description, comments
- Create worktree từ issue
- Update issue status (In Progress → In Review)
- Linear agent access cho AI context

---

### FR-7: Design Mode

#### FR-7.1: Browser Integration

**Tham chiếu URD:** UR-050  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống nhúng browser Chromium và cho phép inspect UI elements.

**Components:**
- Embedded browser view (Electron BrowserView/WebContents)
- Annotation overlay layer
- Element picker mode

**Element capture flow:**
1. Người dùng activate Design Mode
2. Click vào element trong browser
3. Hệ thống capture:
   - Outer HTML của element và ancestors
   - Computed CSS styles
   - Screenshot crop quanh element
4. Data được format thành context string
5. Inject vào agent prompt

---

### FR-8: File Management và Editor

#### FR-8.1: Monaco Editor Integration

**Tham chiếu URD:** UR-060  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống cung cấp code editor đầy đủ tính năng.

**Yêu cầu:**
- Monaco Editor (VSCode engine)
- Syntax highlighting cho 50+ ngôn ngữ
- Autosave sau 1 giây idle
- Multi-cursor editing
- Find and replace (regex support)
- Breadcrumb navigation

---

#### FR-8.2: File Explorer

**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống cung cấp file explorer cho từng worktree.

**Yêu cầu:**
- Tree view của file system
- Git status indicators (modified, added, deleted)
- File/folder create, rename, delete
- Drag file vào agent prompt
- Quick search trong file tree

---

#### FR-8.3: Native File Drop to Agent

**Tham chiếu URD:** UR-061  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống cho phép kéo thả file và ảnh vào agent prompt.

**Xử lý:**
- Drag text file: đọc content và đính kèm inline
- Drag image: encode base64 và đính kèm
- Drag folder: list file tree
- Giới hạn file size: 10MB per file, 50MB per prompt

---

### FR-9: Automation và CLI

#### FR-9.1: Automation Scheduling

**Tham chiếu URD:** UR-070  
**Ưu tiên:** Could Have  

**Mô tả:** Hệ thống cho phép lên lịch chạy automation.

**Trigger types:**
- Cron expression
- Manual trigger
- Event-based (push, PR created)

**Automation actions:**
- Create worktree
- Run agent với prompt cụ thể
- Commit và push kết quả
- Send notification

**Schema (`src/shared/automations-types.ts`):**
- Automation definition
- Run history
- Status tracking

---

#### FR-9.2: Orca CLI

**Tham chiếu URD:** UR-071  
**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống cung cấp CLI tool để scripting và automation.

**Commands:**
```
orca worktree create [--base <branch>] [--agent <type>] [--prompt <text>]
orca worktree list
orca worktree remove <id>
orca agent status [--worktree <id>]
orca snapshot [--worktree <id>]
orca click <selector>
orca fill <selector> <value>
orca serve [--port <port>]
```

**Implementation:**
- TypeScript compiled to Node.js binary
- Unix socket communication với daemon
- JSON output option (`--json`)
- Exit codes convention (0 = success, 1 = error)

---

### FR-9: Observability — Full-Flow Tracing

#### FR-9.1: Core Trace API (Isomorphic)

**Tham chiếu URD:** UR-040b  
**Ưu tiên:** Should Have  
**F-ref:** F40

**Mô tả:** Hệ thống trace isomorphic hoạt động trên cả Node.js (server/main/relay) và browser, cho phép trace mọi thao tác theo span ID nhất quán.

**Đầu vào:**
- `flow`: cỗi định danh luồng (format `subsystem:operation`)
- `fields`: key/value context đính kèm mỗi event

**Span lifecycle:**
1. `tracer.start(fields)` — phát event `start`, trả về `TraceSpan` với `id` ngẫu nhiên
2. `span.step(label, fields)` — phát event `step` với elapsed time
3. `span.ok(fields)` — phát event `ok` với elapsed time (hoạt động thành công)
4. `span.fail(err, fields)` — phát event `fail` với err message + elapsed time

**Quy tắc:**
- `fail` events **luôn được log** bất kể `ORCA_TRACE` flag
- `start`/`step`/`ok` chỉ log khi `ORCA_TRACE=1` (hoặc `localStorage.ORCA_TRACE=1`)
- Sink errors không được phép throw ra (silently caught)

**Condition lỗi:**
- Sink crash: silently swallowed, không ảnh hưởng caller

---

#### FR-9.2: Sink Registry

**Ưu tiên:** Should Have  

**Mô tả:** Hệ thống sink đăng ký cho phép nhiều consumer nhận `TraceEvent` đồng thời.

**Quy tắc:**
- `registerTraceSink(sink)` — đăng ký, trả về cleanup function
- Cleanup gọi `unregister()` khi component unmount — không gây memory leak
- Mỗi sink chạy độc lập: sink A crash không ảnh hưởng sink B
- Sink mặc định: console sink (tích hợp sẵn trong `emit()`)
- Sink tùy chọn: Zustand store sink (browser), SSE broadcast sink (server)

---

#### FR-9.3: Backend SSE Stream

**Ưu tiên:** Should Have  
**Endpoint:** `GET /api/trace-stream`

**Mô tả:** Server-Sent Events endpoint push mọi `TraceEvent` từ backend về browser real-time.

**Xử lý:**
1. Browser `EventSource` kết nối `/api/trace-stream` (auto-reconnect nội bộ)
2. Server đăng ký `TraceSink` global (lazy, chỉ khi có client đầu tiên)
3. Mỗi `TraceEvent` → serialize JSON → broadcast tới mọi client
4. Heartbeat SSE comment `': heartbeat'` mỗi 15 giây
5. Disconnect: cleanup sink nếu không còn client nào

**Auth (thứ tự ưu tiên):**
- `Authorization: Bearer <ORCA_AGENT_API_SECRET>`
- Header `X-Orca-Admin: 1`
- Header `X-Orca-Trace-Client: 1` (browser trace panel)
- Không có secret cấu hình → allow mọi local connection

**Response headers:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
X-Accel-Buffering: no
```

**Điều kiện lỗi:**
- Client nối không authorized → `401 Unauthorized`
- Method không phải GET → `405 Method Not Allowed`

---

### FR-10: Quick Open

**Tham chiếu URD:** UR-062  
**Ưu tiên:** Must Have  

**Mô tả:** Hệ thống cung cấp universal search/command palette.

**Search scope:**
- Worktrees (theo tên, branch, status)
- Files (fuzzy search trong worktrees)
- Agents (theo tên, status)
- Commands (keyboard shortcuts)
- Repo context (symbols, references)

**Implementation (`src/shared/quick-open-filter.ts`):**
- Fuzzy matching với scoring
- Readdir walk cho file discovery
- Git directory collapse
- Keyboard navigation (↑↓ để chọn, Enter để open)

---

## 4. Yêu cầu phi chức năng

### NFR-1: Hiệu năng

#### NFR-1.1: Startup Performance

| Metric | Target | Measurement |
|--------|--------|-------------|
| Cold start to usable UI | < 3s | Từ click icon đến interactive |
| Daemon cold start | < 1s | Từ start đến ready |
| Worktree creation | < 30s | Từ click đến terminal ready |
| Agent startup | < 5s | Từ click đến agent prompt |

#### NFR-1.2: Runtime Performance

| Metric | Target | Measurement |
|--------|--------|-------------|
| Terminal typing latency | < 16ms | keydown → screen update |
| Terminal frame rate | ≥ 60fps | Rolling average |
| UI interaction response | < 100ms | Click → visible response |
| Idle CPU (0 agents) | < 1% | macOS Activity Monitor |
| Idle CPU (1 agent running) | < 5% | macOS Activity Monitor |
| Memory per worktree | < 200MB | RSS per worktree process |

#### NFR-1.3: Terminal Output Performance

- Render 10,000 lines/sec without freezing
- 100,000+ lines scrollback without memory spike
- WebGL rendering offloads to GPU

---

### NFR-2: Độ tin cậy

#### NFR-2.1: Uptime

- App không crash với rate > 0.1% của sessions
- Auto-update thành công rate > 99%
- SSH reconnect success rate > 95%

#### NFR-2.2: Data Integrity

- Không bao giờ xóa worktree có uncommitted changes mà không có xác nhận người dùng
- SQLite database không bị corrupt khi crash
- Git operations atomic (không để repo ở trạng thái half-done)

#### NFR-2.3: Error Recovery

- Crash reporting tự động (với user consent)
- Diagnostic bundle export khi có lỗi
- Graceful degradation khi AI provider không available

---

### NFR-3: Bảo mật

#### NFR-3.1: Credential Security

- Credentials (API keys, SSH keys) được lưu trong OS keychain
- Không log credentials trong bất kỳ file nào
- Memory scrubbing cho sensitive data sau khi dùng

#### NFR-3.2: Communication Security

- Mobile ↔ Desktop: TweetNaCl E2E encryption
- SSH: standard SSH encryption (không custom crypto)
- Desktop ↔ Relay: authenticated WebSocket

#### NFR-3.3: Agent Sandboxing

- Trust presets giới hạn agent permissions
- Không cho phép agent access file ngoài worktree (trừ khi explicitly granted)
- Audit log cho agent actions (file write, command execute)

#### NFR-3.4: Network

- Hỗ trợ HTTP/HTTPS proxy
- Không gửi data ra ngoài khi offline
- Certificate validation không bị bypass

---

### NFR-4: Khả năng bảo trì

#### NFR-4.1: Code Quality

- TypeScript strict mode
- Oxlint linting enforcement
- Max file lines ratchet (không tăng vượt threshold mà không review)
- No vague module names (helpers, utils, common)

#### NFR-4.2: Testability

- Unit tests cho business logic (Vitest)
- E2E tests với Playwright (electron-headless)
- Git binary compatibility test với multiple Git versions
- Performance benchmarks trong CI

#### NFR-4.3: Cross-platform

- Tất cả file path operations sử dụng `path.join` (không hardcode `/` hoặc `\`)
- Platform-specific code sau runtime checks (không compile-time `#if`)
- Keyboard shortcuts: `CmdOrCtrl` cho menu, runtime check cho UI

---

### NFR-5: Khả năng sử dụng

#### NFR-5.1: Onboarding

- Feature wall giới thiệu tính năng cho user mới
- Contextual tours theo workflow (worktree, SSH, mobile)
- Feature tips inline (không modal, không blocking)

#### NFR-5.2: Accessibility

- Keyboard-first navigation toàn app
- Platform-aware shortcut labels (⌘ vs Ctrl)
- Sufficient color contrast (WCAG AA minimum)

#### NFR-5.3: Localization

- Tất cả user-facing string qua i18n system (`i18next`)
- Hỗ trợ: en, zh-CN, ja, ko, es, pt
- RTL layout: không bắt buộc nhưng không block

---

### NFR-6: Khả năng mở rộng

#### NFR-6.1: Agent Extensibility

- Thêm agent mới không cần sửa core code
- Agent config defined trong `tui-agent-config.ts`
- Generic agent hooks interface

#### NFR-6.2: Git Provider Extensibility

- Provider-agnostic code cho source control operations
- Provider-specific logic sau explicit checks (không GitHub-only naming)

#### NFR-6.3: CLI Extensibility

- CLI commands có thể được gọi bởi agent
- Headless dispatch interface cho automation

---

## 5. Yêu cầu giao diện

### 5.1 User Interface

#### 5.1.1 Layout chính

```
┌──────────────────────────────────────────────────────────┐
│ [Repo Icon] [Workspace Name]                    [User]   │ ← Title Bar
├───────┬──────────────────────────────────────────────────┤
│       │                                                  │
│ Side  │              Main Content Area                   │
│ Bar   │  (Worktrees | Terminal | Editor | Browser)       │
│       │                                                  │
│       │                                                  │
├───────┴──────────────────────────────────────────────────┤
│ [Status Bar: Agent Status | Git | Usage]                 │ ← Status Bar
└──────────────────────────────────────────────────────────┘
```

#### 5.1.2 Responsive behavior

- Minimum window size: 800x600
- Sidebar có thể collapse
- Panels resizable bằng drag
- Left sidebar appearance có thể cấu hình

#### 5.1.3 Theme

- Dark mode (default)
- System theme follow
- Custom terminal themes (Warp themes support)
- Color tokens trong `src/renderer/src/assets/main.css`

---

### 5.2 Software Interfaces

#### 5.2.1 Electron IPC

**Main → Renderer messages:**
- Worktree state updates
- Agent status changes
- Terminal data (PTY output)
- Notification events

**Renderer → Main requests:**
- Create worktree
- Start/stop agent
- File operations
- Git operations

#### 5.2.2 SSH Relay Protocol

- WebSocket-based binary protocol
- Framed messages với type + payload
- Authenticated via session token
- Versioned protocol (incompatible versions: error + reconnect)

#### 5.2.3 Mobile ↔ Desktop Protocol

- WebSocket connection (local network)
- E2E encrypted messages (TweetNaCl)
- Message types: events, dispatch, query

#### 5.2.4 External APIs

| API | Auth Method | Rate Limit Handling |
|-----|-------------|---------------------|
| GitHub REST/GraphQL | OAuth, PAT | Exponential backoff, cache |
| GitLab REST | PAT, OAuth | Same |
| Linear SDK | API key | Request batching |
| Jira | Basic/OAuth | On-demand |

---

### 5.3 Hardware Interfaces

- **GPU**: Sử dụng GPU cho WebGL terminal rendering
- **Microphone**: Optional, cho speech input (Sherpa-ONNX)
- **Network**: Bắt buộc cho AI agent và remote features

---

## 6. Ràng buộc thiết kế

### 6.1 Kiến trúc Electron

- Main process: Node.js, không DOM, không UI
- Renderer process: React, không direct Node.js (qua preload bridge)
- Preload: Context bridge, expose limited API
- Worker threads cho heavy computation (file scanning, text search)

### 6.2 Git Compatibility

- Git 2.25 là minimum baseline (xem `docs/reference/git-compatibility.md`)
- `GitCapabilityCache` để cache version-specific behavior
- Không dùng `simple-git` làm source of truth cho version detection
- Capability scope: host-specific (native, WSL, SSH)

### 6.3 Cross-Platform

- Tất cả platform-specific code sau runtime checks
- macOS: Apple Silicon (arm64) và Intel (x64) native binaries
- Windows: ConPTY cho terminal
- Linux: AppImage, .deb distribution

### 6.4 Security Constraints

- Không disable CSP trong renderer
- Không enable `nodeIntegration` trong renderer trực tiếp
- Tất cả Node.js access qua preload context bridge
- IPC messages phải validated

### 6.5 Dependency Constraints

- Node.js 24 runtime
- PNPM package manager
- TypeScript 7+ với strict mode
- Không dùng `any` type (trừ special cases)

---

## 7. Phụ lục

### A. Danh sách module chính

| Module | Location | Mô tả |
|--------|---------|-------|
| `persistence.ts` | `src/main/` | State management toàn app |
| `hooks.ts` | `src/main/` | Agent hook system |
| `agent-awake-service.ts` | `src/main/` | Agent lifecycle management |
| `ssh-connection.ts` | `src/main/ssh/` | SSH connection core |
| `ssh-relay-session.ts` | `src/main/ssh/` | SSH relay session |
| `tui-agent-config.ts` | `src/shared/` | Agent configurations |
| `types.ts` | `src/shared/` | Global type definitions |
| `runtime-types.ts` | `src/shared/` | Runtime environment types |
| `keybindings.ts` | `src/shared/` | Keyboard shortcuts |
| `telemetry-events.ts` | `src/shared/` | Telemetry event catalog |
| `trace/index.ts` | `src/shared/trace/` | Core trace API (createTracer, registerTraceSink) |
| `trace/browser.ts` | `src/shared/trace/` | Browser adapter (initBrowserTrace, SSE client) |
| `trace/tracers.ts` | `src/shared/trace/` | Pre-built Tracers registry |
| `trace-sse-routes.ts` | `src/server/` | GET /api/trace-stream SSE handler |

### B. Testing Requirements

| Test Type | Tool | Coverage Target |
|-----------|------|----------------|
| Unit tests | Vitest | > 70% business logic |
| E2E tests | Playwright | Critical paths |
| SSH integration | Docker | SSH relay scenarios |
| Performance | Custom benchmarks | CI budget gates |
| Git compatibility | Multiple Git versions | Core workflow |

### C. Deployment Artifacts

| Platform | Artifact | Size Target |
|----------|---------|------------|
| macOS arm64 | `.dmg` | < 200MB |
| macOS x64 | `.dmg` | < 200MB |
| Windows | `.exe` (NSIS) | < 200MB |
| Linux | `.AppImage` | < 200MB |
| Linux | `.deb` | < 200MB |
| iOS | `.ipa` (App Store) | < 50MB |
| Android | `.apk` | < 50MB |

### FR-11: Multi-User Authentication (Web Server Mode)

#### FR-11.1: Local Login

**Tham chiếu URD:** UR-110  
**Ưu tiên:** Must Have (ORCA_MULTI_USER=1)  

**Mô tả:** Người dùng đăng nhập bằng email/password qua `POST /auth/local`.

**Xử lý:**
1. Validate email + password field (Zod schema)
2. Lookup user trong `orca_users` table
3. Verify password với bcrypt (12 rounds)
4. Tạo session record trong `orca_sessions` (UUID, 8h TTL)
5. Set `Set-Cookie: orca_session=<token>; HttpOnly; SameSite=Lax`
6. Return `{id, email, name, role}`

**Điều kiện lỗi:**
- Sai password → 401 Unauthorized
- Account inactive → 403 Forbidden
- Tắt `ORCA_MULTI_USER` → 404 (endpoint không có)

---

#### FR-11.2: Session Middleware

**Ưu tiên:** Must Have  

**Mô tả:** `requireAuth()` guard cho `/admin/api/*` và WebSocket upgrade.

**Xử lý:**
1. Extract `orca_session` cookie từ request
2. Validate session trong `orca_sessions` (not expired, not revoked)
3. Update `last_seen_at` timestamp
4. Inject `userId` + `role` vào request context
5. Nếu invalid → 401 (HTTP) hoặc close WS connection

---

#### FR-11.3: Auth Endpoints

**Ưu tiên:** Must Have  

| Endpoint | Mô tả |
|----------|---------|
| `POST /auth/local` | Login bằng email + password |
| `POST /auth/logout` | Invalidate session |
| `GET /auth/me` | Trả về current user info |
| `GET /auth/sso/:provider` | SSO stub (Phase 2) |
| `GET /login` | Trả về Login SPA |

---

### FR-12: Per-User Process Sandbox

#### FR-12.1: Session Manager

**Tham chiếu URD:** UR-111  
**Ưu tiên:** Must Have  

**Mô tả:** Mỗi user được cấp phát một Node.js process độc lập (fork).

**Tính năng:**
- `SessionManager.getOrCreate(userId)` → fork nếu chưa có
- Idle timeout: 4h không có WS connection
- Spawn timeout: 30s → error
- Max respawn: 3 lần trước khi abandon
- Per-user `userDataPath`: `~/.orca/users/<userId>/`
- Unix socket: `~/.orca/users/<userId>/orca.sock`

#### FR-12.2: WebSocket Session Router

**Mô tả:** Proxy WS connections tới đúng user process.

**Xử lý:**
1. WS connection mở → extract userId từ session middleware
2. `WsSessionRouter.route(userId, ws)` → khớp nối qua Unix socket
3. Bi-directional proxy giữa browser WS và user process socket
4. Dọn dẹp khi WS close

---

### FR-13: Admin Panel

#### FR-13.1: Admin REST API

**Tham chiếu URD:** UR-112  
**Ưu tiên:** Must Have  

| Endpoint | Method | Mô tả |
|----------|--------|---------|
| `/admin/api/users` | GET | List users (filter role, status) |
| `/admin/api/users` | POST | Tạo user mới |
| `/admin/api/users/:id` | GET/PATCH/DELETE | User CRUD |
| `/admin/api/sessions` | GET | List active sessions |
| `/admin/api/sessions/:id` | DELETE | Kill session |
| `/admin/api/audit` | GET | Audit log (filter by action, user) |
| `/admin/api/stats` | GET | Tổng số users, sessions, uptime |

**Guard:** `requireAdmin` middleware → 403 nếu role != `admin`.

#### FR-13.2: Audit Log

**Mô tả:** Ghi mọi action quan trọng vào `orca_audit_log`.

**Events được ghi:**
- `login.success`, `login.fail`
- `user.create`, `user.deactivate`
- `session.kill`
- `ssh.connect` (với `linuxUser` field)

#### FR-13.3: First-Run Setup

**Mô tả:** Khi khởi động lần đầu (no users in DB) → tự động tạo admin user, in credentials ra stdout.

---

### FR-14: Multi-Database Support

#### FR-14.1: Database Provider Abstraction

**Tham chiếu URD:** UR-113  
**Ưu tiên:** Must Have  

**Interfaces:**
- `IDatabase` — `run()`, `get()`, `all()`, `exec()`, `close()`
- `ISyncDatabase` extends `IDatabase` — synchronous (SQLite desktop)
- `IAsyncDatabase` extends `IDatabase` — async (MySQL/PostgreSQL)
- `IConnectionPool` — `acquire()`, `release()`, `stats()`
- `DatabaseProvider` — factory interface (`connect(config)`)

**Dialects hỗ trợ:**

| Dialect | Driver | Mục đích |
|---------|--------|----------|
| SQLite | `node:sqlite` built-in | Desktop, default |
| MySQL 8.x | `mysql2` | Production server |
| PostgreSQL 14+ | `pg` | Production server |
| TiDB | `mysql2` | MySQL-compatible |
| MariaDB | `mysql2` | MySQL-compatible |

#### FR-14.2: Migration Framework

**Mô tả:** `MigrationRunner` — apply/rollback/status, idempotent, cross-dialect.

**Migrations hiện tại:**
- 0001: initial schema (worktrees, sessions, automations)
- 0002: add automations
- 0003: add workspace sessions
- 0004: orca app tables
- 0005: auth schema (`orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies`)

#### FR-14.3: DSN Configuration

**Mô tả:** `ORCA_DB_URL=<dsn>` (env hoặc YAML config).

**DSN format:** `dialect://user:pass@host:port/database?param=value`

**Config loader priority:** env var → config YAML → default (SQLite)

#### FR-14.4: Health Endpoints

| Endpoint | Mô tả |
|----------|---------|
| `GET /health` | Basic health (always 200 nếu process alive) |
| `GET /health/ready` | Ready check (DB connected, migrations done) |
| `GET /health/metrics` | Prometheus format metrics |

---

### FR-15: Fleet Management

#### FR-15.1: Fleet Inventory Config

**Tham chiếu URD:** UR-120  
**Ưu tiên:** Must Have  

**Mô tả:** `deploy/dev/orca-fleet.yaml` — khai báo fleet as-code.

**Config tính năng:**
- `servers[]`: list servers với hostname, user, identityFile, project, tags
- `projects[]`: nhóm server theo project/team
- `defaults`: relay grace period, Node.js version

**Import flow:**
1. `orca fleet import --file orca-fleet.yaml`
2. Parse YAML → validate Zod schema
3. Upsert vào SSH targets store
4. Return import summary

#### FR-15.2: Bulk Provisioning

**Mô tả:** `orca fleet provision [--project <name>]` — provision nhiều server cùng lúc.

**Xử lý:** Tạo SSH connection → deploy relay → bootstrap Node.js + Git → update status.

#### FR-15.3: Fleet Health Monitoring

**Mô tả:** `FleetHealthMonitor` — poll mỗi server theo interval (mặc định 60s).

**Status:** `healthy` | `degraded` | `unhealthy` | `unreachable`

**Metrics:** CPU%, RAM%, disk%, SSH latency — exposed qua Prometheus `/metrics`.

---

### FR-16: Dev Server Onboarding Wizard

**Tham chiếu URD:** UR-121  
**Ưu tiên:** Must Have  

**Wizard steps:**
1. **Connect Dev Server** — SSH credentials, test connection
2. **Detect Platform** — OS, arch, available tools
3. **Detect AI Agents** — scan PATH cho claude, codex, gemini...
4. **Preflight Check** — `gh`, `git`, disk space, ports
5. **Deploy Relay** — upload binary, verify hash, start relay
6. **Register** — lưu `DevServer` record vào persistence
7. **Multi-Server Checklist** — per-server setup tracking

---

### FR-17: Agent WebSocket Protocol

#### FR-17.1: relay-websocket Mode

**Tham chiếu URD:** UR-130  
**Ưu tiên:** Must Have  

**Mô tả:** Orca kết nối tới Agent WS Server (agent chạy WS server).

**Flow:**
1. Orca đọc `relayWebsocketUrl` từ DevServer config
2. Orca gửi HTTP Upgrade với `Authorization: Bearer <agentToken>`
3. Sau handshake → `SshChannelMultiplexer` wired qua `WsTransport`
4. Full JSON-RPC over binary frames

#### FR-17.2: direct-websocket Mode

**Tham chiếu URD:** UR-131  
**Ưu tiên:** Must Have  

**Mô tả:** Agent kết nối vào Orca WS Server (agent là WS client).

**Flow:**
1. Agent connect `ws://orca:6768/agent`
2. Agent gửi `agent.handshake { agentToken, name, version }`
3. Orca validate token → trả `handshake-ok { sessionId }`
4. Multiplexer wired → full JSON-RPC

#### FR-17.3: Wire Protocol

**Frame format:** 13-byte binary header:
```
[TYPE: 1 byte][SEQ: 4 bytes BE][ACK: 4 bytes BE][LEN: 4 bytes BE]
[PAYLOAD: LEN bytes UTF-8 JSON]
```

**Types:** `DATA=0`, `ACK=1`, `KEEPALIVE=2`, `CLOSE=3`

---

### FR-18: Remote Source Control Integrations

#### FR-18.1: CLI-Based Integrations (GitHub/GitLab)

**Tham chiếu URD:** UR-132  
**Ưu tiên:** Must Have  

**Mô tả:** Proxy `preflight.check` tới Dev Server qua SSH relay.

**GitHub flow:**
1. Renderer gửi `preflight.check { devServerId: "ds-abc" }`
2. Backend proxy request tới relay handler trên Dev Server
3. Relay chạy `gh --version`, `gh auth status` với `GH_CONFIG_DIR=~/.config/gh/<userId>/`
4. Kết quả merge qua `mergePreflightStatuses()`

**Auth flow (`gh auth login`):**
- `github.startAuthLogin()` RPC → mở PTY trên Dev Server
- Frontend render terminal output qua `WebModeCliAuthSection`

#### FR-18.2: API Token Integrations

**Mô tả:** `WebCredentialStore` — AES-256-GCM per-user credential storage.

**RPC API:**
- `credentials.set(service, token)` → encrypt + store
- `credentials.get(service)` → decrypt + return
- `credentials.delete(service)`
- `credentials.list()` → service names only (no tokens)

**Integrations:** Bitbucket, Azure DevOps, Gitea, Linear, Jira

---

*Tài liệu này được cập nhật dựa trên codebase Orca v4.1 và tuân thủ chuẩn IEEE 830-1998 (adapted) — 2026-07-28.**

---

### FR-19: User Profile Hierarchy

#### FR-19.1: OrcaProfile Schema & 3-tầng Inheritance

**Tham chiếu URD:** UR-140  
**Ưu tiên:** Must Have  
**F-ref:** F33

**Mô tả:** Hệ thống profile 3 tầng: Company profile → Department profile → User profile. `ProfileResolver` deep-merge theo thuật toán ưu tiên User > Dept > Company.

**Yêu cầu chức năng:**
- `OrcaProfile` gồm 6 sections: `agent`, `editor`, `shell`, `mcp`, `security`, `envVars`
- Deep-merge: `envVars` dùng override merge, `pathAdditions` dùng concatenation
- `security` và `approvedModels` bị lock ở Company level — child không override được
- Cache resolve TTL 60s, invalidate khi parent profile update
- CRUD API cho Company/Dept/User profile (permission: admin quản lý company, lead quản lý dept)
- Source attribution: mỗi field trong effective profile có metadata `source: 'company'|'dept'|'user'`

#### FR-19.2: Profile Editor UI

**Mô tả:** Editor 3 tầng — user thấy inherited values từ parent, có thể override ở tầng của mình.

**Yêu cầu:**
- Company Profile Editor — chỉ admin
- Department Profile Editor — admin/lead
- User Profile Editor — chính user
- Effective Profile Panel: hiển thị merged result với source badges

---

### FR-20: Project-Dev Server Binding

**Tham chiếu URD:** UR-141  
**Ưu tiên:** Must Have  
**F-ref:** F34

**Mô tả:** Mỗi Project gắn với một dev server cụ thể. `ProjectServerRouter` tự động route mọi hoạt động đến đúng server.

**Yêu cầu chức năng:**
- `orca_projects` table: `id`, `name`, `dev_server_id` (required), `repo_path`, `default_branch`, `visibility`
- `ProjectService`: CRUD + membership management
- `ProjectServerRouter`: resolve `project.devServerId` trước khi spawn agent/worktree/terminal
- `ProfileAwareAgentSpawner`: inject `resolvedProfile` + `ORCA_PROJECT_ID` + `ORCA_MODEL` vào agent env
- `git.worktree.add` thực hiện trên dev server của project
- Project members: roles `owner`, `member`, `viewer`

---

### FR-21: AI Provider Account Management

**Tham chiếu URD:** UR-142  
**Ưu tiên:** Must Have  
**F-ref:** F35

**Yêu cầu chức năng:**
- Providers hỗ trợ: Anthropic, OpenAI, Google Gemini, Azure OpenAI, AWS Bedrock, Ollama, vLLM
- Account scopes: `server` | `project` | `user`
- Credential storage: AES-256-GCM trên Dev Server tại `~/.orca/ai-providers/<accountId>.enc`, KHÔNG lưu trên Orca Server
- Encryption: `masterKey = scrypt(ORCA_AI_CREDENTIAL_KEY + ':' + accountId, accountId)`
- Test connection bắt buộc trước khi save account
- Health check background mỗi 15 phút qua relay
- Quota tracking per account per day (`orca_provider_usage` table)
- Alert khi quota > 80% hoặc key invalid
- Key rotation với 30s grace period

**Priority resolution khi spawn agent:**
1. Explicit accountId
2. Scoped ref string (`server:`, `project:`, `user:`)
3. Model-based auto-detect + cascade: user-scope > project-scope > server-default

---

### FR-22: Multi-Server Workflow Orchestration

**Tham chiếu URD:** UR-143  
**Ưu tiên:** Should Have  
**F-ref:** F36

**Yêu cầu chức năng:**

**FR-22.1: Workflow Definition**
- YAML/JSON schema với fields: `id`, `name`, `version`, `steps[]`, `template_id`, `visibility`
- Step types: `agent`, `shell`, `action`, `webhook`, `parallel`, `condition`
- Server spec per step: `project:<id>` | `server:<id>` | `fleet:tag:<tag>`
- Provider spec per step: `server:anthropic-default` | `project:<id>:<provider>` | `{ model: '...' }`
- Variable interpolation: `{{inputs.*}}`, `{{outputs.<stepId>.*}}`, `{{project.*}}`, `{{user.*}}`

**FR-22.2: Execution Engine**
- DAGBuilder: build adjacency graph từ `depends_on`, topological sort, wave planning
- WorkflowOrchestrator: execute waves in parallel (Promise.allSettled cho parallel type)
- State persistence: `orca_workflow_executions` + `orca_step_executions` (resumable)
- Stream real-time: `step.output`, `step.completed`, `execution.completed` events qua WebSocket

**FR-22.3: Template System**
- Scopes: `company` | `team` | `personal`
- Inheritance: `parent_template_id` + `overrides` (field patches) + `inject_steps` + `remove_steps`
- Visibility: `private` | `team` | `company` | `public` (share token)
- Company scope: admin approval required nếu requester là lead

---

### FR-23: Task Graph Management

**Tham chiếu URD:** UR-144  
**Ưu tiên:** Must Have  
**F-ref:** F37

**Yêu cầu chức năng:**

**FR-23.1: Graph Data Model**
- `orca_tasks`: đủ fields (type, status, priority, labels, parent_id, assignee, reporter, estimate, prompt_template, ai_context, visibility)
- `orca_task_edges`: dependency edges (from, to, edge_type: depends_on/blocks/relates_to)
- `orca_task_grants`: grant (scope, team_id/user_id, permission, apply_tree, expires_at)
- Cycle detection bằng BFS trước khi INSERT dependency edge
- Auto-block: task status → `blocked` nếu dependency chưa `done`

**FR-23.2: AI Planning**
- AI decompose: gửi `task.title + description + aiContext + project tech stack` → JSON subtask suggestions
- AI prompt generation từ task metadata
- Tech stack detection từ dev server files (package.json, go.mod, pom.xml...)
- Critical path calculation từ `estimatedHours` + dependency graph

**FR-23.3: Access Control**
- Grant resolution: owner > admin > user-scope > team-scope > company-scope
- apply_tree: grant propagates đến tất cả descendants (resolved at query time, không denormalize)
- Share link: public token, view-only, no login required
- Grant expiry: `expires_at` field, expired grants bị ignore

**FR-23.4: Agent Execution from Task**
- Build task preamble: title + description + aiContext + parent context + completed dep outputs
- `promptTemplate` interpolation: `{{task.*}}`, `{{project.*}}`, `{{worktree.*}}`
- Auto-advance task.status → `review` khi agent complete
- Stream PTY output → Task Activity Feed (WebSocket)
- Record `actual_hours` từ agent session duration

---

### FR-24: Project Workspace

**Tham chiếu URD:** UR-145  
**Ưu tiên:** Must Have  
**F-ref:** F38

**Yêu cầu chức năng:**

**FR-24.1: WorkspaceContext**
- Initialize khi user chọn project: load project + server + profile + gitStatus + worktrees
- RelayConnectionPool: singleton per dev server, reuse, cleanup idle > 5min
- Offline mode: banner + cached file tree + disable write operations
- Git status background poll: 5s khi Git tab active hoặc agent running
- Cross-panel event bus: `agent.complete`, `git.commit`, `worktree.switched`

**FR-24.2: Remote File Explorer**
- Lazy-load directories: depth=1 per `relay.call('fs.readDir')` expand
- Git status decorations: inline color badges (M=yellow, A=green, D=red, ?=grey)
- File viewer: read content qua relay, syntax highlight, max 5MB
- File search: `fs.glob` (by name) + `fs.grep` (by content) qua relay, limit 50/30 results
- Context menu: copy path, open in terminal (cd), git actions (stage/discard)

**FR-24.3: Workspace Terminal**
- PTY session trên dev server của project (relay)
- Default `cwd` = current worktree path
- Auto-update `cwd` khi worktree switch

---

### FR-25: Remote Git UI

**Tham chiếu URD:** UR-146  
**Ưu tiên:** Must Have  
**F-ref:** F39

**Yêu cầu chức năng:**
- Git status: parse `git status --porcelain=v2`, poll 5s khi tab active
- Visual diff: unified format, syntax highlighted, per file
- Stage/Unstage: individual file hoặc tất cả (`git add` / `git restore --staged`)
- Discard changes: `git restore` với confirm dialog
- Commit: manual message + AI generate (từ staged diff qua LLM)
- Push/Pull: progress stream (line-by-line output qua WebSocket)
- Conflict detection: scan `U` status sau pull → list conflict files → AI resolve agent
- Branch management: list (local + remote), create, checkout, delete (`git branch`), merge (--no-ff)
- Stash: push + pop
- Git log: last 50 commits, `--oneline --graph --decorate`
- PR creation: GitHub CLI (Category A) hoặc API token via WebCredentialStore (Category B)
- AI PR description: generate từ `git diff <base>` + `git log`
- Worktree switcher: `git worktree list` / `git worktree add` / switch

---

*Tài liệu này được cập nhật dựa trên codebase Orca v5.0 và tuân thủ chuẩn IEEE 830-1998 (adapted) — Full-Flow Tracing added (2026-08-01).*

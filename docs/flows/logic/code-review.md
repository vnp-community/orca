# Luồng Dữ liệu — Code Review

**Domain:** Code Review  
**Nghiệp vụ:** BL-CR-01 → BL-CR-05  
**Kiến trúc thực tế:** Orca là Electron Desktop App (Main Process + Renderer) hoặc Web Server mode  
**Cập nhật:** 2026-08-01

> **⚠️ Lưu ý về Dev Server Integration:**  
> Khi worktree nằm trên **Remote Dev Server**, tất cả Git CLI operations phải đi qua `relay.call('git.*')` → Dev Server.  
> HLD ban đầu chỉ mô tả local flow với `child_process.execFile`. Cần bổ sung remote path.

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Renderer (React UI) / Browser | UI | Diff viewer, annotation pane, PR form |
| Main Process / Orca Server | Business Logic | DiffService, GitService, GitHubService |
| Dev Server (Relay) | Remote Execution | git diff/commit/push trên remote repo |
| Git Binary (local hoặc remote) | External | git diff, log, add, commit, push |
| GitHub/GitLab REST API | External | PR creation, review submission |
| AI Agent (PTY / remote PTY) | External | Commit message generation |
| SQLite Database | Persistence | Annotation records |

---

## BL-CR-01 — Xem Diff của Agent Changes

```
Người dùng (Alex/Maya)
    │
    ▼
[Renderer/Browser] click "Diff" tab trên worktree card
    │ IPC invoke: 'diff.load' { worktreeId }
    ▼
[Main Process — DiffService.load()]
    ├─ Nếu LOCAL worktree:
    │   child_process.execFile('git', ['diff', 'HEAD'])        ← Git CLI local
    │   child_process.execFile('git', ['diff', '--cached'])    ← Git CLI local
    │   child_process.execFile('git', ['status'])              ← Git CLI local
    │
    ├─ Nếu REMOTE worktree (Dev Server):
    │   relay = RelayConnectionPool.getOrConnect(devServerId)
    │   relay.call('git.diff', { repoPath, ref: 'HEAD' })      ← Dev Server
    │   relay.call('git.diff', { repoPath, staged: true })     ← Dev Server
    │   relay.call('git.status', { cwd: repoPath })            ← Dev Server
    │
    └─ Parse unified diff format → structured DiffData
    │
    ▼
[Renderer/Browser] DiffViewer component
    ├─ Render syntax-highlighted diff (Monaco / CodeMirror)
    ├─ Files tree với change counts
    └─ Line-level navigation

Luồng LOCAL:
User → Renderer → IPC → Main → Git CLI local → diff parser → Renderer

Luồng REMOTE:
User → Renderer → IPC → Main → relay → Dev Server → Git CLI → result → Renderer
```

---

## BL-CR-02 — Annotate Dòng Code trong Diff

```
Người dùng (Maya/Alex)
    │
    ▼
[Renderer/Browser] click trên dòng code trong DiffViewer
    │ IPC/WS invoke: 'annotation.create' { worktreeId, file, line, text }
    ▼
[Main Process — AnnotationService.create()]
    └─ INSERT annotations { id, worktreeId, file, line, text, createdAt }  ← SQLite
    │
    ▼
[Renderer/Browser] hiển thị inline annotation marker trên dòng code

SEND TO AGENT (nếu agent đang chạy):
    │ IPC/WS invoke: 'annotation.sendToAgent' { sessionId, annotationIds[] }
    ▼
[Main Process — AgentManager.injectAnnotations()]  ← (chưa implement - BUG-AG-ORCH-005)
    ├─ Load annotations từ SQLite
    ├─ Format context message: "Review feedback:\n  file:line — comment"
    │
    ├─ Nếu LOCAL agent: inject vào PTY stdin trực tiếp
    └─ Nếu REMOTE agent: relay.call('agent.sendInput', { ptyId, data })  ← Dev Server
                         (MISSING — BUG-AG-ORCH-001)

Luồng:
User click line → Renderer → IPC → Main → SQLite (INSERT annotation)
                                        → Renderer (inline marker)
Send to Agent → IPC → Main → (local) PTY stdin | (remote) relay → Dev Server PTY
```

---

## BL-CR-03 — Gửi Feedback về Agent

```
Người dùng (Maya/Alex)
    │
    ▼
[Renderer/Browser] chọn annotations, click "Send to Agent"
    │ IPC/WS invoke: 'review.sendFeedback' { sessionId, feedback }
    ▼
[Main Process — AgentManager.sendFeedback()]
    ├─ Build feedback message (Markdown format)
    │
    ├─ Nếu LOCAL agent: pty.write(feedbackMessage) → PTY stdin
    └─ Nếu REMOTE agent: relay.call('agent.sendInput', { ptyId, data: feedbackMessage })
                         [Dev Server: PTY handle.write(data)]
    │
    ▼
[Renderer/Browser] agent card: status → "running" (processing feedback)

Luồng:
User → Renderer → IPC → Main → (local) PTY stdin | (remote) relay → Dev Server PTY → Agent process
```

---

## BL-CR-04 — Tạo Commit Message bằng AI

```
Người dùng (Alex/Maya)
    │
    ▼
[Renderer/Browser] click "AI: Generate Commit Message" trong Diff panel
    │ IPC/WS invoke: 'git.generateCommitMessage' { worktreeId }
    ▼
[Main Process — GitService.generateCommitMessage()]
    │
    ├─ Nếu LOCAL:
    │   child_process.execFile('git', ['diff', '--staged'])    ← Git CLI local
    │   Parse diff → prompt context
    │   Inject vào local agent PTY stdin
    │   Wait for agent response (parse from PTY output)
    │
    ├─ Nếu REMOTE:
    │   relay.call('git.diff', { cwd: repoPath, staged: true }) ← Dev Server
    │   Parse diff → prompt context
    │   relay.call('agent.sendInput', { ptyId, data: prompt })  ← Dev Server
    │   Wait for agent output events → parse result
    │
    └─ Return generated message
    │
    ▼
[Renderer/Browser] pre-fill commit message input field
    │ Người dùng edit nếu cần → click "Commit"
    │ IPC/WS invoke: 'git.commit' { worktreeId, message }
    ▼
[Main Process]
    ├─ LOCAL: git commit -m "<message>"               ← Git CLI
    └─ REMOTE: relay.call('git.commit', { message })  ← Dev Server

Luồng LOCAL:
User → Renderer → IPC → Main → Git CLI (diff --staged) → Local PTY (AI gen) → Renderer

Luồng REMOTE:
User → Renderer → IPC → Main → relay → Dev Server Git CLI (diff --staged)
                               → relay → Dev Server PTY (agent)
                               → relay → Dev Server Git CLI (commit) → done
```

---

## BL-CR-05 — Tạo Pull Request với AI

```
Người dùng (Alex/Maya)
    │
    ▼
[Renderer/Browser] click "Create PR"
    │ IPC/WS invoke: 'pr.create' { worktreeId, githubToken }
    ▼
[Main Process — GitHubService.createPR()]
    │
    ├─ Push branch:
    │   LOCAL:  child_process.execFile('git', ['push', 'origin', branch])
    │   REMOTE: relay.call('git.push', { branch, repoPath })  ← Dev Server
    │
    ├─ Load diff summary (từ BL-CR-01)
    │
    ├─ AI-generate PR title + description:
    │   LOCAL:  inject to local agent PTY → parse response
    │   REMOTE: relay.call('agent.sendInput', { ptyId, data: prompt }) ← Dev Server
    │           stream output → parse response
    │
    ├─ GitHub REST API: POST /repos/{owner}/{repo}/pulls
    │   Body: { title, body, head: branch, base: main }
    │   Headers: Authorization: Bearer <token>
    │   [Gọi từ Main Process / Orca Server — không qua Dev Server]
    │
    ├─ Nhận PR URL từ response
    └─ emit: pr:created { url, number }
    │
    ▼
[Renderer/Browser] hiển thị PR link + open in browser option

Luồng LOCAL:
User → Renderer → IPC → Main → Git CLI (push) → Local PTY (AI gen)
                            → GitHub REST API → Renderer (PR URL)

Luồng REMOTE (Dev Server):
User → Renderer → IPC → Main → relay → Dev Server: git push
                               → relay → Dev Server: agent sendInput (AI gen)
                            → GitHub REST API (Main process) → Renderer (PR URL)
```

---

## Sơ đồ tổng quan — Code Review

```
┌─────────────┐   IPC/WS   ┌─────────────────────────────────────┐
│  Renderer   │◄──────────►│  Main Process / Orca Server          │
│  DiffViewer │            │  DiffService                         │
│  Annotations│            │  GitService                          │
│  PR Form    │            │  GitHubService                       │
└─────────────┘            └───┬──────────────┬───────────────────┘
                               │              │
                      ┌────────▼──┐  ┌────────▼────────┐
                      │  Git CLI  │  │  GitHub REST API │
                      │  (local)  │  │  POST /pulls     │
                      └─────┬─────┘  │  Bearer token    │
                            │        └─────────────────┘
                     ┌──────▼──────────────────────────────┐
                     │  Dev Server (Remote)                  │
                     │  relay.call('git.diff')               │
                     │  relay.call('git.commit')             │
                     │  relay.call('git.push')               │
                     │  relay.call('agent.sendInput')        │
                     │  → Dev Server Git CLI + Agent PTY     │
                     └─────────────────────────────────────┘

Chiều kết nối:
  Dev Server ──WS connect──► Orca Server  (Dev Server = WS client)
  Orca Server ──JSON-RPC──► Dev Server    (relay.call requests)
  Dev Server ──JSON-RPC──► Orca Server    (results + stream output)
```

---

## Khác biệt HLD vs Implementation

| HLD Mô tả | Thực tế |
|-----------|---------|
| `Daemon Unix Socket` | Electron IPC / HTTP WS |
| `DiffService` chỉ local Git CLI | Cần dual-path: local + relay cho remote |
| `Daemon.writeToPty()` | relay.call('agent.sendInput') cho remote; agent.sendInput chưa implement |
| Code review chỉ local | Phải bổ sung remote path qua Dev Server relay |

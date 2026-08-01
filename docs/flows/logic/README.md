# Data Flows — Business Logic

Thư mục này chứa **luồng dữ liệu chi tiết** cho từng nhóm nghiệp vụ của Orca, dựa trên kiến trúc HLD v1.

**Kiến trúc tham chiếu:** [`/docs/hld/v1/`](../../hld/v1/)  
**Nghiệp vụ nguồn:** [`/docs/logic/`](../../logic/)  
**Cập nhật:** 2026-07-31

---

## Danh sách luồng dữ liệu

| File | Domain | Nghiệp vụ | Thành phần chính |
|------|--------|-----------|-----------------|
| [worktree-management.md](./worktree-management.md) | Worktree Management | BL-WT-01 → 05 | Main, Git CLI, Daemon, SQLite |
| [agent-orchestration.md](./agent-orchestration.md) | Agent Orchestration | BL-AG-01 → 05 | Main, Daemon, PTY, AI Agent |
| [terminal-management.md](./terminal-management.md) | Terminal Management | BL-TM-01 → 04 | Daemon, node-pty, xterm.js, SQLite |
| [remote-development.md](./remote-development.md) | Remote Development | BL-SSH-01 → 04 | SSH (ssh2), Relay Binary, Port Fwd |
| [code-review.md](./code-review.md) | Code Review | BL-CR-01 → 05 | Git CLI, GitHub API, AI Agent |
| [project-integration.md](./project-integration.md) | Project Integration | BL-PI-01 → 04 | GitHub/GitLab/Linear API, Credential |
| [mobile-companion.md](./mobile-companion.md) | Mobile Companion | BL-MB-01 → 04 | TweetNaCl WS, APNs/FCM |
| [automation.md](./automation.md) | Automation | BL-AT-01 → 04 | Scheduler, EventBus, Daemon |
| [design-browser.md](./design-browser.md) | Design & Browser | BL-DB-01 → 03 | CDP, WebContents, Agent PTY |
| [cli-headless.md](./cli-headless.md) | CLI & Headless | BL-CLI-01 → 03 | Unix Socket, Daemon, HTTP |
| [auth.md](./auth.md) | Auth & User Mgmt | BL-AUTH-01 → 05 | Express, bcrypt, Session, Audit |
| [fleet.md](./fleet.md) | Fleet Management | BL-FLEET-01 → 04 | SSH, SFTP, Health Monitor |
| [agent-ws.md](./agent-ws.md) | Agent WebSocket | BL-AWS-01 → 03 | WS binary, JSON-RPC, HMAC |
| [remote-integration.md](./remote-integration.md) | Remote Integration | BL-INT-01 → 03 | AES-256-GCM, gh/glab CLI, Preflight |
| [profile.md](./profile.md) | Profile & Project | BL-PRF-01 → 04 | ProfileResolver, 3-layer merge |
| [ai-providers.md](./ai-providers.md) | AI Provider Mgmt | BL-AIP-01 → 03 | SubtleCrypto, relay encrypt, quota |
| [workflow-orchestration.md](./workflow-orchestration.md) | Workflow Orch. | BL-WF-01 → 03 | DAG, waves, StepExecutors |
| [task-graph.md](./task-graph.md) | Task Graph | BL-TG-01 → 04 | DAG, AI planner, grant 5-level |
| [project-workspace.md](./project-workspace.md) | Project Workspace | BL-PW-01 → 04 | RelayPool, Explorer, Git, Agent |

---

## Ký hiệu trong luồng dữ liệu

```
→   Luồng dữ liệu đi đến (request/call)
←   Luồng phản hồi (response)
│   Nhánh xử lý
├─  Step trong flow
└─  Bước cuối trong nhánh
┌┐  Component box
↔   Bidirectional communication
×N  Lặp N lần (parallel hoặc serial)
```

## Các thành phần kiến trúc hay xuất hiện

| Thành phần | Mô tả | Giao thức |
|------------|-------|-----------|
| Renderer (React) | UI layer — Electron sandboxed Chromium | contextBridge API |
| Main Process | Business logic — Node.js, Electron main | IPC / Unix Socket |
| Daemon Process | Headless server — PTY manager | Unix Socket NDJSON |
| Relay Binary | Node.js on remote host | WebSocket binary |
| Dev Server Agent | node-pty + agent spawn | JSON-RPC 2.0 |
| Server DB | SQLite / MySQL / PostgreSQL | IConnectionPool SQL |
| Git CLI | child_process.execFile | stdin/stdout |
| SSH (ssh2) | Remote connection | SSH protocol |
| WebSocket :6768 | Web server WS endpoint | Binary frames |

---

## Phân loại theo actor

| Actor | Nghiệp vụ chính |
|-------|----------------|
| Alex (Senior Dev) | WT, AG, CR, TM |
| Maya (Tech Lead) | CR, PI, AG, TG |
| Carlos (Remote Dev) | SSH, TM, AG, PW |
| Sam (Mobile) | MB, AG monitor |
| DevOps | AT, CLI, FLEET |
| Admin | AUTH, FLEET, AIP, PRF |
| Lead | PRF, TG, WF, PW |
| Agent Developer | AWS |

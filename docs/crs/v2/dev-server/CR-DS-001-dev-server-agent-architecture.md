# CR-DS-001 — Dev Server Agent Architecture

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-001 |
| **Tên** | Dev Server Agent — Full Backend on Dev Server |
| **Loại** | Architectural Change |
| **Priority** | P0 — Critical |
| **Phiên bản** | v6.0 |
| **Ngày tạo** | 2026-07-30 |
| **Trạng thái** | Proposed |
| **Tác giả** | Architecture Team |
| **Tác động HLD** | C1, C2, C3, C4, deployment.md, security.md |
| **Tác động Features** | F01–F04, F07, F09, F12–F14, F17–F18, F29, F36–F39 |

---

## Bối cảnh & Vấn đề

### Kiến trúc hiện tại (v5.x)

```
Orca Backend Server (Gateway)         Dev Server (Remote Host)
┌─────────────────────────────┐       ┌─────────────────────────┐
│ Business Logic (ALL)         │       │ Orca Relay (thin binary) │
│   Worktree Management        │──────▶│   PTY bridging            │
│   Agent Orchestration        │  SSH  │   FS read/write          │
│   Git Operations             │  WS   │   Git command exec       │
│   Profile Resolution         │       │   Port scanning          │
│   Workflow Dispatch          │       │   Agent hook interception│
│   Task Execution             │       └─────────────────────────┘
└─────────────────────────────┘
```

**Vấn đề với thin relay:**
1. Relay chỉ là "pipe" — không có business logic, không có state
2. Mọi business logic phải đi qua Orca Server → network latency cao
3. Relay không thể tự phục hồi khi mất kết nối về Orca Server
4. Không thể scale: nhiều users → nhiều relay channels → bottleneck tại server
5. Relay không thể thực hiện autonomous operations (cron, health check local)
6. Relay version phải match Orca Server chặt chẽ

---

## Giải pháp: Dev Server Agent Architecture

### Mô hình mới (v6.0)

```
┌──────────────────────────────────────────────────────────────┐
│  ORCA BACKEND SERVER (Control Plane / Gateway)               │
│                                                               │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │ Auth/Session │  │ Tenant/Team  │  │ Fleet Management   │  │
│  │ (F22, F23,  │  │ User/Profile │  │ (F27, F28, F31)    │  │
│  │  F24)       │  │ (F32, F33)   │  │                    │  │
│  └─────────────┘  └──────────────┘  └────────────────────┘  │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │ Project Reg │  │ AI Provider  │  │ Workflow/Task       │  │
│  │ (F34, F38)  │  │ Registry     │  │ Dispatcher         │  │
│  │             │  │ (F35 meta)   │  │ (F36, F37)         │  │
│  └─────────────┘  └──────────────┘  └────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ Agent Connection Pool (AgentConnectionManager)       │    │
│  │ WS multiplexer | Session context injection           │    │
│  └──────────────────────────────────────────────────────┘    │
└────────────────────────────┬─────────────────────────────────┘
                             │ Persistent WS / SSH tunnel
                    ┌────────┴────────────────────────┐
                    │                                  │
         ┌──────────▼──────────┐        ┌─────────────▼──────────┐
         │ DEV SERVER AGENT A  │        │ DEV SERVER AGENT B     │
         │ (headless, no UI)   │        │ (headless, no UI)      │
         │                     │        │                        │
         │ ┌─────────────────┐ │        │ ┌─────────────────┐   │
         │ │ PTY Manager     │ │        │ │ PTY Manager     │   │
         │ │ Worktree Mgr    │ │        │ │ Worktree Mgr    │   │
         │ │ Git Engine      │ │        │ │ Git Engine      │   │
         │ │ Agent Spawner   │ │        │ │ Agent Spawner   │   │
         │ │ FS Engine       │ │        │ │ FS Engine       │   │
         │ │ AI Cred Store   │ │        │ │ AI Cred Store   │   │
         │ │ Step Executor   │ │        │ │ Step Executor   │   │
         │ │ Health Reporter │ │        │ │ Health Reporter │   │
         │ └─────────────────┘ │        │ └─────────────────┘   │
         └─────────────────────┘        └────────────────────────┘
```

---

## Định nghĩa Dev Server Agent

### Vai trò

Dev Server Agent là một **full Orca backend** chạy headless (không có UI, không có Auth layer) trên mỗi dev server. Nó:

1. **Implements toàn bộ data-plane operations** — mọi thao tác thực sự xảy ra trên dev server
2. **Không cần kết nối về Orca Backend Server** để thực thi local tasks (autonomous)
3. **Báo cáo trạng thái** về Orca Backend Server qua event stream
4. **Nhận lệnh** từ Orca Backend Server qua RPC channel (khi có kết nối)
5. **Tự duy trì state** — local SQLite cho worktrees, sessions, task executions

### Khác biệt với Orca Relay (thin binary)

| Khía cạnh | Orca Relay (cũ) | Dev Server Agent (mới) |
|-----------|-----------------|------------------------|
| Business logic | Không có | Đầy đủ (worktree, git, agent) |
| State | Stateless | Stateful (local SQLite) |
| Autonomous ops | Không | Có (cron, health check, auto-cleanup) |
| Connection dependency | Luôn cần kết nối | Hoạt động offline |
| AI Provider | Không quản lý | Credential store, health check |
| Step execution | Không | Workflow step, task agent executor |
| Protocol | Binary frame RPC | JSON-RPC + Event Stream |
| Version coupling | Chặt (must match server) | Loose (semantic versioning) |
| Deploy method | SFTP upload binary | systemd service / Docker |

---

## Phân chia trách nhiệm (Responsibility Boundary)

### Orca Backend Server — Control Plane

| Responsibility | Module | Ghi chú |
|---------------|--------|---------|
| Authentication & Sessions | F22, F23, F24 | Chỉ server có auth context |
| Tenant / Company profile | F33 | Policy authority |
| Team & RBAC | F32 | Permission enforcement |
| User management | F23, F25 | CRUD, invite |
| Project registry | F34 | devServerId binding |
| AI Provider metadata | F35 | Không lưu credentials |
| Workflow dispatching | F36 | DAG build + dispatch steps |
| Task graph management | F37 | DAG management + grant |
| Fleet monitoring | F27, F31 | Aggregate agent health |
| WebSocket multiplexer | F22 | Route UI ↔ Agent |
| Admin panel | F25 | Company/team admin |
| Mobile companion bridge | F03 | Push notification routing |

### Dev Server Agent — Data Plane

| Responsibility | Feature tương ứng | Ghi chú |
|---------------|------------------|---------|
| PTY / Terminal sessions | F02 | node-pty local |
| Worktree CRUD | F01 | `git worktree` local |
| Git operations | F39 | status, diff, commit, push |
| AI agent spawn | F04 | Claude, Codex, Gemini on PTY |
| AI Provider credential store | F35 | AES-256-GCM local files |
| File system ops | F12 | read, write, watch |
| Text search | F13 | ripgrep local |
| Workflow step execution | F36 | agent-step, shell-step executor |
| Task agent execution | F37 | TaskAgentExecutor |
| Port scan & forwarding | F07 | Local port ops |
| SSH tunneling | F07 | outbound SSH to other hosts |
| Ephemeral VM | F18 | Local VM lifecycle |
| Automation execution | F14 | Local cron + event |
| AI Vault storage | F17 | Session storage local |
| Health reporting | F27 | CPU, RAM, disk, latency |

---

## Agent Capability Advertisement

Khi kết nối về Backend, Agent quảng bá capabilities:

```typescript
interface AgentCapabilities {
  agentVersion: string             // '6.0.0'
  protocolVersion: number          // 3
  os: 'linux' | 'darwin' | 'windows'
  arch: 'x64' | 'arm64'
  features: {
    pty: boolean                   // node-pty available
    git: boolean                   // git binary present
    docker: boolean                // docker available (for F18)
    aiAgents: {                    // detected AI agents on PATH
      [agentName: string]: {
        version: string
        path: string
      }
    }
    aiProviders: string[]          // configured provider accounts
    workspaceRoots: string[]       // accessible repo paths
  }
  resources: {
    cpuCores: number
    memoryGb: number
    diskGb: number
    gpuAvailable: boolean
  }
}
```

---

## Acceptance Criteria

- [ ] Dev Server Agent binary có thể cài đặt như systemd service / launchd / Docker container
- [ ] Agent tự khởi động khi server boot, không cần Orca Backend
- [ ] Agent hoạt động đầy đủ (PTY, git, worktree) ngay cả khi mất kết nối về Backend
- [ ] Agent reconnect tự động về Backend khi kết nối khôi phục (exponential backoff)
- [ ] Agent quảng bá capabilities khi kết nối (handshake)
- [ ] Backend có thể dispatch operations xuống Agent qua RPC
- [ ] Agent stream events lên Backend (PTY output, agent status, health metrics)
- [ ] Agent local SQLite persist state qua restart
- [ ] Version negotiation: backward-compat trong minor versions
- [ ] Security: Agent chỉ accept RPC từ authenticated Backend (shared secret / mTLS)

---

## Tham chiếu

- [CR-DS-002](./CR-DS-002-gateway-agent-rpc-protocol.md) — Protocol chi tiết
- [CR-DS-003](./CR-DS-003-feature-delegation-matrix.md) — Feature delegation
- [CR-DS-004](./CR-DS-004-agent-lifecycle-management.md) — Lifecycle & deployment
- [CR-DS-005](./CR-DS-005-agent-session-context-propagation.md) — Session context
- HLD: [C2-containers.md](../../hld/C2-containers.md) — Container thay đổi
- HLD: [C3-components.md](../../hld/C3-components.md) — C3.13 Dev Server Agent

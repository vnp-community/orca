# ADR-013 — Dev Server Agent Replaces Thin Relay Binary

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-013 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | C1, C2, C3.13, C4.11, README (v6.0) |
| **CR Ref** | CR-DS-001, CR-DS-003 |
| **Code Ref** | `src/agent/` (NEW), `src/main/dev-server/agent-connection-manager.ts` (NEW) |
| **Feature Ref** | F01–F04, F07, F09, F12–F14, F17–F18, F29, F36–F39 |
| **Supersedes** | [ADR-004](./ADR-004-relay-binary-ssh-remote-execution.md) |

---

## Bối cảnh

### Vấn đề với Thin Relay (ADR-004)

Kiến trúc v5.x sử dụng **Thin Relay binary** — một Node.js binary được auto-deploy lên dev server via SFTP. Relay hoạt động như "pipe": nhận JSON-RPC calls từ Orca Server, thực thi operations (PTY, file, git), trả kết quả.

**Giới hạn khi scale lên enterprise:**

| Vấn đề | Mô tả |
|--------|-------|
| **Stateless** | Relay không giữ state → mọi operations phải carry full context mỗi lần |
| **Business logic tập trung** | Tất cả business logic nằm ở Orca Server → mọi remote op đều có 1 network round-trip |
| **Không autonomous** | Nếu Orca Server down, relay không thể tự thực hiện bất kỳ operation nào |
| **Version coupling chặt** | Relay version phải match Orca Server version → không thể deploy riêng biệt |
| **Không scale multi-user** | Relay không có user context → mọi requests đi vào cùng process space |
| **Không quản lý credentials** | AI Provider credentials không thể lưu trên relay |
| **Deployment hạn chế** | SFTP auto-deploy không phù hợp với container/Kubernetes environments |

---

## Quyết định

### Dev Server Agent = Full Orca Backend (headless)

Thay thế Thin Relay bằng **Dev Server Agent** — một Orca backend đầy đủ chạy headless (không UI) trên mỗi dev server.

### Phân chia Control Plane / Data Plane

```
┌─────────────────────────────────────────────────────────┐
│  Orca Backend Server — CONTROL PLANE                     │
│  • Auth, Session, Tenant, Team, User                     │
│  • Profile (F33), Project (F34)                          │
│  • AI Provider Registry — metadata only (F35)            │
│  • Workflow Dispatch (F36), Task Graph (F37)             │
│  • Fleet Monitoring (F27, F28, F31)                      │
│  • AgentConnectionManager + SignedContextIssuer          │
└──────────────────────────┬──────────────────────────────┘
                           │ Persistent WebSocket (wss://)
                           │ JSON-RPC + Signed Context
                           ↓
┌─────────────────────────────────────────────────────────┐
│  Dev Server Agent — DATA PLANE (headless)                │
│  • PTY Manager (F02)                                     │
│  • Worktree Engine (F01)                                 │
│  • AI Agent Spawner — profile-aware (F04)                │
│  • Git Engine (F39)                                      │
│  • File System Engine (F12, F13)                         │
│  • AI Provider Credential Store (F35)                    │
│  • Workflow Step Executor (F36)                          │
│  • Health Reporter (F27)                                 │
│  • Local SQLite                                          │
└─────────────────────────────────────────────────────────┘
```

### Agent Characteristics

| Characteristic | Thin Relay (v5.x) | Dev Server Agent (v6.0) |
|---------------|-------------------|------------------------|
| Business logic | None | Full (worktree, git, agent, FS) |
| State | Stateless | Stateful (local SQLite) |
| Autonomous operations | None | Health check, cleanup, credential management |
| Connection direction | Gateway initiates (SFTP + SSH exec) | Agent initiates (outbound WS) |
| Offline capability | None | Full operation locally |
| AI Provider credentials | Not supported | AES-256-GCM local store |
| Step execution | Not supported | Workflow step, task agent |
| Protocol | 13-byte binary header (ADR-005) | JSON-RPC 2.0 + Signed Context (ADR-014, ADR-015) |
| Deployment | SFTP auto-deploy | systemd / Docker / launchd |
| Version coupling | Strict | Loose (semantic versioning) |
| Build output | Single Node.js script | Single binary (Node.js + ncc) |

### Agent Connection Model (Agent-initiated)

```
Thay vì Gateway SSH vào server để deploy + start relay:

Agent → wss://orca-backend.company.com/agent/connect
  Header: Authorization: Bearer <agentToken>
  
Lợi ích:
  - Gateway không cần SSH access vào dev server
  - Agent hoạt động behind NAT/firewall
  - Agent tự restart (systemd KeepAlive)
  - Agent tự reconnect (exponential backoff)
```

### Backward Compatibility

```
Thin Relay (v4/v5) vẫn được support trong v6.0 cho:
  - Existing relay-ssh connections (legacy)
  - Servers chưa được upgrade lên Agent

Khi cả relay và agent available:
  - Gateway ưu tiên Agent connection
  - Fallback về relay nếu agent không available
  - Warning trong Admin Panel: "Server X đang chạy legacy relay"
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------| 
| **Dev Server Agent (full headless backend)** ✅ | Autonomous, stateful, proper multi-user isolation, enterprise-grade deployment |
| Thin Relay (giữ nguyên ADR-004) | Không scale: stateless, gateway-dependent, no credential store |
| Thicker Relay (thêm logic vào relay) | Half-solution: vẫn có version coupling, deployment phức tạp |
| Pure serverless (Lambda per operation) | Cold start latency, không hỗ trợ PTY streaming |
| SSH Agent Forwarding | Security concern, không hỗ trợ web mode |

---

## Hậu quả

**Tích cực:**
- Agent autonomous → hoạt động khi mất kết nối với Gateway
- Proper multi-user isolation trên agent (PTY ownership, path enforcement, git author)
- AI Provider credentials có thể lưu trên agent (AES-256-GCM)
- Workflow step execution trực tiếp trên agent → giảm latency
- Enterprise deployment: systemd / Docker / Kubernetes
- Loose version coupling → agent và gateway có thể deploy riêng biệt

**Tiêu cực:**
- Cần build `src/agent/` package (new codebase)
- Agent binary lớn hơn (~80MB vs ~5MB relay)
- Agent cần ORCA_AI_CREDENTIAL_KEY env var (per-server setup)
- Backward compat: phải maintain relay protocol trong thời gian migration
- Admin phải chạy install script trên mỗi dev server

---

## Migration Path

```
Phase 1: Build Agent core (src/agent/)
  - AgentRpcServer, ContextVerifier, PtyManager, EventBus
  - Parallel với relay (relay vẫn chạy)

Phase 2: Gateway changes
  - AgentConnectionManager (accept agent connections)
  - SignedContextIssuer
  - AgentDispatcher (route ops to agent)

Phase 3: Feature migration (per dev server)
  - Admin installs agent trên server
  - Gateway detects agent → switches to Agent mode
  - Relay deactivated (but not removed yet)

Phase 4: Relay deprecation
  - Announce: relay support removed in v7.0
  - Migration timeline: 6 months
```

---

## Trạng thái Implementation

❌ Chưa implement (v6.0 proposed)  
🎯 `src/agent/` — new package  
🎯 `src/main/dev-server/agent-connection-manager.ts`  
🎯 `src/main/dev-server/signed-context-issuer.ts`  
🎯 `src/main/dev-server/agent-dispatcher.ts`  
🎯 `deploy/agent/install.sh` — install script  
🎯 `deploy/agent/Dockerfile` — Docker image

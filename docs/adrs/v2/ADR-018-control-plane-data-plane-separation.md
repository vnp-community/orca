# ADR-018 — Control Plane / Data Plane Separation

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-018 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | README (Architecture Layers L0–L5 vs A0–A4), C1, C2 |
| **CR Ref** | CR-DS-001 |
| **Code Ref** | `src/main/` (Control Plane), `src/agent/` (Data Plane) |
| **Feature Ref** | F22–F25 (auth/admin), F33–F37 (enterprise features) |
| **Amends** | [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md) |

---

## Bối cảnh

HLD v6.0 phân chia rõ ràng **Control Plane** (Orca Backend Server) và **Data Plane** (Dev Server Agent). ADR-013 quyết định thay relay bằng Agent nhưng chưa chỉ rõ ranh giới trách nhiệm giữa hai planes. Cần ADR này để:

1. Định nghĩa rõ những gì thuộc Control Plane vs Data Plane
2. Xác định communication contract giữa 2 planes
3. Đảm bảo không bị "plane leakage" (business logic ở sai nơi)

---

## Quyết định

### Control Plane (Orca Backend Server — L0–L5)

> **"Gateway quyết định ai được làm gì, khi nào, và ở đâu"**

| Layer | Trách nhiệm |
|-------|-------------|
| **L0 UI** | React SPA, Admin Panel, Task Board, Workflow UI |
| **L1 Platform** | IPlatformServices: NodeAdapter (server) / ElectronAdapter (desktop) |
| **L2 Auth** | AuthManager, SessionManager, bcrypt 12r, HTTP-only cookie, SSO |
| **L3 Control** | **Tenant/Team/Profile** (F32, F33), **Project Registry** (F34), **AI Provider Registry** — metadata only (F35), **Workflow DAG Builder + Dispatcher** (F36), **Task Graph + Grant System** (F37), **Fleet Monitor** (F27, F31), **Agent Connection Manager**, **Signed Context Issuer** |
| **L4 Repository** | IStateRepository: SqlRepository |
| **L5 Database** | IConnectionPool, Migrations 0001–0010 |

**Control Plane responsibilities:**
- ✅ Authentication & Authorization
- ✅ Tenant/Team/User hierarchy
- ✅ Policy decisions (who can do what)
- ✅ Workflow orchestration decisions (which step runs where)
- ✅ Task planning and assignment
- ✅ AI Provider **metadata** (which accounts exist, which to use)
- ✅ Fleet health monitoring
- ✅ Signed context issuance (30s TTL)
- ❌ **NOT**: Execute PTY, run git, spawn agents, read files, store credentials

### Data Plane (Dev Server Agent — A0–A4)

> **"Agent thực thi những gì được ủy quyền, không hơn"**

| Layer | Trách nhiệm |
|-------|-------------|
| **A0 RPC** | JSON-RPC 2.0 server, context verification, method routing |
| **A1 Operations** | PTY, worktree, git, file system, SSH tunnel |
| **A2 Execution** | Workflow step execution, task agent execution |
| **A3 Storage** | AI credentials (AES-256-GCM), AI vault, local SQLite |
| **A4 Reporting** | Event streaming, health metrics, local audit |

**Data Plane responsibilities:**
- ✅ Execute PTY sessions (per-userId isolated)
- ✅ Run git commands (with author injection from signed ctx)
- ✅ Spawn AI agents (with profile injection)
- ✅ Read/write files
- ✅ Store AI API keys (AES-256-GCM, ORCA_AI_CREDENTIAL_KEY)
- ✅ Execute workflow steps (agent/shell/action)
- ✅ Report health metrics
- ❌ **NOT**: Make auth decisions, manage users, orchestrate workflows (only execute steps)

---

## Communication Contract

```
Control Plane → Data Plane:

  1. DISPATCH: "Execute this step on server X"
     Gateway → Agent.call('workflow.step.execute', {
       stepDef: { type: 'agent', prompt: '...' },
       _ctx: SignedExecutionContext  // HMAC-SHA256, 30s TTL
     })

  2. QUERY: "What is the health of server X?"
     Gateway → Agent.call('health.metrics')
     → { cpu: 45, ram: 62, disk: 78, latency: 12 }

  3. SUBSCRIPTION: "Stream events from server X"
     Gateway → Agent.notify('event.subscribe', { types: ['pty.output', 'agent.status'] })
     → Agent → Gateway stream: { type: 'pty.output', ptyId, chunk }

Data Plane → Control Plane:
  ONLY via events (reactive, not proactive policy decisions):
  - event.ptyOutput: PTY output chunks
  - event.agentStatus: idle/running/waiting/complete
  - event.gitPushDone: push result
  - event.stepDone: workflow step completion
  - event.health: periodic health report
```

---

## Plane Boundary Enforcement (Anti-patterns)

### ❌ WRONG: Control Plane logic in Agent

```typescript
// BAD: Agent making authorization decision
async function spawnAgent(ctx: RpcExecutionContext, params: SpawnParams) {
  // WRONG: Agent should NOT check permissions
  const user = await db.query('SELECT * FROM orca_users WHERE id = ?', [ctx.userId])
  if (user.role !== 'developer') throw new ForbiddenError()
  // ^^^^ This is Control Plane responsibility
}
```

### ✅ RIGHT: Authorization in Control Plane, execution in Data Plane

```typescript
// CORRECT: Gateway validates, then issues signed context
// Gateway (Control Plane):
async function routeAgentSpawn(userId: string, projectId: string) {
  const member = await ProjectService.getMember(projectId, userId)  // RBAC check
  if (!member) throw new ForbiddenError()

  const ctx = SignedContextIssuer.issue({ userId, projectId, role: member.role })
  await agentConnectionManager.call(devServerId, 'pty.spawn', { ...params, _ctx: ctx })
}

// Agent (Data Plane):
async function spawnAgent(params: SpawnParams) {
  // Verify ctx signature, expiry — that's all agent does for auth
  const ctx = ContextVerifier.verify(params._ctx)
  // Then execute with trusted userId from ctx
  await PtyManager.create(ctx.userId, binary, args, env)
}
```

---

## Rationale

| Lựa chọn | Đánh giá |
|---|---|
| **Control Plane / Data Plane separation (strict)** ✅ | Rõ ràng; Security: agent compromise không leak auth logic; Scalable: multiple Gateways → multiple Agents |
| Monolithic (all in Gateway, relay for exec) | ❌ Scalability issue; relay stateless; no credential isolation |
| Distributed monolith (Agent has auth logic) | ❌ Dual auth paths; inconsistent RBAC; hard to audit |
| Microservices per feature | ❌ Overkill; operational complexity; latency |

---

## Security Implication

```
Threat model: Attacker compromises Dev Server Agent

v5.x (thick relay):
  → Can impersonate any user (no context verification)
  → Can access all worktrees

v6.0 (Agent with signed ctx):
  → ctx.userId verified via HMAC → can only act as context user
  → ctx.expiresAt = +30s → replay window tiny
  → Agent has no DB access → cannot escalate to admin
  → AI credentials scoped by accountId → isolated
```

---

## Trạng thái Implementation

🚧 Pattern định nghĩa; cần enforce qua code review  
🎯 ESLint rule: `src/agent/` không được import từ `src/main/session/`, `src/main/auth/`  
🎯 `src/main/dev-server/signed-context-issuer.ts` — phát context  
🎯 `src/agent/rpc/context-verifier.ts` — xác minh context

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md) | Agent architecture overview |
| [ADR-015](../v1/ADR-015-signed-execution-context-gateway-agent.md) | HMAC signed context |
| [ADR-017](./ADR-017-dev-server-agent-layer-model.md) | Agent internal layer model |
| [flows/authentication.md](../../flows/authentication.md#94-hmac-sha256-signed-context-v60) | HMAC context trong auth |
| **HLD README v6.0** | Architecture Layers L0–L5 + A0–A4 |

# ADR-019 — Agent Autonomous Operation & Reconnect Strategy

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-019 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | README (Architectural Principle 8: Agent Autonomous Operation), C4.11 |
| **CR Ref** | CR-DS-001, CR-DS-003 |
| **Code Ref** | `src/agent/rpc/reconnect-manager.ts`, `src/agent/reporting/event-emitter.ts`, `src/agent/storage/local-db.ts` |
| **Feature Ref** | F01, F04, F27, F36, F37 |
| **Amends** | [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md), [ADR-017](./ADR-017-dev-server-agent-layer-model.md) |

---

## Bối cảnh

HLD v6.0 Architectural Principle 8 phát biểu:
> *"Agent phải hoạt động đầy đủ ngay cả khi Gateway offline — stateful, self-sufficient."*

Thin Relay (v5.x) hoàn toàn phụ thuộc vào Gateway. Nếu Gateway down, relay không thể thực hiện bất kỳ thao tác nào. Trong môi trường enterprise (data center deployments), Gateway có thể restart trong khi các Dev Server vẫn cần phục vụ ongoing work (agents đang chạy, PTY sessions active, workflow steps executing).

**Vấn đề cần giải quyết:**
1. Agent đang chạy workflow step → Gateway restart → step phải tiếp tục
2. PTY session đang active → Gateway disconnect → user không mất terminal
3. Events bị miss khi Gateway offline → replay after reconnect

---

## Quyết định

### 1. Agent-Initiated Connection (Outbound)

```typescript
// Agent connects TO Gateway — không phải ngược lại
// Lợi ích:
// - Agent behind NAT/firewall → không cần inbound port
// - Agent self-restart via systemd KeepAlive
// - Tự nhiên hơn cho reconnect (agent là initiator)

// src/agent/rpc/reconnect-manager.ts
class ReconnectManager {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private readonly MAX_DELAY_MS = 30_000  // 30s cap

  async connect(gatewayUrl: string, token: string): Promise<void> {
    while (true) {
      try {
        this.ws = new WebSocket(`${gatewayUrl}/agent/connect`, {
          headers: { Authorization: `Bearer ${token}` }
        })
        await this.waitForOpen()
        this.reconnectAttempts = 0  // reset on success
        this.onConnected()
        await this.waitForClose()
      } catch (err) {
        const delay = this.calculateDelay()
        log.warn(`Reconnect in ${delay}ms (attempt ${++this.reconnectAttempts})`)
        await sleep(delay)
      }
    }
  }

  private calculateDelay(): number {
    // Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s (cap)
    return Math.min(1000 * Math.pow(2, this.reconnectAttempts), this.MAX_DELAY_MS)
  }
}
```

### 2. Local State Persistence (Agent Survives Restart)

```typescript
// src/agent/storage/local-db.ts
// SQLite tables on Dev Server:

CREATE TABLE agent_pty_sessions (
  id          TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL,
  binary      TEXT NOT NULL,
  args        TEXT NOT NULL DEFAULT '[]',
  cwd         TEXT NOT NULL,
  env_json    TEXT NOT NULL DEFAULT '{}',
  cols        INTEGER NOT NULL DEFAULT 220,
  rows        INTEGER NOT NULL DEFAULT 50,
  created_at  INTEGER NOT NULL,
  last_seen   INTEGER NOT NULL
);

CREATE TABLE agent_worktrees (
  id          TEXT PRIMARY KEY,
  repo_path   TEXT NOT NULL,
  branch      TEXT NOT NULL,
  path        TEXT NOT NULL,
  user_id     TEXT NOT NULL,
  project_id  TEXT,
  created_at  INTEGER NOT NULL
);

CREATE TABLE agent_task_runs (
  id            TEXT PRIMARY KEY,
  task_id       TEXT NOT NULL,
  user_id       TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'running',
  pty_session_id TEXT,
  started_at    INTEGER NOT NULL,
  completed_at  INTEGER
);

// Khi Agent restart:
// 1. Reload active PTY sessions → resume PTY (nếu process còn sống)
// 2. Reload in-progress task runs → resume hoặc mark failed
// 3. Reload worktrees → không cần action (git worktrees persist on disk)
```

### 3. Event Buffer & Replay

```typescript
// src/agent/reporting/event-emitter.ts
class EventEmitter {
  private buffer: AgentEvent[] = []
  private readonly MAX_BUFFER = 1000  // G11: overflow strategy

  emit(event: AgentEvent): void {
    if (this.isGatewayConnected()) {
      this.sendToGateway(event)
    } else {
      // Buffer events while Gateway offline
      if (this.buffer.length >= this.MAX_BUFFER) {
        // OVERFLOW STRATEGY:
        // Drop oldest events (ring buffer behavior)
        // But NEVER drop: agent.complete, task.complete, git.push.done
        const priority = this.getPriority(event)
        if (priority === 'critical') {
          this.buffer.shift()  // remove oldest non-critical
          this.buffer.push(event)
        } else {
          log.warn('Event buffer full, dropping:', event.type)
        }
      } else {
        this.buffer.push(event)
      }
    }
  }

  onReconnect(): void {
    // Replay buffered events to Gateway
    const toReplay = [...this.buffer]
    this.buffer = []

    for (const event of toReplay) {
      this.sendToGateway({
        ...event,
        _replayed: true,           // signal to Gateway
        _originalTimestamp: event.timestamp,
        timestamp: Date.now()
      })
    }
  }

  private getPriority(event: AgentEvent): 'critical' | 'normal' {
    const CRITICAL_EVENTS = ['agent.complete', 'task.done', 'git.push.done', 'workflow.step.done']
    return CRITICAL_EVENTS.includes(event.type) ? 'critical' : 'normal'
  }
}
```

### 4. Workflow Step Continuity

```typescript
// Khi Gateway disconnects TRONG KHI agent step đang execute:

// Agent (Data Plane):
// - Step tiếp tục chạy (no interruption)
// - Output buffer trong EventEmitter
// - Khi Gateway reconnect → replay events
// - Gateway queries: workflow.getExecution(executionId) → thấy step vẫn running

// Gateway (Control Plane):
// Sau reconnect, AgentConnectionManager gọi:
await agent.call('workflow.getActiveSteps', { executionId })
// → [{ stepId, status: 'running', startedAt }]
// → Gateway cập nhật internal state
// → Resume watching events từ agent
```

---

## Open Questions / Known Gaps (từ ADR v1 Gaps)

| Gap | Giải pháp |
|-----|-----------|
| **G8**: Clock skew Gateway↔Agent > 5s → context expire sớm | Allow ±5s drift in ContextVerifier; NTP required on Dev Server |
| **G11**: Buffer 1000 events overflow | Ring buffer + critical event preservation (trên) |
| **G12**: Backward compat: relay (v5) và agent (v6) cùng tồn tại | AgentDispatcher có fallback: `agent → relay (nếu agent unavailable)` |

---

## Rationale

| Lựa chọn | Đánh giá |
|---|---|
| **Agent-initiated + local SQLite + event buffer** ✅ | Autonomous; resilient; no single point of failure |
| Gateway-initiated connection (old relay model) | ❌ Gateway biết IP agent; agent behind NAT = not reachable |
| Pure stateless agent | ❌ PTY sessions lost on Gateway restart; workflow steps interrupted |
| External Redis for event buffer | ❌ Thêm dependency; latency; Redis failure = buffer loss |

---

## Trạng thái Implementation

❌ Chưa implement  
🎯 `src/agent/rpc/reconnect-manager.ts` — exponential backoff  
🎯 `src/agent/storage/local-db.ts` — SQLite persistence  
🎯 `src/agent/reporting/event-emitter.ts` — buffer + replay  
🎯 Systemd unit file: `KeepAlive=true, Restart=on-failure`

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md) | Agent replaces relay (motivates autonomous op) |
| [ADR-014](../v1/ADR-014-gateway-agent-json-rpc-protocol.md) | JSON-RPC protocol for reconnect |
| [ADR-017](./ADR-017-dev-server-agent-layer-model.md) | Layer A4 (Reporting = EventEmitter) |
| [ADR-018](./ADR-018-control-plane-data-plane-separation.md) | Data plane independence |
| **HLD Gaps G8, G11, G12** | Open questions addressed here |
| **HLD Principle 8** | Agent Autonomous Operation |

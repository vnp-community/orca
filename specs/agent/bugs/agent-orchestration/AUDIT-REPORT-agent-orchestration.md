# Audit Chi tiết: Agent Orchestration Domain

**Tài liệu tham chiếu:** [`agent-orchestration.md`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/docs/flows/logic/agent-orchestration.md)  
**Ngày audit:** 2026-08-01  
**Phương pháp:** So sánh từng bước trong HLD với implementation thực tế

---

## Kiến trúc thực tế vs HLD

### 1. Hướng kết nối WebSocket — ĐÃ ĐÚNG ✅

HLD mô tả:
> Dev Server **chủ động mở WebSocket kết nối đến Orca Server** (`ws://orca:6768/agent`).

Implementation thực tế:
- **direct-websocket mode**: `AgentWebSocketServer` (port `/agent`) nhận inbound → Dev Server connect đến Orca ✅
- **relay-websocket mode**: Orca là WS **client** kết nối đến agent WS server ✅
- **relay-ssh mode**: Orca deploy relay qua SSH ✅

→ **Kiến trúc WS kết nối đã implement đúng hướng.**

---

### 2. Handshake Protocol — CÓ SAI KHÁC ⚠️

HLD mô tả:
```
agent.handshake { agentToken }
← handshake-ok { sessionId }
```

Implementation thực tế (`ws-handshake.ts`):
- **receiver mode** (direct-ws): Agent gửi `agent.handshake { agentToken, platform, arch, nodeVersion, agentVersion }` → Orca nhận, verify token, trả `{ ok: true, orcaVersion, sessionId }` ✅
- **initiator mode** (relay-ws): Orca gửi `agent.handshake { orcaVersion }` → Agent trả `{ ok: true, platform, arch, ... }` ✅

→ **Handshake đã implement, nhưng hai mode khác protocol flow.** Không phải bug, là design decision cho 2 modes. Nhưng HLD không phân biệt — **cần cập nhật HLD**.

---

### 3. Agent authentication — SAI KHÁC (relay-ws mode) ⚠️

**relay-ws mode** (`agent-connection-relay.ts:26`):
```typescript
const token = config.agentToken || 'relay-secret'  // ← HARDCODE FALLBACK!
```

Nếu `config.agentToken` không được set → dùng `'relay-secret'` làm token authenticate. Đây là security fallback không an toàn.

→ **Đã ghi nhận là BUG-AG-AIP-002 (hardcode key).** Bug 003 cũ đã ghi nhận vấn đề này.

---

### 4. BL-AG-01: Agent Spawn — KHÔNG KHỚP ĐỂ INVOKE TỪNG PHẦN ❌

HLD mô tả:
```
[Main Process — AgentManager.start()]
    → conn.call('agent.spawn', { agentBinary, args, cwd, env, userId, cols, rows })
    → Dev Server: node-pty.spawn(agentBinary, args, ...)
    → JSON-RPC event: agent.output { ptyId, data }
```

Implementation thực tế:
- **Orca server side**: `ProfileAwareAgentSpawner.spawn()` gọi `relay.call('agent.exec', { command, workdir, env })` — **không phải** `agent.spawn`!
- **Dev server (relay) side**: `agent-rpc-dispatch.ts` có `case 'agent.spawn'` nhưng nhận params `{ taskId, userId, modelId, accountId, cwd }` — **khác hoàn toàn** với `{ agentBinary, args, env, cols, rows }`!

Có **hai implementation song song không liên kết**:
1. `ProfileAwareAgentSpawner` → `agent.exec` (relay không có handler này)
2. `agent-rpc-dispatch.ts` → `agent.spawn` (không có caller từ Orca server)

→ **Critical mismatch:** Cả hai luồng BL-AG-01 đều bị broken!

---

### 5. BL-AG-01: AgentManager.start() — KHÔNG TỒN TẠI ❌

HLD mô tả: `Main Process — AgentManager.start()`, `AgentConnectionManager.getConnection(devServerId)`.

Thực tế grep không tìm thấy:
- `AgentManager` class hoặc `agentManager` instance
- `AgentConnectionManager` class
- IPC handler `agent.start`

**AgentManager và AgentConnectionManager là hai class trong HLD chưa được implement.**  
Thay vào đó, code dùng:
- `ProfileAwareAgentSpawner` (Orca server → task execution)
- `RelayConnectionPool` + `DevServerRelayBridge` (connection management)

→ Kiến trúc thực tế dùng `RelayConnectionPool` thay `AgentConnectionManager`, nhưng không có flow `agent.start` từ UI.

---

### 6. BL-AG-01: Stream `agent.output` từ Dev Server về Orca — KHÔNG CÓ HANDLER ❌

HLD mô tả:
```
Dev Server → stream PTY output về Orca qua WS đang mở
JSON-RPC event: agent.output { ptyId, data: "<OSC output>" }
→ [Main Process — AgentHookParser]
    INSERT orca_sessions { id, worktreeId, agentType, devServerId, startedAt }
```

Thực tế:
- `agent-spawner.ts:163-165`: Output được gửi qua `ws.send(encodeDataFrame(..., JSON.stringify({result:{type:'spawn.output',ptyId,data}})))` — đây là **response** cho id của request, không phải notification/event!
- Orca server side: Grep `spawn.output` → **không có handler** nào trong `src/main` xử lý `spawn.output` events từ relay.
- `AgentHookParser` → **không tồn tại** trong codebase.
- `orca_sessions` table chỉ dùng cho **HTTP auth sessions**, không phải agent sessions.

→ **BL-AG-01 stream pipeline hoàn toàn không có receiver trên Orca server.**

---

### 7. BL-AG-02: Agent Stop — THIẾU agent.sendInput + SIGTERM KHÔNG PHẢI SIGKILL ❌

Đã ghi nhận chi tiết trong BUG-AG-ORCH-001 và BUG-AG-ORCH-002.

Bổ sung từ audit này:
- `agent.kill` trong relay dùng `SIGTERM` cho cả normal kill (line 215)
- HLD: normal stop = `agent.sendInput(Ctrl+C)`, force stop = `agent.kill(SIGKILL)`
- Implementation: không có `sendInput`, `agent.kill` luôn dùng `SIGTERM`

---

### 8. BL-AG-03: Resume Session — KHÔNG CÓ RELAY-SIDE HANDLER ❌

HLD mô tả:
```
[Main Process — AgentManager.resume()]
    SELECT sessionId, devServerId FROM orca_sessions WHERE worktreeId=?
    → conn.call('agent.spawn', { agentBinary, args: [...resumeArgs], cwd, env, userId })
    → Dev Server: node-pty.spawn(agent --resume <id>)
```

Thực tế:
- Không có `AgentManager.resume()` function
- Không có `SELECT FROM orca_sessions WHERE worktreeId=?` cho agent sessions (chỉ có HTTP auth sessions)
- `agent-rpc-dispatch.ts`: chỉ có `agent.spawn` (nhận params khác HLD), không có resume args handling
- `resolveAgentSpec()` không hỗ trợ `--resume` flag cho Claude

→ **BL-AG-03 chưa implement.**

---

### 9. BL-AG-04: Switch Account / Provider — THIẾU HOÀN TOÀN ❌

HLD mô tả:
```
pattern match rate-limit → emit: agent:rateLimited → alert user
User: [Switch account 2] [Switch provider] [Wait]
→ AgentManager.switchAccount()
    AIProviderResolver.resolve() với account mới
    conn.call('agent.kill') + conn.call('agent.spawn', { newEnv })
```

Thực tế grep `switchAccount`, `agent:rateLimited`, `AIProviderResolver`, `RATE_LIMIT_PATTERNS` (trong context agent orchestration):
- `RATE_LIMIT_PATTERNS` tồn tại trong `src/main/rate-limits/` nhưng chỉ cho UI rate limit display, không cho agent orchestration
- Không có `agent:rateLimited` event emission trong agent flow
- Không có `switchAccount()` trong bất kỳ AgentManager (vì AgentManager chưa tồn tại)

→ **BL-AG-04 chưa implement.**

---

### 10. BL-AG-05: Monitor Trạng thái Real-time — THIẾU OSC Parser ❌

HLD mô tả:
```
[AgentHookParser]
    Parse OSC 133 sequences: ESC]133;A ST → status "running"
    Pattern match: "waiting for input" → status "waiting"
    emit: agent:statusChanged { sessionId, status }
```

Thực tế:
- `AgentHookParser` class **không tồn tại** trong `src/main`
- OSC 133 parsing chỉ được thực hiện trong **terminal emulator** (`orca-runtime.ts`), không phải trong agent output pipeline
- Không có `agent:statusChanged` event anywhere trong main process code
- Không có `agent:rateLimited` event anywhere

→ **BL-AG-05 monitor pipeline hoàn toàn không implement.**

---

### 11. PTY_REGISTRY — Module-level Singleton (Memory Leak Risk) ⚠️

`agent-spawner.ts:50`:
```typescript
const PTY_REGISTRY = new Map<string, { pty, taskId, userId }>()
```

Vấn đề:
- Registry là **module-level singleton** → tồn tại cho toàn bộ lifetime của relay process
- Nếu relay restart (do SSH reconnect) → PTY_REGISTRY bị xóa, nhưng Orca server không biết → ptyId stale
- Không có cleanup khi WS disconnects → PTY orphaned nếu ws đóng trước khi agent exit

---

### 12. buildAgentEnv — Hardcode 'placeholder-key' ❌

`agent-spawner.ts:147`:
```typescript
const env = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)
```

Đã ghi nhận trong BUG-AG-ORCH-003. Bổ sung: không có gọi `handleReadCredential` để đọc actual API key, và kể cả nếu đọc được thì credential store trả về `encryptedBlob` (không phải plaintext key).

---

### 13. resolveAgentSpec — Thiếu codex, opencode ❌

`agent-spawner.ts:81-88`:
```typescript
export function resolveAgentSpec(modelId: string) {
  if (modelId.startsWith('claude')) { return { binary: 'claude', ... } }
  if (modelId.startsWith('gemini')) { return { binary: 'gemini', ... } }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}
```

Đã ghi nhận BUG-AG-ORCH-004. Bổ sung: cũng thiếu:
- `openai` → `codex` hoặc `opencode`
- Args `--no-cache` cho Claude là **không đúng** syntax (Claude CLI không có `--no-cache` flag, chỉ có `--output-format stream-json` và `--verbose`)

---

## Tổng hợp Bugs Mới Phát hiện

| ID | Severity | Vấn đề | File |
|----|----------|--------|------|
| **BUG-AG-ORCH-005** | 🔴 CRITICAL | `AgentManager` và `AgentConnectionManager` không tồn tại — BL-AG-01 broken | N/A |
| **BUG-AG-ORCH-006** | 🔴 CRITICAL | `agent.output` stream handler không có trên Orca server — PTY output bị mất | N/A |
| **BUG-AG-ORCH-007** | 🔴 HIGH | `AgentHookParser` và OSC 133 parser không tồn tại — BL-AG-05 broken | N/A |
| **BUG-AG-ORCH-008** | 🔴 HIGH | `orca_sessions` table cho agent sessions không tồn tại (bảng HTTP auth) | `0005_add_auth_schema.ts` |
| **BUG-AG-ORCH-009** | 🔴 HIGH | BL-AG-03 Resume Session chưa implement | N/A |
| **BUG-AG-ORCH-010** | 🔴 HIGH | BL-AG-04 Switch Account chưa implement | N/A |
| **BUG-AG-ORCH-011** | 🟡 MEDIUM | PTY_REGISTRY module-level → orphaned PTYs khi WS disconnect | `agent-spawner.ts` |
| **BUG-AG-ORCH-012** | 🟡 MEDIUM | Claude args `--no-cache` không đúng CLI syntax | `agent-spawner.ts:83` |
| **BUG-AG-ORCH-013** | 🟡 MEDIUM | relay-ws mode fallback token `'relay-secret'` hardcode | `agent-connection-relay.ts:26` |

---

## Tóm tắt Audit

| Nhóm | HLD có | Implement | Trạng thái |
|------|--------|-----------|------------|
| WS Connection Architecture | ✅ | ✅ DevServerRelayBridge 3 modes | ✅ DONE |
| Handshake Protocol | ✅ | ✅ ws-handshake.ts | ⚠️ 2 mode khác nhau |
| BL-AG-01: Agent Start | ✅ | ❌ AgentManager missing | 🔴 BROKEN |
| BL-AG-01: agent.spawn relay | ✅ | ⚠️ có nhưng params sai | 🔴 MISMATCH |
| BL-AG-01: Output stream | ✅ | ❌ No handler on Orca | 🔴 BROKEN |
| BL-AG-02: Agent Stop (graceful) | ✅ | ❌ sendInput missing | 🔴 BROKEN |
| BL-AG-02: Agent Kill | ✅ | ⚠️ SIGTERM thay SIGKILL | 🟡 PARTIAL |
| BL-AG-03: Resume Session | ✅ | ❌ Not implemented | 🔴 MISSING |
| BL-AG-04: Switch Account | ✅ | ❌ Not implemented | 🔴 MISSING |
| BL-AG-05: Status Monitor | ✅ | ❌ AgentHookParser missing | 🔴 BROKEN |
| Credential Read for Spawn | ✅ | ❌ placeholder-key hardcoded | 🔴 BROKEN |
| Agent Binary Resolution | ✅ | ⚠️ missing codex/opencode | 🟡 PARTIAL |

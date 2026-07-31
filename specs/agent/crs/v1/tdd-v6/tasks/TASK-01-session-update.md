# TASK-01: Update Agent Session Handshake — version + capabilities

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:44

**Phase:** 1 (Simple — 2 dòng thay đổi)
**File:** `src/relay/agent-session.ts`
**Operation:** MODIFY
**CR:** [CR-AG-01](../solutions/CR-AG-01-architecture.md) + [CR-AG-04](../solutions/CR-AG-04-session.md)
**TDD:** TDD-AG-01, TDD-AG-04
**Depends on:** Không có dependency
**Blocked by:** Không

---

## Mục tiêu

Cập nhật `agent-session.ts` để handshake frame phản ánh v5.0 capabilities:
- `agentVersion`: `'2.1.0'` → `'5.0.0'`
- `capabilities`: thêm `'ai.providers'`, `'agent.spawn'`, `'worktrees'`

---

## Context đọc trước

Đọc file này để hiểu vị trí cần sửa:

**File:** `src/relay/agent-session.ts` — **lines 54–72** (function `sendHandshake`)

```typescript
// Đây là đoạn code HIỆN TẠI cần sửa (lines 54-72):
function sendHandshake(ws: WebSocket, wireState: ReturnType<typeof createWireState>): void {
  const rpc = {
    jsonrpc: '2.0' as const,
    id: 1,
    method: AGENT_HANDSHAKE_METHOD,
    params: {
      agentVersion:  '2.1.0',                                 // ← CHANGE THIS
      platform:      process.platform,
      arch:          process.arch,
      nodeVersion:   process.version,
      capabilities:  ['fs', 'git', 'preflight'] as const,    // ← CHANGE THIS
      ...(config.agentToken ? { agentToken: config.agentToken } : {}),
      devServerId:   config.devServerId,
      tools:         tools.map(t => t.name),
    },
  }
  ws.send(encodeDataFrame(wireState, JSON.stringify(rpc)))
  log.info(`Handshake sent: devServerId=${config.devServerId} tools=[${tools.map(t => t.name).join(',')}]`)
}
```

---

## Thay đổi cần thực hiện

### Edit 1 — `src/relay/agent-session.ts` line 60

```diff
-      agentVersion:  '2.1.0',
+      agentVersion:  '5.0.0',
```

### Edit 2 — `src/relay/agent-session.ts` line 64

```diff
-      capabilities:  ['fs', 'git', 'preflight'] as const,
+      capabilities:  ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees'] as const,
```

---

## Verify

Sau khi sửa, build và kiểm tra không có lỗi TypeScript:

```bash
# Từ repo root:
pnpm run build:relay
# Hoặc:
npx tsc --noEmit -p config/tsconfig.node.json
```

Kiểm tra output file confirm change:

```bash
grep -n "agentVersion\|capabilities" src/relay/agent-session.ts
# Expected:
#   60:      agentVersion:  '5.0.0',
#   64:      capabilities:  ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees'] as const,
```

---

## Done criteria

- [ ] `agentVersion` = `'5.0.0'`
- [ ] `capabilities` array có đủ 6 items: `'fs'`, `'git'`, `'preflight'`, `'ai.providers'`, `'agent.spawn'`, `'worktrees'`
- [ ] TypeScript compile không lỗi

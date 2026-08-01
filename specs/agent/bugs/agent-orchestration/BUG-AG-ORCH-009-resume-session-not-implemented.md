# BUG-AG-ORCH-009: BL-AG-03 Resume Session chưa implement — thiếu resumeArgs, agent session lookup

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AG-03):
```
[Main Process — AgentManager.resume()]
    SELECT sessionId, devServerId FROM orca_sessions
        WHERE worktreeId=? ORDER BY startedAt DESC
    Build resume args:
        Claude: ["claude", "--resume", sessionId]
        Codex:  ["codex", "--session-file", sessionFilePath]
    conn.call('agent.spawn', { agentBinary, args: [...resumeArgs], cwd, env, userId })
```

Thực tế:
1. `AgentManager.resume()` **không tồn tại** (AgentManager class chưa implement — BUG-AG-ORCH-005)
2. Bảng `orca_agent_sessions` chưa tồn tại (BUG-AG-ORCH-008)
3. `resolveAgentSpec()` không support `--resume` flag:
   ```typescript
   // agent-spawner.ts:83
   return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
   // ← không có '--resume', không nhận resumeArgs
   ```
4. `agent.spawn` handler (`agent-rpc-dispatch.ts:462`) nhận params `{ taskId, userId, modelId, accountId, cwd }` — **không có** `args` field để pass resume args

## Ảnh hưởng

1. User không thể resume agent session đã bị interrupt
2. Codex session files sẽ không bao giờ được resume
3. Khi Orca server restart → tất cả active sessions bị mất, không thể recover

## Fix đề xuất

**Relay side**: Cần thêm `args` field vào `AgentSpawnRequest` và handler:
```typescript
// agent-spawner.ts
export interface AgentSpawnRequest {
  taskId:    string
  userId:    string
  modelId:   string
  accountId: string
  cwd?:      string
  args?:     string[]    // ← thêm optional resume args
}
```

**`resolveAgentSpec`**: Thêm resume args builder:
```typescript
export function buildResumeArgs(agentType: string, sessionId: string): string[] {
  if (agentType === 'claude') return ['--resume', sessionId]
  if (agentType === 'codex')  return ['--session-file', `~/.codex/${sessionId}.json`]
  return []
}
```

## Liên quan đến luồng

- **BL-AG-03**: Resume Session — not implemented

---

## ⏸ Fix Status: DEFERRED

**Reason:** Resume session infrastructure (session store + Orca Server call) deferred to Phase 3. Partial: AgentSpawnRequest.resumeId field added, buildAgentArgs handles --resume flag for claude.

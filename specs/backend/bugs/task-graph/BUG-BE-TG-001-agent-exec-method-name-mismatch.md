# BUG-BE-TG-001: `ProfileAwareAgentSpawner.spawn()` gọi relay với `agent.exec` — nhưng Dev Server relay chỉ register `agent.execNonInteractive`

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TG-001  
**Note:** agent-rpc-dispatch.ts: method name fixed  

## Mức độ: 🔴 CRITICAL (Integration Break)

## Tóm tắt

`ProfileAwareAgentSpawner` (được dùng bởi `TaskAgentExecutor`, `WorkflowOrchestrator`) gọi:
```typescript
// ProfileAwareAgentSpawner.ts:106
const result = await relay.call('agent.exec', {
  command,
  workdir: workdir ?? project.repoPath,
  env: profileEnv,
})
```

Nhưng relay binary (`relay.ts`) **không có handler** cho `agent.exec`. Handlers thực tế là:
```typescript
// agent-exec-handler.ts:140,143
dispatcher.onRequest('agent.execNonInteractive', ...)
dispatcher.onRequest('agent.cancelExec', ...)
```

Không có `agent.exec` → relay sẽ trả `METHOD_NOT_FOUND` error.

## File liên quan

- [`src/main/project/ProfileAwareAgentSpawner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/project/ProfileAwareAgentSpawner.ts) — Line 106: `relay.call('agent.exec', ...)`
- [`src/relay/agent-exec-handler.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-exec-handler.ts) — Lines 140, 143: chỉ có `agent.execNonInteractive` và `agent.cancelExec`

## Ảnh hưởng

1. **Task execution (`task.execute` RPC)** → `TaskAgentExecutor.executeTask()` → `agentSpawner.spawn()` → relay error → agent không spawn được.
2. **Workflow step executor** (`StepExecutors.ts`) cũng dùng `relay.call('agent.exec', ...)` → same failure.
3. **AI commit message** (`git.generateCommitMessage`) cũng sẽ fail nếu dùng agent exec.
4. Toàn bộ BL-TG-04 (Task → Agent Execution) không hoạt động dù code đã implement.

## Sai khác cụ thể

| Caller | RPC method gọi | Handler thực tế trong relay |
|--------|---------------|---------------------------|
| `ProfileAwareAgentSpawner.spawn()` | `agent.exec` | ❌ Không tồn tại |
| `StepExecutors.ts` | `agent.exec` | ❌ Không tồn tại |
| Relay handler | `agent.execNonInteractive` | ✅ Tồn tại |

## Cách fix

**Option A**: Rename relay call để match:
```typescript
// ProfileAwareAgentSpawner.ts:106
const result = await relay.call('agent.execNonInteractive', {
  command,
  workdir: workdir ?? project.repoPath,
  env: profileEnv,
})
```

**Option B**: Thêm `agent.exec` handler vào relay để forward sang `execNonInteractive`.

## Liên quan đến luồng

- **BL-TG-04**: Task → Agent Execution — toàn bộ luồng bị broken do method name mismatch.
- **BL-WF-02**: Workflow step `type: 'agent'` — cũng broken.

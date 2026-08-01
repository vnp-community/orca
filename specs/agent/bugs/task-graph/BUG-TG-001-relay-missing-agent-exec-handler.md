# BUG-WF-002 [AGENT]: relay dispatch thiếu handler `agent.exec` — `StepExecutors.executeAgent()` sẽ fail

## Mức độ: 🔴 CRITICAL

## Tóm tắt

`src/main/workflow/StepExecutors.ts:88`:
```typescript
const result = await relay.call('agent.exec', {
  stepId: step.id,
  prompt: step.config['prompt'],
  worktreePath: step.config['worktreePath'],
  trustPreset: step.config['trustPreset'] ?? 'default',
})
```

Kiểm tra relay dispatch handlers:
```
case 'agent.spawn':  ✅ (line 462) — spawn agent PTY
case 'agent.kill':   ✅ (line 475) — kill agent
case 'agent.exec':   ❌ MISSING
```

**`agent.exec` không có handler** trong `src/relay/agent-rpc-dispatch.ts`.

Chú ý: `agent.execNonInteractive` có tại `src/relay/agent-exec-handler.ts:140` nhưng được đăng ký qua `dispatcher.onRequest()`, không phải qua switch-case trong `agent-rpc-dispatch.ts`.

Cần kiểm tra xem `agent.exec` có được register ở đâu không.

## Root Cause

`ProfileAwareAgentSpawner` cũng gọi `relay.call('agent.exec', {...})` (line 106). Nếu relay dispatch không handle nó, **mọi agent spawn từ task execution (BL-TG-04) và workflow steps (BL-WF-02) đều fail**.

Chỉ `agent.spawn` (spawn PTY shell + launch agent separately) có trong dispatch, nhưng `agent.exec` (execute agent command non-interactively) là missing.

## Fix đề xuất

Đăng ký `agent.exec` trong relay dispatch:
```typescript
// agent-rpc-dispatch.ts
case 'agent.exec': {
  // Delegate to agent-exec-handler
  return agentExecHandler.execInteractive(rpc.params)
}
```

Hoặc kiểm tra xem `agent-exec-handler.ts` có dispatch thống nhất không.

## Files liên quan

- `src/relay/agent-rpc-dispatch.ts`: thiếu case 'agent.exec'
- `src/relay/agent-exec-handler.ts:140`: agentExecNonInteractive handler
- `src/main/project/ProfileAwareAgentSpawner.ts:106`: gọi agent.exec
- `src/main/workflow/StepExecutors.ts:88`: gọi agent.exec

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** agent-exec-handler.ts: handleAgentExec (non-interactive exec with child_process). Dispatch: 'agent.exec' + 'agent.execNonInteractive' cases added.

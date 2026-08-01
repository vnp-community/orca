# BUG-BE-TG-002: `TaskAIPlanner.decompose()` gọi `relay.call('ai.complete')` — nhưng relay không register handler `ai.complete`

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TG-002  
**Note:** ai-complete-handler.ts: ai.complete RPC handler created  

## Mức độ: 🔴 HIGH (Integration Break)

## Tóm tắt

`TaskAIPlanner.decompose()` thực hiện:
```typescript
// TaskAIPlanner.ts:54
const response = (await relay.call('ai.complete', {
  prompt,
  format: 'json',
  taskId,
})) as { content?: string; text?: string } | string
```

Grep `relay.ts` và toàn bộ `src/relay/`:
```
dispatcher.onRequest('ai.complete', ...) → No results
'ai.complete'                            → No results
```

Relay handlers hiện tại:
- `session.registerRoot`, `session.resolveHome`
- `orca.cli`
- `relay.status`
- `agent.execNonInteractive`, `agent.cancelExec`
- `AGENT_HOOK_*` methods

**Không có `ai.complete`** → relay sẽ trả `METHOD_NOT_FOUND`.

## File liên quan

- [`src/main/task/TaskAIPlanner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/task/TaskAIPlanner.ts) — Line 54: `relay.call('ai.complete', ...)`
- [`src/relay/relay.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/relay.ts) — Không có `ai.complete` handler

## Ảnh hưởng

1. **BL-TG-02** Task decomposition bằng AI → `POST /task/:id/ai-plan` → `TaskAIPlanner.decompose()` → relay error.
2. Toàn bộ AI-powered task planning không hoạt động.
3. `git.generateCommitMessage` trong `git-remote.ts` dùng cùng `relay.call('ai.complete', ...)` → cũng bị ảnh hưởng.

## Nguyên nhân

Có thể `ai.complete` cần được implement trong relay như một adapter gọi agent exec hoặc gọi AI provider API trực tiếp. Nhưng handler chưa được tạo trong `src/relay/`.

## Cách fix đề xuất

Thêm `ai.complete` handler trong relay:
```typescript
// src/relay/ai-complete-handler.ts
dispatcher.onRequest('ai.complete', async (params) => {
  const { prompt, format } = params as { prompt: string; format?: 'json' | 'text' }
  // Gọi AI provider API (đọc từ credential store)
  // Return { content: string }
})
```

## Liên quan đến luồng

- **BL-TG-02**: AI task planning — relay `ai.complete` missing.
- **BL-PW-03**: AI commit message — same issue.

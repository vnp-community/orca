# BUG-BE-TM-006: `terminal.create` không có RBAC `checkScopedTokenPermission` — tài liệu HLD mô tả nhưng code không implement

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-006  
**Note:** terminal.ts: ctx.userId RBAC guard  

## Mức độ: MEDIUM

## Tóm tắt

Theo `terminal-create-flow.md §Bước 2`, User Process phải thực hiện RBAC check:
```
[RBAC] checkScopedTokenPermission('terminal.create', serverId)
    Scoped token phải có allowedServerIds chứa devServerId
    FAIL? → RPC error { code: -32003, message: 'forbidden' }
```

Khi rà soát code, **không tìm thấy** function `checkScopedTokenPermission` hay bất kỳ RBAC guard nào cho `terminal.create` RPC method.

## File liên quan

- [`src/main/runtime/rpc/methods/terminal.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/runtime/rpc/methods/terminal.ts) — Lines 1284-1303
- [`src/main/session/user-process-entry.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/user-process-entry.ts) — (không có RBAC)

## Code thực tế

```typescript
// terminal.ts:1284-1303
defineMethod({
  name: 'terminal.create',
  params: TerminalCreateParams,
  handler: async (params, { runtime }) => ({
    terminal: await runtime.createTerminal(params.worktree, {
      // ... không có permission check nào
    })
  })
})
```

## Hành vi đúng theo HLD

```
OrcaRuntimeRpcServer:
  ├─ [AUTH] parseAndAuth(ORCA_RPC_AUTH_TOKEN)
  │    FAIL? → disconnect 'forbidden'
  │
  ├─ [RBAC] checkScopedTokenPermission('terminal.create', serverId)
  │    Scoped token phải có allowedServerIds chứa devServerId
  │    FAIL? → RPC error { code: -32003, message: 'forbidden' }
  │
  └─ dispatcher.dispatch('terminal.create', params)
```

## Ảnh hưởng

1. **Bảo mật**: Bất kỳ authenticated user nào cũng có thể tạo terminal trên bất kỳ Dev Server nào, không có constraint về allowedServerIds.
2. **Thiếu isolation**: User A có thể tạo terminal trên server của User B nếu biết serverId.
3. **Thiếu error path**: `RPC error { code: -32003 }` không bao giờ được emit → Error handling ở Browser không được kích hoạt.

## Liên quan đến luồng

- **BL-TM-01**: Bước 2 — RPC Dispatch, RBAC check.
- **Trace span**: Không có span cho RBAC failure.

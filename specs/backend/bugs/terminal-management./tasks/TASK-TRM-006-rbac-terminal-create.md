# TASK-TRM-006: Thêm RBAC check cho terminal.create RPC

**Priority:** 🔴 CRITICAL — bảo mật: user có thể tạo terminal trên bất kỳ server nào  
**Effort:** ~20 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TM-006  
**Solution ref:** [SOLUTION-TRM-BE-exact.md](../solutions/SOLUTION-TRM-BE-exact.md)

---

## Mục tiêu

Thêm RBAC check `allowedServerIds` vào `terminal.create` RPC handler để đảm bảo scoped token chỉ có thể tạo terminal trên các Dev Servers được phép.

## Bước 1 — Tìm vị trí chính xác

```bash
grep -n "terminal.create\|'terminal.create'" src/main/runtime/rpc/methods/terminal.ts | head -5
```

Expected output: dòng có `defineMethod` với `name: 'terminal.create'`

## Bước 2 — File cần sửa

```
src/main/runtime/rpc/methods/terminal.ts
```

## Thay đổi cụ thể

Trong `handler` của `terminal.create`, thêm permission check TRƯỚC khi gọi `runtime.createTerminal()`:

**TRƯỚC:**
```typescript
defineMethod({
  name: 'terminal.create',
  params: TerminalCreateParams,
  handler: async (params, { runtime }) => ({
    terminal: await runtime.createTerminal(params.worktree, {
      // ... params
    })
  })
})
```

**SAU:**
```typescript
defineMethod({
  name: 'terminal.create',
  params: TerminalCreateParams,
  handler: async (params, { runtime, rpcAuthContext }) => {
    // FIX BE-TM-006: RBAC check for scoped tokens
    if (rpcAuthContext?.scopedToken?.allowedServerIds) {
      const { allowedServerIds } = rpcAuthContext.scopedToken
      const devServerId = params.devServerId ?? params.worktree?.devServerId
      if (devServerId && !allowedServerIds.includes(devServerId)) {
        throw { code: -32003, message: `forbidden: devServerId '${devServerId}' not in allowedServerIds` }
      }
    }

    return {
      terminal: await runtime.createTerminal(params.worktree, {
        // ... existing params unchanged
      })
    }
  }
})
```

## Note về rpcAuthContext

Nếu `rpcAuthContext` chưa tồn tại trong handler context type, cần kiểm tra:
```bash
grep -n "rpcAuthContext\|RpcHandlerContext\|RpcContext" src/main/runtime/rpc/core.ts | head -10
```

Nếu type chưa có field này, thêm vào `RpcHandlerContext` type:
```typescript
// src/main/runtime/rpc/core.ts — thêm vào interface:
export interface RpcHandlerContext {
  runtime: OrcaRuntimeService
  // ... existing fields
  rpcAuthContext?: {
    scopedToken?: {
      allowedServerIds?: string[]
    }
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm vitest run src/main/runtime/rpc/__tests__/ 2>/dev/null || echo "No test dir yet"

# Manual verify: call terminal.create với devServerId không có trong allowedServerIds
# Expected: RPC error { code: -32003, message: 'forbidden: ...' }
```

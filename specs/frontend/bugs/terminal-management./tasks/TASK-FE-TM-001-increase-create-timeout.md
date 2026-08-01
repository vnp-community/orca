# TASK-FE-TM-001: Tăng timeout cho terminal.create RPC call

**Priority:** 🟠 HIGH — Terminal create timeout quá ngắn  
**Effort:** ~10 phút  
**Status:** ✅ DONE — Implemented  
**Bug refs:** BUG-FE-TM-001  
**Solution ref:** [SOL-FE-TM-001](../solutions/SOL-FE-TM-001-increase-terminal-create-timeout.md)

## Bước 1 — Tìm timeout value

```bash
grep -rn "terminal.create\|createTerminal\|timeout.*terminal\|terminal.*timeout" \
  src/renderer/src/ --include="*.ts" --include="*.tsx" | head -10
```

## Thay đổi

```typescript
// TRƯỚC (timeout quá ngắn):
const terminal = await rpc.call('terminal.create', params, { timeout: 5000 })  // 5s

// SAU (30s để account cho remote Dev Server startup):
const terminal = await rpc.call('terminal.create', params, { timeout: 30_000 })  // 30s
```

## Verification

```bash
pnpm tsc --noEmit
# Test: create terminal trên slow remote Dev Server → không timeout
```

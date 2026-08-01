# BUG-AG-TM-002: `pty.spawn` thiếu SecureFs path traversal validation cho `cwd`

## Mức độ: HIGH

## Tóm tắt

HLD mô tả (terminal-create-flow.md §Bước 5):
```
[ISOLATION CHECK] PTY ownership check: ptyId bound to userId
    SecureFs.validatePath(cwd, projectRoot + allowedRoots)
    FAIL (path traversal)? → error 'path not allowed'
```

Trong `pty-handler.ts::spawn()`, `cwd` được lấy trực tiếp từ params và truyền vào `node-pty.spawn()` mà **không có bất kỳ path validation nào**:

```typescript
const cwd = (params.cwd as string) || resolveDefaultCwd()
// ...
const term = pty.spawn(shell, shellLaunch.args, {
  cwd,  // ← không validate path traversal
  // ...
})
```

## File liên quan

- [`src/relay/pty-handler.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/pty-handler.ts) — Lines 613-680

## Code sai

```typescript
// Line 615
const cwd = (params.cwd as string) || resolveDefaultCwd()
// Không có:
// - SecureFs.validatePath(cwd, ...)
// - Path normalization / resolve
// - allowedRoots check
// - path traversal detection (../../..)
```

## Ảnh hưởng

1. **Path traversal**: Caller có thể gửi `cwd: '/etc'` hoặc `cwd: '../../sensitive'` → PTY spawn trong thư mục nhạy cảm.
2. **Security**: Nếu attacker có thể send `pty.spawn`, có thể spawn shell trong `/root`, `/etc`, v.v.
3. **Multi-user isolation**: User A có thể spawn terminal trong home directory của User B.

## Cách fix đề xuất

```typescript
import { SecureFs } from '../secure-fs'  // hoặc implement inline

const rawCwd = (params.cwd as string) || resolveDefaultCwd()
const allowedRoots = [workspace.path, homedir()]
const cwd = SecureFs.validatePath(rawCwd, allowedRoots) 
  ?? resolveDefaultCwd()
```

## Liên quan đến luồng

- **BL-TM-01**: Bước 5 — ISOLATION CHECK.
- **Trace span**: `error 'path not allowed'` không bao giờ emit.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** pty-handler.ts: validatePtyCwd(rawCwd) rejects ../ and absolute paths outside workspace. Returns safe validated path.

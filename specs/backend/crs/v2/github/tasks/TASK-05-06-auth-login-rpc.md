# TASK-05 & TASK-06: RPC Methods — `github.startAuthLogin` và `gitlab.startAuthLogin`

**Status:** ✅ DONE — 2026-07-25 (AC verified 2026-07-25)  
**Phase:** 3 — Orca Server Proxy  
**Priority:** 🟠 High  
**Depends on:** TASK-01 (RpcContext extension)  
**Solution:** SOL-03-Remote-PTY.md  
**CRs:** CR-GH-002, CR-INT-001  
**Estimated effort:** ~45 phút

---

## Mục tiêu

Tạo 2 RPC methods mới để cho phép user thực hiện `gh auth login` và `glab auth login` qua PTY trên Dev Server. User sẽ nhận `ptyId` và theo dõi quá trình xác thực qua Terminal UI (xterm.js).

---

## File cần tạo: `src/main/runtime/rpc/methods/github-auth.ts` [NEW]

```typescript
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { requiredString } from '../schemas'

const StartAuthLoginParams = z.object({
  devServerId: requiredString('Missing devServerId'),
  host: z.string().optional()  // Custom GitHub Enterprise host
})

const RevokeAuthParams = z.object({
  devServerId: requiredString('Missing devServerId'),
  host: z.string().optional()
})

export const GITHUB_AUTH_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'github.startAuthLogin',
    params: StartAuthLoginParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error('devServerManager not available — not running in Web mode')
      }

      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(`Dev server '${params.devServerId}' relay not connected`)
      }

      const args = ['auth', 'login']
      if (params.host) {
        args.push('--hostname', params.host)
      }

      // Spawn interactive PTY process trên relay (Dev Server)
      // Sử dụng Device Flow (không cần web browser trực tiếp):
      // gh auth login --git-protocol https --web (hiển thị Device Code)
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'gh',
        args,
        cols: 120,
        rows: 30
      })

      return { ptyId, devServerId: params.devServerId }
    }
  }),

  defineMethod({
    name: 'github.revokeAuth',
    params: RevokeAuthParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error('devServerManager not available')
      }

      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(`Dev server '${params.devServerId}' relay not connected`)
      }

      // gh auth logout — fire and forget (PTY không cần interactive)
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'gh',
        args: params.host ? ['auth', 'logout', '--hostname', params.host] : ['auth', 'logout'],
        cols: 80,
        rows: 10
      })

      return { ptyId, devServerId: params.devServerId }
    }
  })
]
```

---

## File cần tạo: `src/main/runtime/rpc/methods/gitlab-auth.ts` [NEW]

```typescript
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { requiredString } from '../schemas'

const StartAuthLoginParams = z.object({
  devServerId: requiredString('Missing devServerId'),
  host: z.string().optional()  // Custom GitLab self-hosted host
})

export const GITLAB_AUTH_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'gitlab.startAuthLogin',
    params: StartAuthLoginParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error('devServerManager not available — not running in Web mode')
      }

      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(`Dev server '${params.devServerId}' relay not connected`)
      }

      const args = ['auth', 'login']
      if (params.host) {
        args.push('--hostname', params.host)
      }

      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'glab',
        args,
        cols: 120,
        rows: 30
      })

      return { ptyId, devServerId: params.devServerId }
    }
  }),

  defineMethod({
    name: 'gitlab.revokeAuth',
    params: StartAuthLoginParams,
    handler: async (params, ctx) => {
      if (!ctx.devServerManager) {
        throw new Error('devServerManager not available')
      }
      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        throw new Error(`Dev server '${params.devServerId}' relay not connected`)
      }
      const ptyId = await relay.call<string>('pty.spawn', {
        command: 'glab',
        args: params.host ? ['auth', 'logout', '--hostname', params.host] : ['auth', 'logout'],
        cols: 80,
        rows: 10
      })
      return { ptyId, devServerId: params.devServerId }
    }
  })
]
```

---

## Cập nhật `src/main/runtime/rpc/methods/index.ts`

Thêm 2 method groups mới vào `ALL_RPC_METHODS`:

```typescript
import { GITHUB_AUTH_METHODS } from './github-auth'
import { GITLAB_AUTH_METHODS } from './gitlab-auth'

export const ALL_RPC_METHODS = [
  // ... existing methods ...
  ...GITHUB_AUTH_METHODS,    // THÊM
  ...GITLAB_AUTH_METHODS,    // THÊM
]
```

---

## Xác nhận `relay.call('pty.spawn', ...)` API

Cần tìm API thực tế của relay:

```bash
grep -rn "pty.spawn\|pty\.create\|createPty" \
  /Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/ssh/ \
  --include="*.ts" | head -20
```

Nếu method name khác (`pty.create` thay vì `pty.spawn`), điều chỉnh code tương ứng.

---

## Acceptance Criteria

1. ✅ `github.startAuthLogin({ devServerId })` trả về `{ ptyId, devServerId }`
2. ✅ `gitlab.startAuthLogin({ devServerId })` trả về `{ ptyId, devServerId }`
3. ✅ PTY process được tạo trên Dev Server (relay.call pty.spawn được gọi)
4. ✅ Không có lỗi TypeScript sau TASK-01 (ctx.devServerManager có type)
5. ✅ params dùng `requiredString()` helper đúng chuẩn của codebase
6. ✅ Build thành công: `pnpm build:backend`

---

## Files cần tạo/sửa

- `src/main/runtime/rpc/methods/github-auth.ts` [NEW]
- `src/main/runtime/rpc/methods/gitlab-auth.ts` [NEW]
- `src/main/runtime/rpc/methods/index.ts` [MODIFY] — thêm 2 method groups

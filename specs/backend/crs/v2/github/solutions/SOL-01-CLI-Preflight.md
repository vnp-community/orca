# Solution cho CR-GH-001 & CR-INT-001: Proxy CLI Preflight qua SSH Relay

## Bối cảnh & TDD Specs liên kết
- Dựa trên **TDD-05 (SSH Relay)**: Orca-relay binary được deploy lên remote server và expose các endpoint qua SSH.
- Dựa trên **TDD-11 (Web Server Mode)**: Multi-user mode spawn các tiến trình tách biệt cho mỗi người dùng, `ORCA_USER_ID` được truyền qua biến môi trường.

## Thiết kế giải pháp

Thay vì thực thi các lệnh `gh` hoặc `glab` trên Orca Server Container, chúng ta sẽ mở rộng giao thức RPC của SSH Relay để cho phép chạy các lệnh `preflight` trực tiếp trên Dev Server.

### 1. Mở rộng orca-relay (Dev Server side)
Thêm endpoint mới `preflight.check.cli` vào `orca-relay` binary.

```typescript
// orca-relay/src/handlers/preflight.ts
export const handlePreflightCheckCli = async (params: { force?: boolean }) => {
  const [gitInstalled, ghInstalled, glabInstalled] = await Promise.all([
    isCommandAvailable('git'),
    isCommandAvailable('gh'),
    isCommandAvailable('glab')
  ]);

  const [ghAuth, glabAuth] = await Promise.all([
    ghInstalled ? runCommandCatch('gh', ['auth', 'status']) : false,
    glabInstalled ? runCommandCatch('glab', ['auth', 'status']) : false
  ]);

  return {
    git: { installed: gitInstalled },
    gh: { installed: ghInstalled, authenticated: ghAuth },
    glab: { installed: glabInstalled, authenticated: glabAuth }
  };
};
```

### 2. Sửa đổi RPC Method trên Orca Server
Tận dụng `devServerManager` để proxy request sang relay.

```typescript
// src/main/runtime/rpc/methods/preflight.ts
defineMethod({
  name: 'preflight.check',
  params: z.object({
    force: z.boolean().optional(),
    devServerId: z.string().optional()
  }),
  handler: async (params, context) => {
    if (params.devServerId) {
      const relay = context.devServerManager.getRelay(params.devServerId);
      if (!relay) throw new Error('Relay not connected');
      
      const cliStatus = await relay.call('preflight.check.cli', { force: params.force });
      // Merge với status của Bitbucket/Azure/Gitea trên Orca Server
      const apiStatus = await getApiIntegrationStatuses(context.userId);
      
      return { ...cliStatus, ...apiStatus };
    }
    // Fallback cho local mode
    return runLocalPreflightCheck(params.force);
  }
});
```

## Ưu điểm
- Giải quyết triệt để lỗi binary không tồn tại trên Orca Server.
- Không cần parse output của command từ client, mọi logic nằm trong relay handler.

---

## ✅ Implementation Status — COMPLETED (2026-07-25)

### Thay đổi so với thiết kế ban đầu

> **Ghi chú quan trọng:** Relay đã có sẵn handler `preflight.check` (không phải `preflight.check.cli`). Thay vì tạo endpoint mới, chúng ta mở rộng handler hiện có để bao gồm `glab`.

### Files đã implement

#### `src/relay/preflight-handler.ts` [MODIFIED]
- ✅ Thêm `checkGlabCli()` method (line 234–252): check `glab --version` và `glab auth status`
- ✅ Sửa `checkFullPreflight()` để include `glab` result trong response
- ✅ Response format: `{ platform, gh, glab, git }` — `glab` field mới

#### `src/main/runtime/rpc/methods/preflight.ts` [MODIFIED]
- ✅ Mở rộng `PreflightCheck` schema thêm `devServerId?: z.string()`
- ✅ Handler proxy sang relay khi `params.devServerId && ctx.devServerManager`
- ✅ Gọi `relay.call('preflight.check', {}, 30_000)` — 30s timeout
- ✅ Fallback sang `runPreflightCheck()` khi không có `devServerId`

#### `src/relay/preflight-handler.test.ts` [MODIFIED]
- ✅ 3 test cases cho `glab` CLI detection: authenticated / not-authenticated / not-installed

#### `src/main/runtime/rpc/methods/preflight.test.ts` [MODIFIED]
- ✅ 3 test cases TASK-04: relay proxy / fallback / relay not connected

### Kết quả test
```
✅ preflight.test.ts — 21/21 tests pass (bao gồm 3 test mới TASK-04)
✅ preflight-handler.test.ts — glab tests implemented
```

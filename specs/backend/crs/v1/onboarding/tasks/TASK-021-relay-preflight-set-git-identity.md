# TASK-021: Sửa `src/relay/preflight-handler.ts` — Thêm `setGitIdentity`

**Phase:** 2 — Remote Preflight  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §B.3  
**Depends on:** TASK-020 (cùng file)  
**Blocks:** TASK-022

---

## Mục tiêu

Thêm method `setGitIdentity()` vào relay để frontend có thể set `git config --global user.name` và `user.email` trên remote dev server.

---

## File cần sửa

**Path:** `src/relay/preflight-handler.ts`

---

## Thay đổi cần thực hiện

```typescript
// Trong class PreflightHandler:

private async setGitIdentity(params: {
  name: string
  email: string
}): Promise<void> {
  await execFileAsync('git', ['config', '--global', 'user.name', params.name])
  await execFileAsync('git', ['config', '--global', 'user.email', params.email])
}

// Register:
this.dispatcher.onRequest('preflight.setGitIdentity', (p) =>
  this.setGitIdentity(p as { name: string; email: string })
)
```

---

## Acceptance Criteria

- [x] `preflight.setGitIdentity` được đăng ký trên dispatcher
- [x] `name` và `email` được set đúng thông qua `git config --global`
- [x] Lỗi từ `git config` được propagate (không swallow)
- [x] TypeScript compile thành công

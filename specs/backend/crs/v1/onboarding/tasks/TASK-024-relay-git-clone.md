# TASK-024: Sửa `src/relay/git-handler.ts` — Thêm `git.clone` với PTY streaming

**Phase:** 2 — Remote Repo  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §C.2  
**Depends on:** (không — relay-side)  
**Blocks:** TASK-025

---

## Mục tiêu

Thêm method `cloneRepo()` vào relay git handler, tạo PTY để stream `git clone --progress` output về caller.

---

## File cần sửa/tạo

**Path:** `src/relay/git-handler.ts` (MODIFY nếu đã tồn tại)

---

## Context cần tra cứu

1. Tìm cách relay tạo PTY hiện tại: grep `createPty` trong `src/relay/`
2. Tìm pattern gọi PTY với progress streaming trong các handler hiện có

---

## Nội dung cần implement

```typescript
// Trong class GitHandler:

async cloneRepo(params: {
  url: string
  targetPath: string
}): Promise<{ path: string }> {
  // Dùng PTY của relay để stream git clone --progress
  // Tìm đúng method tạo PTY trong relay (thay thế createPty nếu tên khác)
  const pty = this.createRelayPty({
    command: 'git',
    args: ['clone', '--progress', params.url, params.targetPath],
    env: {}
  })

  // Chờ PTY hoàn thành (exit code 0 = success)
  await pty.waitForExit()

  return { path: params.targetPath }
}

// Register:
this.dispatcher.onRequest('git.clone', (p) =>
  this.cloneRepo(p as { url: string; targetPath: string })
)
```

---

## Acceptance Criteria

- [x] `git.clone` được đăng ký trên dispatcher
- [x] Clone thành công → return `{ path: targetPath }`
- [x] Git error (exit code != 0) → throw Error với output từ git
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Đọc relay codebase để hiểu cách tạo PTY hiện tại (có thể là `this.ptyManager.create()` hoặc khác)
2. Nếu relay không có PTY mechanism, implement bằng `execFile` với progress parsing từ stderr
3. Ưu tiên PTY nếu có (streaming progress về frontend)

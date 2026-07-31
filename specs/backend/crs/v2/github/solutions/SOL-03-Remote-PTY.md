# Solution cho CR-GH-002: Remote PTY cho Auth Login

## Bối cảnh & TDD Specs liên kết
- Theo **TDD-05 (SSH Relay)**: `SshRelaySession` cung cấp các hàm `createPty`, `writePty`, `resizePty`, `killPty`.
- Thay vì gọi lệnh spawn trực tiếp trên Orca Server, chúng ta sử dụng PTY trên Relay để thực hiện luồng tương tác của CLI login (thiết bị/trình duyệt nhập code, v.v.).

## Thiết kế giải pháp

Tạo một Endpoint RPC mới trên Orca Server để proxy luồng PTY này.

### 1. RPC Handler `github.startAuthLogin` / `gitlab.startAuthLogin`
```typescript
defineMethod({
  name: 'github.startAuthLogin',
  params: z.object({
    devServerId: z.string(),
    host: z.string().optional()
  }),
  handler: async (params, context) => {
    const relay = context.devServerManager.getRelay(params.devServerId);
    if (!relay) throw new Error('Relay not connected');

    const args = ['auth', 'login'];
    if (params.host) {
      args.push('--hostname', params.host);
    }

    // Yêu cầu SSH Relay spawn tiến trình với PTY
    const ptyId = await relay.call('pty.spawn', {
      command: 'gh',
      args: args,
      cols: 120,
      rows: 30
    });
    
    // UI client sẽ nhận ptyId và subscribe vào PTY stream
    return { ptyId, devServerId: params.devServerId };
  }
});
```

### 2. Luồng hoạt động trên Client (Browser)
1. User click "Login with GitHub" trên Settings UI.
2. Trình duyệt gọi RPC `github.startAuthLogin({ devServerId: "DS123" })`.
3. Trình duyệt nhận `ptyId` trả về.
4. UI mở một cửa sổ Terminal (xterm.js) hoặc bắt output để hiển thị mã Device Code cho user.
5. User đọc Device Code và vào trình duyệt (GitHub/GitLab) để authorize.
6. Lệnh `gh auth login` kết thúc, Terminal đóng, tích hợp hoàn tất.

## Ưu điểm
- Hoạt động mượt mà như Local Mode vì PTY stream được bridge qua WebSocket.
- Không cần parse Device Code thủ công, giữ đúng trải nghiệm mặc định của `gh`/`glab`.

---

## ✅ Implementation Status — COMPLETED (2026-07-25)

### Files đã implement

#### `src/main/runtime/rpc/methods/github-auth.ts` [NEW]
- ✅ `github.startAuthLogin({ devServerId, host? })` → `relay.call('pty.spawn', { command: 'gh', args: ['auth', 'login'], cols: 120, rows: 30 })` → `{ ptyId, devServerId }`
- ✅ `github.revokeAuth({ devServerId, host? })` → `relay.call('pty.spawn', { command: 'gh', args: ['auth', 'logout'] })` → `{ ptyId, devServerId }`
- ✅ Params dùng `requiredString()` helper chuẩn codebase
- ✅ Guard check: `ctx.devServerManager` + relay connected
- ✅ Exported as `GITHUB_AUTH_METHODS` → đăng ký vào `ALL_RPC_METHODS`

#### `src/main/runtime/rpc/methods/gitlab-auth.ts` [NEW]
- ✅ `gitlab.startAuthLogin({ devServerId, host? })` → `relay.call('pty.spawn', { command: 'glab', args: ['auth', 'login'] })` → `{ ptyId, devServerId }`
- ✅ `gitlab.revokeAuth({ devServerId, host? })` → `relay.call('pty.spawn', { command: 'glab', args: ['auth', 'logout'] })` → `{ ptyId, devServerId }`
- ✅ Exported as `GITLAB_AUTH_METHODS` → đăng ký vào `ALL_RPC_METHODS`

#### `src/main/runtime/rpc/methods/index.ts` [MODIFIED]
- ✅ `GITHUB_AUTH_METHODS` và `GITLAB_AUTH_METHODS` spread vào `ALL_RPC_METHODS`

### Kết quả thực tế
```
relay.pty-handler.ts: pty.spawn endpoint đã sẵn có — không cần implement thêm trên relay
github-auth.ts (97 lines), gitlab-auth.ts (95 lines) — fully implemented
TypeScript: 0 new errors
```

### Thay đổi so với thiết kế gốc
| Thiết kế gốc | Implementation thực tế |
|---|---|
| `relay.call('pty.spawn', { command, args, cols, rows })` | Đúng như thiết kế + thêm `env: {}` field |
| Chỉ có `startAuthLogin` | Thêm `revokeAuth` (logout) cho cả github và gitlab |
| `z.string()` params | Dùng `requiredString()` helper |

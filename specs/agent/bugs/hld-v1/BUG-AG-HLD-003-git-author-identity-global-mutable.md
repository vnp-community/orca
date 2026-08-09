# BUG-AG-HLD-003 — Git author identity là global mutable config, không gắn với request context

**Mức độ:** 🟠 High
**Status:** 🔴 Open
**Module:** `agent/src/relay/preflight-handler.ts` (`setGitIdentity`)
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "Git & External API Integration")

---

## Mô tả

Thiết kế (`docs/hld/dev-server-architecture.md:192`, mục "Agent Isolation Model") quy định: *"Git author | Injected từ `ctx.userEmail`, không thể bị override"* — ngụ ý mỗi git operation tự động gắn author = user hiện tại của RPC request, cách ly giữa các user dùng chung một dev server agent instance.

Code thật không có cơ chế này. Grep toàn bộ `git-handler.ts` và `agent-git-handler.ts` cho `userEmail`/`GIT_AUTHOR`/`GIT_COMMITTER` → 0 kết quả. Cơ chế identity duy nhất là RPC `preflight.setGitIdentity`:

```ts
// preflight-handler.ts:290-293
private async setGitIdentity(params: { name: string; email: string }): Promise<void> {
  await execFileAsync('git', ['config', '--global', 'user.name', params.name])
  await execFileAsync('git', ['config', '--global', 'user.email', params.email])
}
```

Đây là ghi **global git config một lần** (không per-call, không gắn `RequestContext`) — bất kỳ user nào gọi `preflight.setGitIdentity` sau đó sẽ ghi đè identity cho **mọi** git operation tiếp theo trên cùng agent instance, bất kể user nào thực hiện commit.

## Hậu quả

- Trên một dev server agent instance dùng chung bởi nhiều user (multi-user session), commit của user A có thể bị gán nhầm author là user B nếu B gọi `setGitIdentity` sau A mà trước khi A commit.
- Đây là khoảng trống bảo mật/correctness thật, không chỉ là sai lệch tài liệu — vi phạm rõ ràng lời hứa cách ly "không thể bị override" trong Agent Isolation Model.

## Bằng chứng

```
docs/hld/dev-server-architecture.md:192 → "Git author | Injected từ ctx.userEmail, không thể bị override"
agent/src/relay/preflight-handler.ts:290-293 → setGitIdentity ghi git config --global, không theo context
```

## Đề xuất fix

Thay đổi git commit operations để truyền `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL`/`GIT_COMMITTER_NAME`/`GIT_COMMITTER_EMAIL` như env var per-call (lấy từ `RequestContext`/`ctx.userEmail` của RPC request đang xử lý), thay vì dựa vào `git config --global` mutable. `git.exec` nên từ chối override các biến này nếu caller cố truyền `--global` config command liên quan tới identity.

## Tham khảo

- Audit: `audit/agent/git-ssh-external-api-vs-design-review.md` §2.6

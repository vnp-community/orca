# BUG-BE-HLD-005 — `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user isolation không hoạt động dù 2 luồng relay hẹp có tồn tại

**Mức độ:** 🟠 HIGH (Security — per-user CLI credential isolation broken end-to-end)
**Status:** 🔴 Open
**Module:** `backend/src/main/github/github-auth.ts`, `backend/src/main/gitlab/gitlab-auth.ts`, `backend/src/main/ipc/preflight.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.9/F30, CR-GH-004)

---

## Mô tả

`docs/features/F30-remote-integrations.md` (CR-GH-004 "Session Isolation") đánh dấu **đã hoàn thành** việc mỗi user có `GH_CONFIG_DIR=~/.config/gh/<userId>/` riêng khi Dev Server Agent chạy `gh` CLI thay mặt user đó.

Cơ chế `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` **chỉ tồn tại phía Agent** (`agent/src/relay/agent-git-handler.ts`, `external-api-connector.ts`, `agent-spawner.ts`). Nhưng 2 luồng duy nhất mà Backend thực sự relay sang Agent cho GitHub/GitLab (`*.startAuthLogin` và `preflight.check`) **không bao giờ truyền `userId` hay `GH_CONFIG_DIR` nào**:

```typescript
// github-auth.ts / gitlab-auth.ts
relay.call('pty.spawn', { command: 'gh'/'glab', args, env: {}, ... })
//                                                      ^^^^^^ rỗng — không userId, không GH_CONFIG_DIR
```

`preflight.check` cũng chỉ gửi `{ traceId: span.id }`, không có `userId`.

## Hậu quả

- Dù Agent-side đã sẵn sàng cách ly theo user, **Backend không bao giờ kích hoạt nó** — tính năng cô lập per-user cho CLI auth trên Dev Server (acceptance criterion đánh dấu `[x]` trong F30) **không hoạt động end-to-end**.
- Nếu nhiều user cùng đăng nhập `gh auth login` qua cùng Dev Server, họ có thể vô tình dùng chung 1 config `gh` mặc định (không namespace theo userId) → session/token leak giữa user.

## Bằng chứng

- `backend/src/main/github/github-auth.ts` (hàm `startAuthLogin`, dòng ~50) — `relay.call('pty.spawn', {command:'gh', args, env:{}})`.
- `backend/src/main/gitlab/gitlab-auth.ts` (hàm tương tự, dòng ~48) — cùng pattern `env:{}`.
- `backend/src/main/ipc/preflight.ts:53-57` — `preflight.check` gửi `{traceId}`, không có `userId`.
- Grep `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` toàn `backend/src`: 0 kết quả.

## Đề xuất fix

1. Sửa `github-auth.ts`/`gitlab-auth.ts` để truyền `userId` (từ `ctx.userId`) trong `env`/params của `relay.call('pty.spawn', ...)`, để Agent tự set `GH_CONFIG_DIR=~/.config/gh/<userId>/` trước khi spawn `gh`.
2. Truyền `userId` tương tự cho `preflight.check` nếu Agent-side cần nó để check auth status đúng theo user.
3. Viết integration test: 2 user khác nhau `gh auth login` qua cùng Dev Server phải có 2 config file riêng biệt, không ảnh hưởng lẫn nhau.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.9 (F30, CR-GH-004)
- Doc gốc: `docs/features/F30-remote-integrations.md`
- Liên quan: [BUG-BE-HLD-004](./BUG-BE-HLD-004-github-gitlab-cli-runs-on-backend-not-relayed.md)

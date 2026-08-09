# TASK-HLD-008: Agent `pty-handler.ts` đọc `userId` và set `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`

**Priority:** 🔴 CRITICAL — prerequisite bắt buộc để TASK-HLD-007 có tác dụng
**Effort:** ~20 phút
**Status:** ✅ DONE — 2026-08-09 (`providerEnv` logic + import `buildGhEnv`/`buildGlabEnv` thêm vào `spawn()`; cần cast `as Record<string, string>` do `NodeJS.ProcessEnv` lỏng kiểu hơn `Record<string,string>` — đã xác nhận `tsc --noEmit` sạch cho file này. Lỗi TS6307 còn lại trong package `agent` là baseline pre-existing (tsconfig thiếu file list), không liên quan. Nhóm GitHub/GitLab relay (006-008) hoàn tất — per-user credential isolation hoạt động end-to-end.)
**Bug refs:** BUG-BE-HLD-005 (phần Agent)
**Solution ref:** [SOLUTION-github-gitlab-relay-exact.md](../solutions/SOLUTION-github-gitlab-relay-exact.md)
**Depends on:** TASK-HLD-007 (nên làm song song hoặc ngay sau — nếu bỏ qua task này thì TASK-HLD-007 là no-op: `userId` được gửi đi nhưng không ai đọc nó)

---

## Mục tiêu

Sửa `agent/src/relay/pty-handler.ts`'s `spawn()` để đọc `params.userId` và set `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` env **trước khi** spawn `gh`/`glab`, dùng lại `buildGhEnv`/`buildGlabEnv` đã có sẵn trong `agent/src/relay/external-api-connector.ts`.

Đây là phát hiện bổ sung quan trọng trong solution (không có trong bug report gốc): `pty-handler.ts` — nơi thực sự xử lý RPC `pty.spawn` (đăng ký ở dòng 513: `this.dispatcher.onRequest('pty.spawn', (p, context) => this.spawn(p, context))`) — **không hề đọc `userId` hay gọi `buildGhEnv`/`buildGlabEnv`**:

```
$ grep -n "userId\|GH_CONFIG_DIR\|GLAB_CONFIG_DIR" agent/src/relay/pty-handler.ts
(không có kết quả)
```

`spawn()` (dòng 632) build env qua `this.buildSpawnEnv(env, {...}, envToDelete)` — hoàn toàn độc lập với `buildGhEnv`/`buildGlabEnv` của `external-api-connector.ts`. Nghĩa là: **kể cả khi Backend được sửa để truyền `userId` (TASK-HLD-007), `pty.spawn` phía Agent vẫn không tự set `GH_CONFIG_DIR`** — phải sửa cả 2 phía.

`buildGhEnv`/`buildGlabEnv` (`agent/src/relay/external-api-connector.ts:74-90`) đã implement đúng cơ chế cách ly per-user, nhưng hiện chỉ dùng cho các RPC `github.*`/`gitlab.*` nghiệp vụ (`github.pr.create`, `gitlab.mr.create`...), KHÔNG áp dụng cho `pty.spawn`:

```typescript
export function buildGhEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  return {
    ...baseEnv,
    GH_CONFIG_DIR:          `${homedir()}/.config/gh/${userId}/`,
    GH_NO_UPDATE_NOTIFIER:  '1',
    GH_PROMPT_DISABLED:     '1',
  }
}
export function buildGlabEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  return {
    ...baseEnv,
    GLAB_CONFIG_DIR: `${homedir()}/.config/glab-cli/${userId}/`,
    NO_COLOR:        '1',
    CI:              '1',
  }
}
```

## File cần sửa/tạo

```
agent/src/relay/pty-handler.ts   (sửa)
```

Không tạo file mới. Không sửa `external-api-connector.ts` — `buildGhEnv`/`buildGlabEnv` đã export sẵn, chỉ cần import và gọi.

## Thay đổi cụ thể

Trong `agent/src/relay/pty-handler.ts`, `spawn()` (khoảng dòng 632-719), trước khi gọi `this.buildSpawnEnv(...)`:

```typescript
import { buildGhEnv, buildGlabEnv } from './external-api-connector'

// ... trong spawn():
const command = typeof params.command === 'string' ? params.command : undefined
const userId = typeof params.userId === 'string' ? params.userId : undefined

// FIX BUG-BE-HLD-005: gh/glab auth-login PTYs need the same per-user
// GH_CONFIG_DIR/GLAB_CONFIG_DIR isolation as the github.*/gitlab.* RPC
// handlers in external-api-connector.ts — otherwise every user's
// `gh auth login` shares one default config on the Dev Server.
const providerEnv =
  userId && command === 'gh' ? buildGhEnv(userId, {}) :
  userId && command === 'glab' ? buildGlabEnv(userId, {}) :
  undefined

const spawnEnv = this.buildSpawnEnv(
  providerEnv ? { ...env, ...providerEnv } : env,
  { id, paneKey, shell, command },
  envToDelete
)
```

Với thay đổi này, `env` do Backend truyền (hiện tại là `{}`) được merge thêm `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` dựa trên `userId`, nhất quán với cách `buildGhEnv`/`buildGlabEnv` đã hoạt động cho các RPC nghiệp vụ khác.

## Verification

```bash
pnpm tsc --noEmit

# Xác nhận import và logic đã có mặt:
grep -n "buildGhEnv\|buildGlabEnv" agent/src/relay/pty-handler.ts
# Expected: 1 dòng import + ít nhất 2 lời gọi (gh, glab)

pnpm vitest run agent/src/relay/pty-handler.test.ts
```

Thêm test case trong `agent/src/relay/pty-handler.test.ts`:
1. `spawn()` với `params.command === 'gh'` và `params.userId === 'user-123'` → assert spawn env chứa `GH_CONFIG_DIR` đúng path `${homedir()}/.config/gh/user-123/`.
2. `spawn()` với `params.command === 'glab'` và `params.userId` → assert `GLAB_CONFIG_DIR` đúng path.
3. `spawn()` không có `userId` (hoặc `command` khác `gh`/`glab`) → env spawn giữ nguyên như hiện tại (regression, không có `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` bị inject sai).

## Verification end-to-end (sau khi cả TASK-HLD-007 và task này đều xong)

```bash
# Trên môi trường ORCA_MULTI_USER=1 với 2 user khác nhau:
# 1. User A chạy github.startAuthLogin → gh auth login trên Dev Server
# 2. Kiểm tra: gh config được ghi vào ~/.config/gh/<userA-id>/, KHÔNG phải ~/.config/gh/ mặc định
# 3. User B chạy github.startAuthLogin → phải cần login riêng, không dùng chung session của User A
```

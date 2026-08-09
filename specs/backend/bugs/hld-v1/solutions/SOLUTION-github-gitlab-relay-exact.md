# SOLUTION: BUG-BE-HLD-004 & BUG-BE-HLD-005 — GitHub/GitLab CLI chạy local trên Backend, `GH_CONFIG_DIR` không relay

**Source-verified:** ✅ Dựa trên source code thực tế đọc trực tiếp trong worktree `/opt/repos/orca`
**Files nguồn đã đọc:**
`backend/src/main/runtime/rpc/methods/github-auth.ts`, `backend/src/main/runtime/rpc/methods/gitlab-auth.ts`,
`backend/src/main/runtime/rpc/methods/preflight.ts`, `backend/src/main/runtime/rpc/core.ts`,
`backend/src/main/git/runner.ts`, `backend/src/main/github/gh-utils.ts`, `backend/src/main/github/github-repository-identity.ts`,
`agent/src/relay/agent-rpc-dispatch.ts`, `agent/src/relay/external-api-connector.ts`, `agent/src/relay/pty-handler.ts`,
`backend/src/main/credentials/index.ts`

> Lưu ý: 2 file `backend/src/main/github/github-auth.ts` và `backend/src/main/gitlab/gitlab-auth.ts` mà 2 báo cáo bug gốc trích dẫn **không tồn tại** ở path đó trong worktree hiện tại. Logic tương ứng (`*.startAuthLogin`, `*.revokeAuth` gọi `relay.call('pty.spawn', ...)`) nằm ở:
> - `backend/src/main/runtime/rpc/methods/github-auth.ts`
> - `backend/src/main/runtime/rpc/methods/gitlab-auth.ts`
>
> Nội dung/pattern lỗi giống hệt mô tả trong 2 bug report — chỉ khác đường dẫn file thực tế. Solution này dùng path thật.

---

## 1. Xác nhận lại bằng chứng bug

### 1a. BUG-BE-HLD-004 — local exec thay vì relay

`backend/src/main/github/github-repository-identity.ts:39-50` — khi repo có `connectionId` (SSH), trả `{}` (không set `cwd`), nghĩa là `gh` vẫn chạy **trong process Backend**, không có bước relay:

```typescript
export function ghRepoExecOptions(context: GitHubRepoContext): {
  cwd?: string
  encoding?: BufferEncoding
  wslDistro?: string
} {
  return context.connectionId
    ? {}
    : {
        cwd: context.repoPath,
        ...(context.wslDistro ? { wslDistro: context.wslDistro } : {})
      }
}
```

`backend/src/main/github/gh-utils.ts:1-8` xác nhận đây là entry point dùng chung — mọi caller GitHub nghiệp vụ (`issues.ts`, `rate-limit.ts`, `work-item-details.ts`, `project-view/*`, `client.ts`...) import `ghExecFileAsync`/`gitExecFileAsync` từ đây, re-export nguyên vẹn từ `../git/runner`:

```typescript
import { gitExecFileAsync, ghExecFileAsync, extractExecError } from '../git/runner'
export const execFileAsync = promisify(execFile)
export { ghExecFileAsync, gitExecFileAsync, extractExecError }
```

`backend/src/main/git/runner.ts:1405` (`ghExecFileAsync`) và `:1548` (`glabExecFileAsync`) đều gọi `execFileCapture(resolved.binary, ...)` **trực tiếp trong process Backend**, không có bất kỳ nhánh nào kiểm tra `ORCA_MULTI_USER` hay relay sang Dev Server. Grep xác nhận **0 guard** hiện có:

```
$ grep -n "ORCA_MULTI_USER" backend/src/main/git/runner.ts
(không có kết quả)
```

### 1b. BUG-BE-HLD-005 — `userId`/`GH_CONFIG_DIR` không truyền qua relay

`backend/src/main/runtime/rpc/methods/github-auth.ts:45-56` (`github.startAuthLogin`):

```typescript
const args = ['auth', 'login']
if (params.host) {
  args.push('--hostname', params.host)
}

const ptyId = await relay.call<string>('pty.spawn', {
  command: 'gh',
  args,
  env: {},          // ← rỗng — không userId, không GH_CONFIG_DIR
  cols: 120,
  rows: 30
})
```

`backend/src/main/runtime/rpc/methods/gitlab-auth.ts:43-54` (`gitlab.startAuthLogin`) — pattern giống hệt với `command: 'glab'`.

`backend/src/main/runtime/rpc/methods/preflight.ts:53-57` (`preflight.check`) — cũng không truyền `userId`:

```typescript
const result = await relay.call<Record<string, unknown>>(
  'preflight.check',
  { traceId: span.id },
  30_000
)
```

`RpcContext` (`backend/src/main/runtime/rpc/core.ts:84-87`) **đã có sẵn `userId`** — bug chỉ là các handler trên không đọc nó:

```typescript
// Why: credential-store reads are scoped per authenticated Orca user.
// Each user-process in ORCA_MULTI_USER=1 mode has a distinct userId injected
// via ORCA_USER_ID env var and forwarded here from the session router.
userId?: string
```

Phía Agent (`agent/src/relay/external-api-connector.ts:74-90`) đã implement đúng cơ chế cách ly per-user — nhưng **chỉ dùng cho các RPC `github.*`/`gitlab.*` nghiệp vụ** (`github.pr.create`, `gitlab.mr.create`...), KHÔNG áp dụng cho `pty.spawn`:

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

**Phát hiện bổ sung quan trọng** (không có trong 2 bug report gốc): `agent/src/relay/pty-handler.ts` — nơi thực sự xử lý RPC `pty.spawn` (đăng ký ở dòng 513: `this.dispatcher.onRequest('pty.spawn', (p, context) => this.spawn(p, context))`) — **không hề đọc `userId` hay gọi `buildGhEnv`/`buildGlabEnv`**:

```
$ grep -n "userId\|GH_CONFIG_DIR\|GLAB_CONFIG_DIR" agent/src/relay/pty-handler.ts
(không có kết quả)
```

`spawn()` (dòng 632) build env qua `this.buildSpawnEnv(env, {...}, envToDelete)` — hoàn toàn độc lập với `buildGhEnv`/`buildGlabEnv` của `external-api-connector.ts`. Nghĩa là: **kể cả khi Backend được sửa để truyền `userId`, `pty.spawn` phía Agent vẫn không tự set `GH_CONFIG_DIR`** — phải sửa cả 2 phía. Đã cập nhật fix ở mục 3 bên dưới để phản ánh đúng thực tế này.

---

## 2. Fix tối thiểu (ưu tiên, làm trước) — Guard chặn local exec khi multi-user

**File:** `backend/src/main/git/runner.ts`

Điểm chung nhất để đặt guard là ngay trong `ghExecFileAsync` và `glabExecFileAsync` — đây là 2 hàm mà **mọi** caller GitHub/GitLab cục bộ đi qua (`gh-utils.ts` chỉ re-export, không tự thực thi). Đặt guard ở đây đảm bảo không cần sửa từng call site rải rác (`issues.ts`, `rate-limit.ts`, `work-item-details.ts`, `client.ts`, `merge-request-creation.ts`...).

```typescript
// backend/src/main/git/runner.ts

// Why: in Web Server multi-user mode, gh/glab CLI must run on the Dev Server
// (per-user GH_CONFIG_DIR isolation, see external-api-connector.ts on the
// Agent side) — never on the shared Orca Server host. Executing here would
// silently share one gh/glab auth context across every user. See BUG-BE-HLD-004.
function assertLocalGhCliAllowed(cli: 'gh' | 'glab'): void {
  if (process.env['ORCA_MULTI_USER'] === '1') {
    throw new Error(
      `MULTI_USER_GH_CLI_NOT_SUPPORTED: '${cli}' cannot run locally on the Orca ` +
      `Server host while ORCA_MULTI_USER=1. ${cli === 'gh' ? 'GitHub' : 'GitLab'} ` +
      'operations must be relayed to the Dev Server Agent for per-user credential ' +
      'isolation. This call site needs to be migrated to relay.call(...) — see ' +
      'specs/backend/bugs/hld-v1/BUG-BE-HLD-004-github-gitlab-cli-runs-on-backend-not-relayed.md.'
    )
  }
}

export async function ghExecFileAsync(
  args: string[],
  options: GhExecOptions = {}
): Promise<{ stdout: string; stderr: string }> {
  assertLocalGhCliAllowed('gh')
  // Why: while a bucket's primary rate limit is exhausted, every spawn would
  // return the same 403 — fail fast without paying the subprocess cost. ...
  const rateLimitBucket = classifyGhRateLimitBucket(args)
  // ... (phần còn lại giữ nguyên)
```

```typescript
export async function glabExecFileAsync(
  args: string[],
  options: GlabExecOptions = {}
): Promise<{ stdout: string; stderr: string }> {
  assertLocalGhCliAllowed('glab')
  ;({ args, options } = redirectPortedHostnameToEnv(args, options))
  // ... (phần còn lại giữ nguyên)
```

Nếu muốn thêm 1 lớp bảo vệ nữa gần renderer/API hơn (không bắt buộc, nhưng rẻ), có thể thêm cùng guard vào `ghRepoExecOptions()` (`backend/src/main/github/github-repository-identity.ts`) để fail sớm hơn, trước cả khi build exec options:

```typescript
export function ghRepoExecOptions(context: GitHubRepoContext): {
  cwd?: string
  encoding?: BufferEncoding
  wslDistro?: string
} {
  if (context.connectionId && process.env['ORCA_MULTI_USER'] === '1') {
    // Why: an SSH-connectionId repo with no cwd means gh would run on the
    // shared Orca Server host with no per-user isolation. See BUG-BE-HLD-004.
    throw new Error(
      'MULTI_USER_GH_CLI_NOT_SUPPORTED: gh CLI for SSH-connected repos must be ' +
      'relayed to the Dev Server Agent in Web Server multi-user mode.'
    )
  }
  return context.connectionId
    ? {}
    : {
        cwd: context.repoPath,
        ...(context.wslDistro ? { wslDistro: context.wslDistro } : {})
      }
}
```

Guard trong `runner.ts` là **bắt buộc và đủ** (single choke point); guard trong `ghRepoExecOptions()` chỉ là phòng vệ thêm cho lỗi hiển thị sớm hơn ở use case SSH cụ thể nêu trong bug report.

**Rủi ro cần lưu ý:** guard này sẽ làm mọi thao tác GitHub/GitLab nghiệp vụ (list PR/issues, rate-limit, comments...) **throw lỗi ngay** khi chạy dưới `ORCA_MULTI_USER=1`, cho đến khi mục 4 (di chuyển sang `relay.call`) được làm. Đây là đánh đổi có chủ đích: thà fail rõ ràng còn hơn âm thầm rò rỉ credential giữa user — nhưng cần release đồng thời với ít nhất phần roadmap ở mục 4 cho các luồng nghiệp vụ chính (PR/issue/MR list, tạo PR/MR), nếu không multi-user mode sẽ mất hẳn tính năng GitHub/GitLab.

---

## 3. Fix `github-auth.ts`/`gitlab-auth.ts` — truyền `userId` qua relay

### 3a. Backend — truyền `ctx.userId`

**File:** `backend/src/main/runtime/rpc/methods/github-auth.ts`

```typescript
defineMethod({
  name: 'github.startAuthLogin',
  params: StartAuthLoginParams,
  handler: async (params, ctx) => {
    if (!ctx.devServerManager) {
      throw new Error(
        'github.startAuthLogin requires Web Server mode (devServerManager not available)'
      )
    }
    const relay = ctx.devServerManager.getRelay(params.devServerId)
    if (!relay) {
      throw new Error(
        `Dev server '${params.devServerId}' relay is not connected. ` +
        `Connect to the dev server first.`
      )
    }

    const args = ['auth', 'login']
    if (params.host) {
      args.push('--hostname', params.host)
    }

    // FIX BUG-BE-HLD-005: forward the authenticated user so the Agent can
    // namespace GH_CONFIG_DIR per user (see external-api-connector.ts buildGhEnv).
    const ptyId = await relay.call<string>('pty.spawn', {
      command: 'gh',
      args,
      env: {},
      userId: ctx.userId,
      cols: 120,
      rows: 30
    })

    return { ptyId, devServerId: params.devServerId }
  }
}),
```

Áp dụng tương tự cho `github.revokeAuth` (cùng file) và cả `gitlab.startAuthLogin`/`gitlab.revokeAuth` trong `backend/src/main/runtime/rpc/methods/gitlab-auth.ts` (chỉ đổi `command: 'gh'` → `command: 'glab'`).

`preflight.ts` cũng cần patch tương tự nếu Agent-side `preflight.check` cần biết user hiện tại:

```typescript
// backend/src/main/runtime/rpc/methods/preflight.ts
const result = await relay.call<Record<string, unknown>>(
  'preflight.check',
  { traceId: span.id, userId: ctx.userId },
  30_000
)
```

### 3b. Agent — `pty.spawn` phải thực sự dùng `userId` (bổ sung bắt buộc, không có trong bug report gốc nhưng cần thiết để fix có tác dụng)

Đã xác nhận ở mục 1b: `agent/src/relay/pty-handler.ts`'s `spawn()` **không đọc `params.userId`**. Nếu chỉ sửa Backend như 3a mà không sửa Agent, `userId` sẽ bị bỏ qua và `GH_CONFIG_DIR` vẫn không được set — fix sẽ là no-op.

**File:** `agent/src/relay/pty-handler.ts`, trong `spawn()` (khoảng dòng 632-719), trước khi gọi `this.buildSpawnEnv(...)`:

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

---

## 4. Fix đầy đủ dài hạn (roadmap) — chuyển các RPC handler sang relay

Danh sách RPC method Backend cần đổi từ gọi cục bộ (`OrcaRuntimeService` → `ghExecFileAsync`/`glabExecFileAsync`) sang `relay.call('git.*'|'github.*'|'gitlab.*', ...)`:

### 4a. Case đã có sẵn ở Agent (`agent/src/relay/agent-rpc-dispatch.ts`) — chỉ cần Backend đổi caller

| RPC method | Agent handler | Dòng dispatch |
|---|---|---|
| `git.pr.create` | `agent-git-handler.ts::handleGitPrCreate` | :411 |
| `git.worktree.list/add/remove` | `agent-git-handler.ts` | :422-451 |
| `github.pr.create` | `external-api-connector.ts::handleGitHubPrCreate` | :488 |
| `github.pr.merge` | `external-api-connector.ts::handleGitHubPrMerge` | :499 |
| `github.issue.list` | `external-api-connector.ts::handleGitHubIssueList` | :510 |
| `github.issue.create` | `external-api-connector.ts::handleGitHubIssueCreate` | :521 |
| `gitlab.mr.create` | `external-api-connector.ts::handleGitLabMrCreate` | :532 |
| `gitlab.pipeline.status` | `external-api-connector.ts::handleGitLabPipelineStatus` | :543 |

Backend cần sửa các handler tương ứng trong `backend/src/main/runtime/rpc/methods/github.ts`/`gitlab.ts` (nơi hiện gọi `OrcaRuntimeService.listRepoIssues()` → `listIssues()` → `ghExecFileAsync()`) để thay bằng `ctx.devServerManager.getRelay(devServerId).call('github.issue.list', { userId: ctx.userId, cwd, ... })`.

### 4b. Handler đã viết ở Agent nhưng **chưa được đăng ký** trong `agent-rpc-dispatch.ts` — cần bổ sung case trước khi Backend có thể gọi

Grep xác nhận các hàm này **tồn tại** trong `external-api-connector.ts` nhưng **không có case tương ứng** trong `agent-rpc-dispatch.ts`:

- `handleGitHubAuthStatus` (dòng 311) → cần thêm `case 'github.auth.status':`
- `handleGitLabAuthStatus` (dòng 460) → cần thêm `case 'gitlab.auth.status':`
- `handleGitLabMrList` (dòng 394) → cần thêm `case 'gitlab.mr.list':`

### 4c. Nghiệp vụ Backend hiện có mà Agent **chưa có handler nào** — cần viết mới cả 2 phía (Agent handler + dispatch case + Backend caller)

Dựa trên bằng chứng BUG-BE-HLD-004 (`backend/src/main/github/rate-limit.ts`, `work-item-details.ts`, `project-view/*`, `pr-review-comment-lines.ts`, `merged-pr-commit-membership.ts`, `comment-reactions.ts`; `backend/src/main/gitlab/issues.ts`, `merge-request-creation.ts`, `gitlab-project-recents.ts`) chạy hoàn toàn cục bộ và không có method Agent song song nào:

- `github.rateLimit.check`
- `github.workItem.details`
- `github.projectView.*`
- `github.pr.comments` / `github.pr.reviewComments`
- `gitlab.issue.list` / `gitlab.issue.create`
- `gitlab.project.recents` / `gitlab.project.refResolve`

Việc này cần được lập roadmap/CR riêng — không viết đủ code trong solution này, chỉ liệt kê phạm vi việc cần làm dựa trên grep thực tế.

---

## 5. Khuyến nghị thứ tự triển khai

1. **Làm ngay (mục 2 + 3):** rẻ, không cần thiết kế lại kiến trúc, giảm rủi ro rò rỉ credential giữa user ngay lập tức trong `ORCA_MULTI_USER=1`.
   - Guard `assertLocalGhCliAllowed()` trong `runner.ts` (mục 2).
   - Truyền `userId` qua `pty.spawn` ở cả Backend (`github-auth.ts`/`gitlab-auth.ts`/`preflight.ts`) và Agent (`pty-handler.ts` đọc `userId` để build `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`) (mục 3).
   - Lưu ý: bật guard mục 2 mà chưa làm mục 4 sẽ làm mất chức năng GitHub/GitLab nghiệp vụ trong multi-user mode — cân nhắc rollout theo flag hoặc làm đồng thời tối thiểu các method PR/issue/MR list phổ biến nhất trong mục 4a.
2. **Làm theo roadmap riêng (mục 4):** di chuyển toàn bộ `backend/src/main/github/*`/`gitlab/*` sang `relay.call`, bổ sung 3 case còn thiếu ở Agent (4b), và viết mới các method chưa có song song ở Agent (4c). Nên tách thành nhiều PR nhỏ theo từng nhóm method để dễ review và test riêng biệt (list vs. create vs. auth-status).

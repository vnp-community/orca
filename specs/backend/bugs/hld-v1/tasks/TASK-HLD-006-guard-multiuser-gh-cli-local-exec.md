# TASK-HLD-006: Guard chặn `gh`/`glab` CLI chạy local trên Backend khi multi-user

**Priority:** 🔴 CRITICAL — rò rỉ credential giữa user trong `ORCA_MULTI_USER=1`
**Effort:** ~15 phút
**Status:** ✅ DONE — 2026-08-09 (guard `assertLocalGhCliAllowed()` thêm vào `ghExecFileAsync`/`glabExecFileAsync`; đồng thời làm luôn phần optional trong `ghRepoExecOptions()`. `tsc --noEmit` sạch cho cả 2 file. ⚠️ Rủi ro đã biết: mọi thao tác gh/glab sẽ throw khi `ORCA_MULTI_USER=1` cho tới khi TASK-HLD-009 (roadmap relay) triển khai — đây là đánh đổi có chủ đích theo đúng solution.)
**Bug refs:** BUG-BE-HLD-004
**Solution ref:** [SOLUTION-github-gitlab-relay-exact.md](../solutions/SOLUTION-github-gitlab-relay-exact.md)
**Depends on:** không có (làm trước, độc lập)

---

## Mục tiêu

Thêm hàm guard `assertLocalGhCliAllowed()` vào `ghExecFileAsync`/`glabExecFileAsync` trong `backend/src/main/git/runner.ts` để **fail rõ ràng** thay vì âm thầm chạy `gh`/`glab` cục bộ trên process Backend khi `ORCA_MULTI_USER === '1'`.

Hiện tại `ghExecFileAsync` (dòng ~1405) và `glabExecFileAsync` (dòng ~1548) trong `runner.ts` gọi `execFileCapture(resolved.binary, ...)` trực tiếp trong process Backend, **không có bất kỳ nhánh nào** kiểm tra `ORCA_MULTI_USER` hay relay sang Dev Server. Grep xác nhận 0 guard hiện có:

```
$ grep -n "ORCA_MULTI_USER" backend/src/main/git/runner.ts
(không có kết quả)
```

Đây là bug bảo mật: trong Web Server multi-user mode, `gh`/`glab` CLI phải chạy trên Dev Server (Agent) với `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` cách ly theo user. Chạy cục bộ trên Backend nghĩa là **một credential context `gh`/`glab` duy nhất bị share giữa mọi user**.

## File cần sửa/tạo

```
backend/src/main/git/runner.ts   (sửa)
```

Không tạo file mới. Không sửa file khác — mọi caller (`gh-utils.ts`, `issues.ts`, `rate-limit.ts`, `work-item-details.ts`, `client.ts`, `merge-request-creation.ts`...) đi qua `ghExecFileAsync`/`glabExecFileAsync` nên đặt guard tại đây là đủ (single choke point), không cần sửa từng call site.

## Thay đổi cụ thể

Thêm hàm guard (đặt trước `ghExecFileAsync`, dùng chung cho cả `gh` và `glab`):

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
```

Gọi guard ở đầu 2 hàm hiện có, giữ nguyên phần thân còn lại:

```typescript
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

### Tùy chọn (không bắt buộc, rẻ) — lớp bảo vệ thêm gần renderer/API hơn

Có thể thêm cùng guard vào `ghRepoExecOptions()` trong `backend/src/main/github/github-repository-identity.ts` để fail sớm hơn, trước cả khi build exec options, cho use case SSH cụ thể trong bug report:

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

Guard trong `runner.ts` là **bắt buộc và đủ**; guard trong `ghRepoExecOptions()` chỉ là phòng vệ thêm, có thể bỏ qua nếu muốn scope nhỏ nhất.

## Rủi ro cần lưu ý

Guard này sẽ làm **mọi thao tác GitHub/GitLab nghiệp vụ** (list PR/issues, rate-limit, comments...) throw lỗi ngay khi chạy dưới `ORCA_MULTI_USER=1`, cho đến khi TASK-HLD-009 (roadmap di chuyển sang `relay.call`) được làm. Đây là đánh đổi có chủ đích: thà fail rõ ràng còn hơn âm thầm rò rỉ credential giữa user. Cân nhắc rollout đồng thời với ít nhất các method PR/issue/MR list phổ biến nhất (xem TASK-HLD-009 mục 4a) để không mất hẳn tính năng GitHub/GitLab trong multi-user mode.

## Verification

```bash
pnpm tsc --noEmit

# Guard tồn tại và được gọi đúng chỗ:
grep -n "assertLocalGhCliAllowed" backend/src/main/git/runner.ts
# Expected: 1 định nghĩa hàm + 2 (hoặc 3 nếu làm cả optional) lời gọi

# Test guard throw đúng khi multi-user:
ORCA_MULTI_USER=1 pnpm vitest run backend/src/main/git/runner.test.ts -t "assertLocalGhCliAllowed"
# Expected: ghExecFileAsync/glabExecFileAsync throw Error chứa "MULTI_USER_GH_CLI_NOT_SUPPORTED"

# Test không có guard khi không multi-user (regression):
unset ORCA_MULTI_USER
pnpm vitest run backend/src/main/git/runner.test.ts
# Expected: các test hiện có (gh/glab exec bình thường) vẫn pass, không throw
```

Viết thêm test case mới trong `backend/src/main/git/runner.test.ts` (hoặc file test tương ứng) để cover: (1) `ghExecFileAsync` throw khi `ORCA_MULTI_USER=1`, (2) `glabExecFileAsync` throw khi `ORCA_MULTI_USER=1`, (3) cả hai hàm chạy bình thường khi không set biến này hoặc set giá trị khác `'1'`.

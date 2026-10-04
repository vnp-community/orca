# BUG-AG-WT-001 — `handleGitExec` không bao giờ trả JSON-RPC `error` khi `git` exit code khác 0; comment của `RelayExecutor.branchExistsLocally` claim sai về whitelist

**Mức độ:** 🟡 Medium
**Status:** 🔴 Open
**Module:** `agent/src/relay/agent-git-handler.ts` (`handleGitExec`); `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (`branchExistsLocally`'s doc comment)
**Phát hiện:** 2026-09-15, trong lúc điều tra `WORKTREE_CREATE_FAILED`/`INFRA_AGENT_EXEC_FAILED` sau khi đã fix bug ownership (`.git` sai chủ) và bug filesystem permission (`/opt` không cho `ubuntu` ghi) cho repo "aiops"/"aiops-v3" trên `test-01`.

---

## Bug #1: `handleGitExec` luôn trả `result` (thành công) dù `git` exit code ≠ 0

**File:** `agent/src/relay/agent-git-handler.ts:201-216`

```ts
child.on('close', (code) => {
  clearTimeout(timer)
  const exitCode = code ?? 0
  log.info(`git.exec: ${rawArgs.join(' ')} → exitCode=${exitCode}`)
  const outLen = stdout.join('').length
  if (exitCode === 0) {
    span.ok({ cmd: argsStr, exitCode, outLen })
  } else {
    span.fail(`git exit ${exitCode}`, { cmd: argsStr, exitCode, outLen })   // chỉ fail cho TRACE span
  }
  resolve({
    jsonrpc: '2.0',
    id,
    result: { stdout: stdout.join(''), stderr: stderr.join(''), exitCode }   // ← LUÔN result, kể cả exitCode≠0
  })
})
```

Dù `exitCode` khác 0 (git thất bại thật — permission denied, ref conflict, v.v.), hàm vẫn `resolve` với `result:` (JSON-RPC thành công), không bao giờ `error:`. Chỉ có `child.on('error', ...)` (spawn-level, ví dụ ENOENT thật) mới tạo ra JSON-RPC `error`.

### Hậu quả

`backend-go`'s `RelayExecutor.CreateWorktree` (`relay_executor.go:354`):
```go
if err := r.relay(ctx, repoPath, "git.worktree.add", params, nil); err != nil {
    return domain.WorktreeCreateResult{}, err
}
```
truyền `nil` làm result pointer (comment: "no structured result to unmarshal") — nghĩa là **không đọc `exitCode`/`stderr`** từ response, chỉ quan tâm có JSON-RPC `error` hay không. Vì bug #1 khiến git thất bại thật (exit ≠ 0) vẫn trả `result`, bước này coi như THÀNH CÔNG và tiếp tục bước kế (`git rev-parse HEAD` tại `targetPath` — path **chưa từng được tạo** vì bước trước fail) → bước này mới thật sự lỗi, và lỗi hiển thị (`"spawn git ENOENT"`, do Node.js báo nhầm khi `cwd` không tồn tại) **không liên quan gì đến nguyên nhân thật** (permission denied ở bước trước).

Đây chính là lý do chuỗi lỗi `INFRA_AGENT_EXEC_FAILED`/`WORKTREE_CREATE_FAILED` trong phiên làm việc này khó chẩn đoán — log cause (`apperrors` — SOL-009) chỉ thấy được `"spawn git ENOENT"`, không thấy được nguyên nhân gốc thật (`Permission denied` ở lệnh `git worktree add`).

### Fix direction (chưa implement)

`handleGitExec` nên trả JSON-RPC `error` khi `exitCode !== 0` (ít nhất cho các subcommand có tính "must succeed" như `worktree add`) — hoặc caller (`RelayExecutor.CreateWorktree`) phải tự đọc `result.exitCode`/`result.stderr` thay vì bỏ qua bằng `nil`. Cần cân nhắc: nhiều subcommand khác (`status`, `diff`) coi exit code khác 0 là dữ liệu hợp lệ (ví dụ `git diff --exit-code`), nên fix không nên áp dụng chung cho mọi subcommand — cần thiết kế riêng, không làm vội trong bug report này.

---

## Bug #2 (nhỏ, cosmetic): comment sai về `show-ref` whitelist

**File:** `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go:379`

```go
// show-ref is confirmed whitelisted on the relay's git.exec
// surface — see ListLocalBranches' doc comment above.
```

**Sai** — `agent/src/relay/agent-git-handler.ts`'s `ALLOWED_GIT_SUBCOMMANDS` (dòng 41-64) hiện tại **không có** `show-ref`:
```
status, diff, add, restore, commit, push, pull, fetch, branch, checkout,
merge, rebase, stash, log, worktree, remote, tag, show, rev-parse, config,
describe, shortlog
```

Xác nhận live trên `b15.openledger.vn` (log `apperrors: internal cause`):
```
"cause":"git subcommand not allowed: \"show-ref\". Allowed: add, branch, checkout, commit, config, describe, diff, fetch, log, merge, pull, push, rebase, remote, restore, rev-parse, shortlog, show, stash, status, tag, worktree"
```

May mắn là `branchExistsLocally` (hàm dùng `show-ref`) đã tự thiết kế fail-safe: *"Any error (missing ref, or an unexpected relay/git failure) is treated as 'does not exist'"* — nên bug này **vô hại về hành vi** (luôn nuốt lỗi, `createBranch` default về `true` đúng như thiết kế ban đầu trước khi có check này). Chỉ cần sửa lại comment cho khớp thực tế, hoặc thêm `show-ref` vào whitelist agent nếu muốn `branchExistsLocally` thật sự hoạt động đúng như tên gọi (hiện tại nó luôn trả `false`, tức "không tồn tại", 100% các lần gọi — làm mất tác dụng thật của tính năng "check existing branch" mà nó được thêm vào để giải quyết, theo đúng nội dung comment 2026-09-12 incident nó tham chiếu).

## Liên quan

- Phát hiện trong cùng đợt điều tra với [BUG-011](../../../backend-go/bugs/missing-v2/BUG-011-detected-worktrees-merge-disk-first-drops-db-rows.md), [BUG-012](../../../backend-go/bugs/missing-v2/BUG-012-gitgateway-status-failed-opaque-relay-error.md).
- Cùng theme "silent success che giấu lỗi thật" như `missing-v2/solutions/README.md`'s "Cross-cutting design theme" đã ghi nhận nhiều lần trong session này.

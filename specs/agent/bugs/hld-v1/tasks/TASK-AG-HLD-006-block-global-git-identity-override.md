# TASK-AG-HLD-006 — Chặn `git config --global/--system user.name|user.email` Qua `git.exec` (WS-Agent Path)

**Solution:** [SOL-AG-HLD-003](../solutions/SOL-AG-HLD-003-per-client-git-identity-env.md)
**Bug:** [BUG-AG-HLD-003](../BUG-AG-HLD-003-git-author-identity-global-mutable.md)
**File:** `agent/src/relay/agent-git-handler.ts`
**Phụ thuộc:** —
**Estimated:** 30 phút
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

`validateGitArgs()` trong `agent-git-handler.ts` (đường WS-agent riêng, dùng bởi Dev Server Agent qua `agent-rpc-dispatch.ts`) cho phép subcommand `config` không có ràng buộc nào trên `--global`/`--system` — khác với `GitHandler.exec()` (relay daemon, `git-exec-validator.ts`) vốn đã chặn các flag này. Vá lỗ hổng: từ chối `git config --global/--system user.name|user.email` qua `git.exec`, buộc caller dùng `preflight.setGitIdentity`.

---

## Context

Đọc trước:
- `agent/src/relay/agent-git-handler.ts` — `ALLOWED_GIT_SUBCOMMANDS` (có `'config'`), `SHELL_METACHARACTERS`, `GitValidationError`, hàm `validateGitArgs()`

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/agent-git-handler.ts`

**TÌM** (nguyên văn, dòng 86-106):
```typescript
export function validateGitArgs(args: string[]): void {
  if (args.length === 0) {
    throw new GitValidationError('GIT_NO_SUBCOMMAND', 'git args must not be empty — provide a subcommand')
  }

  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new GitValidationError(
      'GIT_DISALLOWED_SUBCOMMAND',
      `git subcommand not allowed: "${args[0]}". Allowed: ${[...ALLOWED_GIT_SUBCOMMANDS].sort().join(', ')}`
    )
  }

  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new GitValidationError(
        'GIT_SHELL_METACHARACTER_IN_ARG',
        `Unsafe character in git argument: "${arg}"`
      )
    }
  }
}
```

**THAY BẰNG:**
```typescript
export function validateGitArgs(args: string[]): void {
  if (args.length === 0) {
    throw new GitValidationError('GIT_NO_SUBCOMMAND', 'git args must not be empty — provide a subcommand')
  }

  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new GitValidationError(
      'GIT_DISALLOWED_SUBCOMMAND',
      `git subcommand not allowed: "${args[0]}". Allowed: ${[...ALLOWED_GIT_SUBCOMMANDS].sort().join(', ')}`
    )
  }

  // Why: BUG-AG-HLD-003 — identity must come from preflight.setGitIdentity's
  // per-client registry, not a global config write that leaks to every
  // other client sharing this dev server agent.
  if (
    args[0] === 'config' &&
    (args.includes('--global') || args.includes('--system')) &&
    (args.includes('user.name') || args.includes('user.email'))
  ) {
    throw new GitValidationError(
      'GIT_SHELL_METACHARACTER_IN_ARG',
      'git config --global/--system user.name|user.email is not allowed via git.exec — use preflight.setGitIdentity'
    )
  }

  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new GitValidationError(
        'GIT_SHELL_METACHARACTER_IN_ARG',
        `Unsafe character in git argument: "${arg}"`
      )
    }
  }
}
```

> [!IMPORTANT]
> `GitValidationError`'s `code` field có union type hẹp: `'GIT_NO_SUBCOMMAND' | 'GIT_DISALLOWED_SUBCOMMAND' | 'GIT_SHELL_METACHARACTER_IN_ARG'` (định nghĩa ở constructor `GitValidationError`, dòng 76-84 cùng file). Đoạn thêm mới tái dùng mã `'GIT_SHELL_METACHARACTER_IN_ARG'` đã có sẵn trong union — không cần mở rộng type. Nếu muốn mã lỗi riêng biệt hơn (vd. `'GIT_IDENTITY_OVERRIDE_BLOCKED'`), phải sửa thêm union type của `GitValidationError.code` — không bắt buộc cho task này.

---

## Verify

```bash
cd agent
npx tsc --noEmit
npx vitest run src/relay/__tests__/agent-git-handler.test.ts
```

Test case mới cần thêm (trong `agent-git-handler.test.ts`):
- `validateGitArgs(['config', '--global', 'user.name', 'attacker'])` → ném `GitValidationError`.
- `validateGitArgs(['config', '--global', 'user.email', 'attacker@evil.com'])` → ném `GitValidationError`.
- `validateGitArgs(['config', '--system', 'user.name', 'x'])` → ném `GitValidationError`.
- `validateGitArgs(['config', '--global', 'core.editor', 'vim'])` → **không** ném lỗi (chỉ chặn `user.name`/`user.email`, các key config khác vẫn được phép).
- `validateGitArgs(['config', '--local', 'user.name', 'x'])` → **không** ném lỗi từ nhánh mới (không có `--global`/`--system`) — vẫn phải pass qua `SHELL_METACHARACTERS` check như bình thường.
- Regression: các subcommand khác (`status`, `commit`, `push`, ...) không bị ảnh hưởng.

Sau khi sửa, chạy `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận thay đổi chỉ chạm `validateGitArgs` trong `agent-git-handler.ts`.

---

## Definition of Done

- [ ] `validateGitArgs()` từ chối `args[0] === 'config'` kèm `--global`/`--system` kèm `user.name`/`user.email` bằng `GitValidationError`
- [ ] Các lệnh `git config --global` với key khác `user.name`/`user.email` (vd. `core.editor`) vẫn được phép
- [ ] Các subcommand khác (`status`, `commit`, ...) không bị ảnh hưởng
- [ ] Test mới trong `agent-git-handler.test.ts` pass
- [ ] `npx tsc --noEmit` (trong `agent/`) pass
- [ ] `npx vitest run src/relay/__tests__/agent-git-handler.test.ts` pass
- [ ] `detect_changes({scope: "compare", base_ref: "main"})` chỉ show thay đổi trong `agent-git-handler.ts`

---

## Kết Quả Thực Thi (2026-08-09)

Đã sửa `validateGitArgs()` trong `agent-git-handler.ts`: chặn `git config --global/--system user.name|user.email` qua `git.exec`, các key config khác và subcommand khác không bị ảnh hưởng.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.

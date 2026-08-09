# TASK-AG-HLD-009 — Route `handleGitLabMrCreate` Qua `glabExecFileAsync` Của `runner.ts`

**Solution:** [SOL-AG-HLD-005](../solutions/SOL-AG-HLD-005-route-gh-glab-through-runner-breaker.md)
**Bug:** [BUG-AG-HLD-005](../BUG-AG-HLD-005-gh-rate-limit-breaker-not-wired.md)
**File:** `agent/src/relay/external-api-connector.ts`
**Phụ thuộc:** —
**Estimated:** 60 phút
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

`handleGitLabMrCreate()` hiện tự dựng `execFileCaptured` (spawn trực tiếp) để gọi `glab mr create` — bypass retry + WSL-aware command resolution của `runner.ts` (`glabExecFileAsync`). Route lệnh `glab mr create` qua `glabExecFileAsync`, mirror cách làm với `gh pr create` (TASK-AG-HLD-008).

---

## Context

Đọc trước:
- `agent/src/main/git/runner.ts` — `glabExecFileAsync()`, `extractExecError()` (cả hai đã `export`)
- `agent/src/relay/external-api-connector.ts` — `interface ExecResult`, `execFileCaptured()`, `handleGitLabMrCreate()`

> [!NOTE]
> Task này độc lập với TASK-AG-HLD-008 (chỉnh `gh`) — cả hai sửa cùng file `external-api-connector.ts` nhưng ở hai hàm khác nhau (`handleGitHubPrCreate` vs `handleGitLabMrCreate`). Nếu cả hai task được áp dụng, đoạn "TÌM" của phần import ở task này giả định TASK-AG-HLD-008 **chưa** áp dụng — nếu import `ghExecFileAsync, extractExecError` đã có sẵn (do TASK-AG-HLD-008 làm trước), chỉ cần thêm `glabExecFileAsync` vào cùng dòng import hiện có thay vì tạo dòng import mới trùng `extractExecError`.

---

## Thay Đổi Cần Thực Hiện

### 1. `agent/src/relay/external-api-connector.ts` — thêm import

**TÌM** (nguyên văn, nếu TASK-AG-HLD-008 CHƯA áp dụng — dòng 1-20):
```typescript
// src/relay/external-api-connector.ts
// External API connectors for Orca Dev Agent v5.0.
//
// Design principles:
//   - CLI-based: gh (GitHub CLI) and glab (GitLab CLI) — NOT SDK
//   - Per-user isolation: GH_CONFIG_DIR / GLAB_CONFIG_DIR per userId
//   - No shell injection: spawn() with array args, shell: false
//   - Metachar validation on all user input
//   - Timeout mandatory: 30s default
//   - Idempotency: github.pr.create checks existing PR first
//   - Auth never through Gateway: tokens stay on dev server filesystem

import { spawn } from 'node:child_process'
import { homedir } from 'node:os'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const apiTracer = createTracer('agent:ext-api')
```

**THAY BẰNG:**
```typescript
// src/relay/external-api-connector.ts
// External API connectors for Orca Dev Agent v5.0.
//
// Design principles:
//   - CLI-based: gh (GitHub CLI) and glab (GitLab CLI) — NOT SDK
//   - Per-user isolation: GH_CONFIG_DIR / GLAB_CONFIG_DIR per userId
//   - No shell injection: spawn() with array args, shell: false
//   - Metachar validation on all user input
//   - Timeout mandatory: 30s default
//   - Idempotency: github.pr.create checks existing PR first
//   - Auth never through Gateway: tokens stay on dev server filesystem

import { spawn } from 'node:child_process'
import { homedir } from 'node:os'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'
import { glabExecFileAsync, extractExecError } from '../main/git/runner'

const apiTracer = createTracer('agent:ext-api')
```

> Nếu TASK-AG-HLD-008 đã áp dụng trước (import `ghExecFileAsync, extractExecError` đã tồn tại), thay vào đó chỉ thêm `glabExecFileAsync` vào import hiện có:
> ```typescript
> import { ghExecFileAsync, glabExecFileAsync, extractExecError } from '../main/git/runner'
> ```

### 2. Thêm helper adapter `execGlabCaptured`

**TÌM** (nguyên văn, cuối hàm `execFileCaptured` + section header kế tiếp, dòng 63-72 — nếu TASK-AG-HLD-008 đã thêm `execGhCaptured` trước, đặt `execGlabCaptured` ngay sau nó thay vì trước `// ─── Environment builders`):
```typescript
    child.stdin?.end()
  })
}

// ─── Environment builders ─────────────────────────────────────────────────────

export function buildGhEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
```

**THAY BẰNG:**
```typescript
    child.stdin?.end()
  })
}

// ─── glab via runner.ts (circuit breaker + retry) ──────────────────────────────
// Why: BUG-AG-HLD-005 — mirrors the gh adapter: retry + WSL-aware resolution
// for glab, translated back to this file's { stdout, stderr, exitCode } shape.
async function execGlabCaptured(
  args: string[],
  opts: { cwd: string; env: NodeJS.ProcessEnv; timeout: number; idempotent?: boolean }
): Promise<ExecResult> {
  try {
    const { stdout, stderr } = await glabExecFileAsync(args, {
      cwd: opts.cwd, env: opts.env, timeout: opts.timeout, idempotent: opts.idempotent
    })
    return { stdout, stderr, exitCode: 0 }
  } catch (err: unknown) {
    const { stdout, stderr } = extractExecError(err)
    return { stdout, stderr, exitCode: 1 }
  }
}

// ─── Environment builders ─────────────────────────────────────────────────────

export function buildGhEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
```

### 3. `handleGitLabMrCreate()` — dùng `execGlabCaptured` cho lệnh `glab mr create`

**TÌM** (nguyên văn, trong `handleGitLabMrCreate`):
```typescript
  const glabArgs = [
    'mr', 'create',
    '--title',         title,
    '--description',   description,
    '--target-branch', targetBranch,
    '--yes',           // non-interactive
  ]

  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'glab mr create failed', { method: 'gitlab.mr.create', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
    }
```

**THAY BẰNG:**
```typescript
  const glabArgs = [
    'mr', 'create',
    '--title',         title,
    '--description',   description,
    '--target-branch', targetBranch,
    '--yes',           // non-interactive
  ]

  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    // BUG-AG-HLD-005: route through runner.ts's glabExecFileAsync — retry +
    // WSL-aware resolution, mirroring the gh path. `glab mr create` is a
    // write — never auto-retried as idempotent.
    const result = await execGlabCaptured(glabArgs, { cwd, env, timeout: 30_000, idempotent: false })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'glab mr create failed', { method: 'gitlab.mr.create', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
    }
```

> [!IMPORTANT]
> KHÔNG đổi các handler `glab` khác trong file (`handleGitLabMrList`, `handleGitLabPipelineStatus`, `handleGitLabAuthStatus`) — vẫn dùng `execFileCaptured` như cũ (ngoài scope Phase 1 của solution). `interface ExecResult` đã có sẵn trong file (không export) — không cần import thêm.

---

## Verify

```bash
cd agent
npx tsc --noEmit
npx vitest run src/relay/__tests__/external-api-connector.test.ts
npx vitest run src/main/git/__tests__/runner.test.ts
```

Test case mới cần thêm (trong `external-api-connector.test.ts`):
- `handleGitLabMrCreate` với `idempotent: false` không tự retry khi `glab` trả lỗi 502/503 giả lập.
- `handleGitLabMrCreate` gọi đúng `glabExecFileAsync` (không còn `execFileCaptured`) cho lệnh `mr create` — assert qua mock của `runner.ts`.
- Regression: các handler `glab` khác (`handleGitLabMrList`, `handleGitLabPipelineStatus`, `handleGitLabAuthStatus`) không đổi hành vi.

Sau khi sửa, chạy `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận blast radius chỉ nằm trong `external-api-connector.ts`.

---

## Definition of Done

- [ ] `external-api-connector.ts` import `glabExecFileAsync`, `extractExecError` từ `../main/git/runner` (gộp vào import hiện có nếu TASK-AG-HLD-008 đã chạy trước)
- [ ] Hàm `execGlabCaptured()` mới thêm, dịch throw-on-error của `glabExecFileAsync` sang `{ stdout, stderr, exitCode }`
- [ ] `handleGitLabMrCreate()`'s lệnh `glab mr create` dùng `execGlabCaptured(glabArgs, { cwd, env, timeout: 30_000, idempotent: false })` thay vì `execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })`
- [ ] `handleGitLabMrList`/`handleGitLabPipelineStatus`/`handleGitLabAuthStatus` KHÔNG bị đổi — vẫn dùng `execFileCaptured`
- [ ] Test mới: `idempotent: false` không tự retry lỗi transient cho `glab`
- [ ] Regression: các handler `glab` khác không đổi hành vi
- [ ] `npx tsc --noEmit` (trong `agent/`) pass
- [ ] `npx vitest run src/relay/__tests__/external-api-connector.test.ts` pass
- [ ] `npx vitest run src/main/git/__tests__/runner.test.ts` pass
- [ ] `detect_changes({scope: "compare", base_ref: "main"})` chỉ show thay đổi trong `external-api-connector.ts`

---

## Kết Quả Thực Thi (2026-08-09)

Đã thêm `execGlabCaptured()` (mirror `execGhCaptured` từ TASK-008, cùng import `glabExecFileAsync` gộp chung dòng) và route lệnh `glab mr create` trong `handleGitLabMrCreate` qua adapter này. Các handler glab khác (`handleGitLabMrList`/`handleGitLabPipelineStatus`/`handleGitLabAuthStatus`) không đổi.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.

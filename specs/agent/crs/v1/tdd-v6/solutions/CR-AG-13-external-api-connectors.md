# CR-AG-13: External API Connectors — GitHub & GitLab

**CR:** CR-AG-13
**TDD:** [TDD-AG-13](../../tdd/v5/13-external-api-connectors.md)
**Ngày:** 2026-07-30
**Độ phức tạp:** Medium-High — new module, per-user env isolation, idempotency
**ADR:** ADR-012
**HLD Ref:** C3.12, §12 (dev-server-architecture.md)

---

## 1. Phân tích Code Hiện Tại

### Code đã có ✅ — có thể REUSE

| File | Reuse gì | Mức độ |
|------|---------|--------|
| `src/relay/agent-git-handler.ts` | `SHELL_METACHARACTERS` regex, `validateGitArgs()` pattern | 90% |
| `src/relay/agent-git-handler.ts` | `handleGitExec()` → dùng cho `getCurrentBranch()` helper | 80% |
| `src/relay/agent-config.ts` | `AgentConfig`, `toolPath`, `toolEnv` | 100% |
| `src/relay/agent-logger.ts` | `AgentLogger` | 100% |
| `src/relay/preflight-handler.ts` | `gh auth status`, `glab auth status` patterns | 70% |
| `src/relay/agent-rpc-dispatch.ts` | Add 6 new case routes | 90% |
| `src/shared/agent-wire-protocol.ts` | `AgentErrorCode` | 100% |

### Code chưa có ❌ — cần tạo mới

| File | Nội dung |
|------|---------|
| `src/relay/external-api-connector.ts` | GitHub + GitLab connector (all handlers) |
| `src/relay/__tests__/external-api-connector.test.ts` | Tests |

---

## 2. Solution — New File: `src/relay/external-api-connector.ts`

### 2.1 Module structure

```typescript
// src/relay/external-api-connector.ts
import { spawn } from 'node:child_process'
import { homedir } from 'node:os'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// REUSE pattern từ agent-git-handler.ts:
const SHELL_METACHARACTERS = /[&|;$`<>\\!]/

// Exports — all handlers
export {
  handleGitHubPrCreate,
  handleGitHubPrMerge,
  handleGitHubIssueList,
  handleGitHubIssueCreate,
  handleGitHubAuthStatus,
  handleGitLabMrCreate,
  handleGitLabMrList,
  handleGitLabPipelineStatus,
  handleGitLabAuthStatus,
  // Env builders (exported for testing)
  buildGhEnv,
  buildGlabEnv,
  execFileCaptured,  // exported for testing
}
```

### 2.2 Key implementation detail: `execFileCaptured`

**REUSE pattern** từ `agent-git-handler.ts` `handleGitExec()` — cùng spawn approach:

```typescript
// Rất giống handleGitExec() nhưng không cần validateGitArgs()
// vì external tools (gh, glab) không phải git subcommands
function execFileCaptured(binary, args, opts) {
  return new Promise((resolve) => {
    const child = spawn(binary, args, {
      cwd: opts.cwd,
      env: opts.env,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,   // MANDATORY
    })
    // ... timeout + stdout/stderr capture (same pattern as handleGitExec)
  })
}
```

### 2.3 Idempotency for `github.pr.create`

```typescript
// Quan trọng: kiểm tra PR đã tồn tại cho branch hiện tại
// REUSE handleGitExec pattern để get current branch:

async function getCurrentBranch(cwd, env) {
  // Tái sử dụng handleGitExec với { args: ['rev-parse', '--abbrev-ref', 'HEAD'] }
  // Hoặc execFileCaptured('git', ['rev-parse', '--abbrev-ref', 'HEAD'], ...)
}

async function checkExistingPr(cwd, branch, userId, env) {
  // gh pr list --head <branch> --json url,number,title,state --limit 1
  // Parse JSON → return first result or null
}
```

---

## 3. Extend: `src/relay/agent-rpc-dispatch.ts`

Thêm 6 routes mới sau cases hiện tại:

```typescript
// github.*
case 'github.pr.create':         → handleGitHubPrCreate
case 'github.pr.merge':          → handleGitHubPrMerge
case 'github.issue.list':        → handleGitHubIssueList
case 'github.issue.create':      → handleGitHubIssueCreate

// gitlab.*
case 'gitlab.mr.create':         → handleGitLabMrCreate
case 'gitlab.pipeline.status':   → handleGitLabPipelineStatus
```

---

## 4. Relation to Existing Handlers

| Handler | Khi nào dùng | Ai gọi |
|---------|-------------|--------|
| `agent-git-handler.ts:handleGitExec` | git subcommands (status, diff, add...) | Orca UI (Remote Git Panel) |
| `agent-git-handler.ts:handleGitPrCreate` | `git.pr.create` — wrapper quanh `gh pr create` | Orca UI (Commit Panel) |
| **`external-api-connector.ts:handleGitHubPrCreate`** | `github.pr.create` — full PR với idempotency + JSON response | Workflow Orchestrator |
| **`external-api-connector.ts:handleGitLabMrCreate`** | `gitlab.mr.create` — glab MR | Workflow Orchestrator |

> **Quyết định design:** giữ cả 2 — `git.pr.create` (đơn giản, nhanh) và `github.pr.create` (idempotent, JSON response đầy đủ). Gateway chọn theo context.

---

## 5. Tests

Theo TDD-AG-13 §7:
- **Validation tests** (pure — không cần gh/glab binary):
  - SHELL_METACHARACTERS rejection: 3 tests
  - Missing params: 2 tests per handler
  - buildGhEnv isolation: 3 tests
  - buildGlabEnv isolation: 2 tests
- **execFileCaptured timeout**: 1 test (uses `sleep 100`)
- **Integration tests** (cần gh/glab — skip nếu không có):
  - `github.auth.status` nếu `gh` có trong PATH
  - `gitlab.auth.status` nếu `glab` có trong PATH

---

## 6. Implementation Checklist

- [ ] `src/relay/external-api-connector.ts` — tạo file mới (~350 lines)
- [ ] `buildGhEnv(userId)` — GH_CONFIG_DIR + GH_NO_UPDATE_NOTIFIER + GH_PROMPT_DISABLED
- [ ] `buildGlabEnv(userId)` — GLAB_CONFIG_DIR + NO_COLOR + CI
- [ ] `execFileCaptured()` — spawn no-shell, timeout 30s, capture stdout/stderr
- [ ] `getCurrentBranch()` — helper via `git rev-parse --abbrev-ref HEAD`
- [ ] `checkExistingPr()` — idempotency helper via `gh pr list --head`
- [ ] `handleGitHubPrCreate()` — validate + idempotency + execFile('gh')
- [ ] `handleGitHubPrMerge()` — `gh pr merge --squash`
- [ ] `handleGitHubIssueList()` — `gh issue list --json`
- [ ] `handleGitHubIssueCreate()` — `gh issue create`
- [ ] `handleGitHubAuthStatus()` — `gh auth status` (cho preflight)
- [ ] `handleGitLabMrCreate()` — `glab mr create --yes`
- [ ] `handleGitLabMrList()` — `glab mr list`
- [ ] `handleGitLabPipelineStatus()` — `glab pipeline status`
- [ ] `handleGitLabAuthStatus()` — `glab auth status` (cho preflight)
- [ ] `src/relay/agent-rpc-dispatch.ts` — thêm 6 case routes
- [ ] `src/relay/__tests__/external-api-connector.test.ts` — test file (≥20 tests)

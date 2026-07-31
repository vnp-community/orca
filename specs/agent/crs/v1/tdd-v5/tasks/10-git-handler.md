# TASK-10: Create src/relay/git-handler.ts

**Phase:** 5 (v5.0 extensions)  
**SOL Ref:** SOL-09  
**Estimated time:** 2h  
**Precondition:** TASK-08 (agent-connections) hoàn thành. TASK-06 (rpc-dispatch) phải tồn tại.  

---

## Tạo file mới: `src/relay/git-handler.ts`

### Imports

```typescript
import { spawn } from 'node:child_process'
import type WebSocket from 'ws'
import { encodeDataFrame } from './agent-wire'
import type { WireState } from './agent-wire'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
```

### Whitelist — ALLOWED_GIT_SUBCOMMANDS (Set<string>)

```typescript
const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',
  'config', 'describe', 'shortlog',
])
```

### SHELL_METACHARACTERS regex

```typescript
const SHELL_METACHARACTERS = /[&|;$`<>\\!]/
```

### GitValidationError class

```typescript
export class GitValidationError extends Error {
  constructor(
    public readonly code: 'GIT_NO_SUBCOMMAND' | 'GIT_DISALLOWED_SUBCOMMAND' | 'GIT_SHELL_METACHARACTER_IN_ARG',
    message: string
  ) {
    super(message)
    this.name = 'GitValidationError'
  }
}
```

### validateGitArgs() — exported

```typescript
export function validateGitArgs(args: string[]): void
// Throws GitValidationError on: empty args, disallowed subcommand, metachar in any arg
```

### handleGitExec() — exported

```typescript
export async function handleGitExec(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object>
```

Logic:
1. Parse `args` (Array<string>), `cwd` (string|workDir), `timeout` (min(x, 60_000))
2. `validateGitArgs(args)` → catch GitValidationError → return `InvalidParams (-32602)` error
3. `spawn('git', args, { cwd, env: config.toolEnv, shell: false, stdio: ['pipe','pipe','pipe'] })`
4. Collect stdout/stderr, on close → return `{ jsonrpc: '2.0', id, result: { stdout, stderr, exitCode } }`
5. Timeout → `child.kill('SIGTERM')` → return error response

### handleGitExecStream() — exported

```typescript
export async function handleGitExecStream(
  ws: WebSocket,
  wireState: WireState,
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<void>
```

Logic:
1. `validateGitArgs(args)` → error frame nếu invalid, return
2. `spawn('git', args, { shell: false })` 
3. `child.stdout.on('data')` → mỗi line → gửi frame `{ type: 'stream.chunk', line }`
4. `child.stderr.on('data')` → mỗi line → gửi frame `{ type: 'stream.chunk', line, source: 'stderr' }`
5. `child.on('close', code)` → gửi frame `{ type: 'stream.end', exitCode: code ?? 0 }`
6. Luôn check `ws.readyState === 1` trước khi gửi

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "git-handler" || echo "No errors"
```

## Definition of Done

- [x] `src/relay/git-handler.ts` created
- [x] `validateGitArgs`, `handleGitExec`, `handleGitExecStream` exported
- [x] `GitValidationError` exported
- [x] `ALLOWED_GIT_SUBCOMMANDS` có 23 entries
- [x] `SHELL_METACHARACTERS` catches `& | ; $ \` < > \ !`
- [x] `shell: false` trong cả 2 spawn calls
- [x] `pnpm run typecheck:node` passes

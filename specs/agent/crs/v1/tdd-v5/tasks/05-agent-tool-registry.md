# TASK-05: Create src/relay/agent-tool-registry.ts

**Phase:** 2  
**SOL Ref:** SOL-04  
**Estimated time:** 2h  
**Precondition:** TASK-03 (agent-config.ts) hoàn thành  

---

## Tạo file mới: `src/relay/agent-tool-registry.ts`

File này gồm 3 phần: `runToolCommand()`, `ToolDefinition` interface, và `ALL_TOOL_DEFINITIONS` array.

**QUAN TRỌNG:**
- `shell: false` trong `spawn()` — bắt buộc
- Handlers nhận `(params: Record<string, unknown>, config: AgentConfig)` — không dùng global
- Built-in tools (binary=null): `read_file`, `list_dir` — luôn available
- Tool `codex` và `codex_code` thêm nếu cần sau, hiện tại có 9 tools cơ bản

### Nội dung đầy đủ

Xem chi tiết implementation tại: `specs/agent/crs/v1/tdd-v5/solutions/04-agent-tool-registry.md`

**Các imports cần thiết:**
```typescript
import { spawn } from 'node:child_process'
import { accessSync, constants } from 'node:fs'
import { join, isAbsolute } from 'node:path'
import { readFile, readdir, stat } from 'node:fs/promises'
import type { AgentConfig } from './agent-config'
```

**Interface chính:**
```typescript
export interface ToolResult {
  stdout: string
  stderr: string
  exitCode: number
  meta?: Record<string, unknown>
}

export interface ToolDefinition {
  readonly name: string
  readonly binary: string | null   // null = built-in, no binary check
  readonly description: string
  readonly inputSchema: ToolInputSchema
  handler(params: Record<string, unknown>, config: AgentConfig): Promise<ToolResult>
}
```

**9 tools cần implement:**
1. `claude_code` (binary: `claude`) — `--print` mode, timeout 300s
2. `gh` (binary: `gh`) — GitHub CLI, timeout 60s
3. `git` (binary: `git`) — general git tool, timeout 60s
4. `gitnexus` (binary: `gitnexus`) — code intelligence, timeout 60s
5. `codegraph` (binary: `codegraph`) — code analysis, timeout 60s
6. `docker` (binary: `docker`) — container ops, timeout 120s
7. `shell` (binary: `bash`) — `bash -c`, timeout max 600s
8. `read_file` (binary: null) — built-in, async `readFile()`
9. `list_dir` (binary: null) — built-in, async `readdir()`

**`runToolCommand()` helper:**
```typescript
export function runToolCommand(
  binary: string,
  args: string[],
  opts: { cwd: string; timeout: number; env: NodeJS.ProcessEnv }
): Promise<ToolResult>
// spawn với shell: false, SIGTERM on timeout, stdout/stderr captured
```

**`discoverTools()` — pure function:**
```typescript
export async function discoverTools(config: AgentConfig): Promise<ToolDefinition[]>
// Checks binary exists in config.toolPath, always includes binary=null tools
```

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-tool-registry" || echo "No errors"
```

## Definition of Done

- [x] `src/relay/agent-tool-registry.ts` created (~150 lines)
- [x] All 9 tool definitions present
- [x] `shell: false` trong mọi spawn call
- [x] `discoverTools()` exported, pure function, accepts `AgentConfig`
- [x] `runToolCommand()` exported
- [x] `ToolDefinition`, `ToolResult` interfaces exported
- [x] `pnpm run typecheck:node` passes

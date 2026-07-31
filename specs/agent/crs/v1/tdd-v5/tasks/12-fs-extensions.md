# TASK-12: Create src/relay/fs-agent-extensions.ts

**Phase:** 5 (v5.0 extensions)  
**SOL Ref:** SOL-11  
**Estimated time:** 2h  
**Precondition:** TASK-03 hoàn thành. `fs-handler-file-read.ts` và `fs-handler-utils.ts` đã tồn tại trong codebase.  

---

## Tạo file mới: `src/relay/fs-agent-extensions.ts`

### Imports — REUSE existing handlers

```typescript
import { readdir, stat } from 'node:fs/promises'
import { join, isAbsolute } from 'node:path'
import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
// REUSE existing:
import { readRelayFileContent } from './fs-handler-file-read'
import { checkRgAvailable } from './fs-handler-utils'
```

### 4 exported handler functions

#### handleFsReadDir()

```typescript
export async function handleFsReadDir(
  id: string | number | null,
  params: Record<string, unknown>,  // { path, depth? }
  config: AgentConfig
): Promise<object>
```

Logic:
- `depth = Math.min(params.depth ?? 1, 5)` — max depth 5
- `stat(absPath).isDirectory()` → return InvalidParams nếu không phải dir
- Đệ quy `readDirRecursive(dir, maxDepth, currentDepth)`
- Sort: directories trước, rồi files; alphabetically trong mỗi group
- Return `{ entries: FileTreeNode[], path: absPath }`

```typescript
interface FileTreeNode {
  path: string; name: string; type: 'file' | 'directory'
  size?: number; children?: FileTreeNode[]
}
```

#### handleFsReadFile()

```typescript
export async function handleFsReadFile(
  id: string | number | null,
  params: Record<string, unknown>,  // { path }
  config: AgentConfig
): Promise<object>
```

Logic:
- `readRelayFileContent(absPath)` — đã handle size limit (MAX_TEXT_FILE_SIZE = 10MB), binary detection
- Map errors: "File too large" → `InvalidParams`, "ENOENT" → `PathNotFound`, else → `ServerError`
- Return `{ content, encoding: 'utf-8'|'base64', isBinary, path }`

#### handleFsGrep()

```typescript
export async function handleFsGrep(
  id: string | number | null,
  params: Record<string, unknown>,  // { root?, pattern, maxResults? }
  config: AgentConfig
): Promise<object>
```

Logic:
- `maxResults = Math.min(params.maxResults ?? 50, 200)`
- `rgAvailable = await checkRgAvailable()` — reuse existing
- Nếu rg: `spawn('rg', ['--json', '--ignore-case', '--max-count', str, pattern, root], { shell: false })`
  - Parse JSON lines, type==='match' → collect `{ file, line, text }`
- Nếu không có rg: `spawn('grep', ['-r','-n','-i', pattern, root], { shell: false })`
  - Parse `file:line:text` format
- Return `{ matches: GrepMatch[], total, truncated: matches.length >= maxResults }`

#### handlePreflightCheck()

```typescript
export async function handlePreflightCheck(
  id: string | number | null,
  params: Record<string, unknown>,  // { services: string[] }
  config: AgentConfig
): Promise<object>
```

Logic:
- services: `['github-cli', 'ripgrep', 'docker', 'claude']`
- Mỗi service: spawn `binary --version`, timeout 5s → true/false
- `github-cli` → binary `gh`; `ripgrep` → binary `rg`
- Return `{ [service]: boolean }`

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "fs-agent-extensions" || echo "No errors"

# Verify imports resolve
grep "readRelayFileContent\|checkRgAvailable" src/relay/fs-agent-extensions.ts
node -e "require('./out/relay/agent.js')" 2>&1 | head -3  # after build
```

## Definition of Done

- [x] `src/relay/fs-agent-extensions.ts` created
- [x] `handleFsReadDir`, `handleFsReadFile`, `handleFsGrep`, `handlePreflightCheck` exported
- [x] `readRelayFileContent` import từ `'./fs-handler-file-read'` resolves
- [x] `checkRgAvailable` import từ `'./fs-handler-utils'` resolves
- [x] `shell: false` trong mọi spawn call
- [x] `pnpm run typecheck:node` passes

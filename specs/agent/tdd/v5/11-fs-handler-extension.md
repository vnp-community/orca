# TDD-AG-11: FS Handler Extension (v5.0)

**Document:** TDD-AG-11 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Remote filesystem operations — readDir (depth), readFile (limit), grep (rg)
**Feature:** F38
**ADR:** ADR-011
**HLD Ref:** C3.12
**Backend TDD:** TDD-19
**Frontend TDD:** TDD-FE-17

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. New RPC Methods

| Method | Description |
|--------|-------------|
| `fs.readDir` | List directory with depth, git status overlay |
| `fs.readFile` | Read file with 1MB size limit + base64 for binary |
| `fs.grep` | Ripgrep/grep search in files |
| `preflight.check` | Check service availability on dev server |

---

## 2. fs.readDir Handler

```javascript
case 'fs.readDir':
  response = await handleFsReadDir(rpc);
  break;

async function handleFsReadDir(rpc) {
  const { path: dirPath, depth = 1 } = rpc.params;

  const absPath = path.isAbsolute(dirPath) ? dirPath : path.join(WORK_DIR, dirPath);

  if (!fs.existsSync(absPath) || !fs.statSync(absPath).isDirectory()) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32602, message: `Not a directory: ${absPath}` } };
  }

  function readDirRecursive(dir, currentDepth) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    return entries.map(entry => {
      const fullPath = path.join(dir, entry.name);
      const node = {
        path: fullPath,
        name: entry.name,
        type: entry.isDirectory() ? 'directory' : 'file',
        size: entry.isFile() ? fs.statSync(fullPath).size : undefined,
      };
      if (entry.isDirectory() && currentDepth < depth) {
        node.children = readDirRecursive(fullPath, currentDepth + 1);
      }
      return node;
    }).sort((a, b) => {
      // Directories first, then files, alphabetically
      if (a.type !== b.type) return a.type === 'directory' ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
  }

  try {
    const entries = readDirRecursive(absPath, 1);
    return {
      jsonrpc: '2.0', id: rpc.id,
      result: { entries, path: absPath },
    };
  } catch (err) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32603, message: err.message } };
  }
}
```

---

## 3. fs.readFile Handler (with size limit)

```javascript
const FS_READ_MAX_SIZE = 1 * 1024 * 1024;  // 1MB

case 'fs.readFile':
  response = await handleFsReadFile(rpc);
  break;

async function handleFsReadFile(rpc) {
  const { path: filePath, encoding = 'utf-8', maxSize = FS_READ_MAX_SIZE } = rpc.params;

  const absPath = path.isAbsolute(filePath) ? filePath : path.join(WORK_DIR, filePath);

  if (!fs.existsSync(absPath)) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32602, message: `File not found: ${absPath}` } };
  }

  const stat = fs.statSync(absPath);
  const effectiveMax = Math.min(maxSize, FS_READ_MAX_SIZE);

  if (stat.size > effectiveMax) {
    return {
      jsonrpc: '2.0', id: rpc.id,
      error: {
        code: -32602,
        message: 'FILE_TOO_LARGE',
        data: { size: stat.size, maxSize: effectiveMax },
      },
    };
  }

  try {
    const content = fs.readFileSync(absPath);
    const isText = encoding === 'utf-8' && isTextBuffer(content);

    return {
      jsonrpc: '2.0', id: rpc.id,
      result: {
        content: isText ? content.toString('utf-8') : content.toString('base64'),
        encoding: isText ? 'utf-8' : 'base64',
        size: stat.size,
        path: absPath,
      },
    };
  } catch (err) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32603, message: err.message } };
  }
}

// Heuristic: treat as text if first 1000 bytes contain no null bytes
function isTextBuffer(buf) {
  const sample = buf.slice(0, Math.min(1000, buf.length));
  return !sample.includes(0);
}
```

---

## 4. fs.grep Handler

```javascript
case 'fs.grep':
  response = await handleFsGrep(rpc);
  break;

async function handleFsGrep(rpc) {
  const { root, pattern, maxResults = 50, includeContent = true, include = [] } = rpc.params;

  const absRoot = path.isAbsolute(root) ? root : path.join(WORK_DIR, root);

  // Prefer ripgrep (rg) over grep
  const rgBinary = TOOL_PATH.split(':').reduce((found, dir) => {
    if (found) return found;
    const candidate = path.join(dir, 'rg');
    try { fs.accessSync(candidate, fs.constants.X_OK); return candidate; } catch { return null; }
  }, null);

  let args;
  if (rgBinary) {
    args = [
      '--json',          // JSON output for structured parsing
      '--max-count', String(maxResults),
      '--ignore-case',
      ...include.map(p => ['-g', p]).flat(),
      pattern,
      absRoot,
    ];
  } else {
    // Fallback: grep -r
    args = [
      '-r', '-n', '-i',
      '--include=*.ts', '--include=*.js', '--include=*.go',  // sensible defaults
      '--max-count=1',
      `-m${maxResults}`,
      pattern,
      absRoot,
    ];
  }

  const result = await runCommandCapture(rgBinary || 'grep', args, { cwd: WORK_DIR, timeout: 15000 });

  if (result.exitCode !== 0 && result.exitCode !== 1) {  // 1 = no matches (ok)
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32603, message: result.stderr } };
  }

  // Parse rg JSON output
  const matches = [];
  if (rgBinary) {
    for (const line of result.stdout.split('\n')) {
      if (!line.trim()) continue;
      try {
        const obj = JSON.parse(line);
        if (obj.type === 'match') {
          matches.push({
            file: obj.data.path.text,
            line: obj.data.line_number,
            text: obj.data.lines.text.trimEnd(),
          });
          if (matches.length >= maxResults) break;
        }
      } catch { /* skip non-JSON lines */ }
    }
  }

  return {
    jsonrpc: '2.0', id: rpc.id,
    result: { matches, total: matches.length, truncated: matches.length >= maxResults },
  };
}
```

---

## 5. preflight.check Handler

```javascript
case 'preflight.check':
  response = await handlePreflightCheck(rpc);
  break;

async function handlePreflightCheck(rpc) {
  const { services = [] } = rpc.params;
  const results = {};

  for (const service of services) {
    switch (service) {
      case 'github-cli': {
        const r = await runCommandCapture('gh', ['--version'], { cwd: WORK_DIR, timeout: 5000 });
        results['github-cli'] = r.exitCode === 0;
        break;
      }
      case 'ripgrep': {
        const r = await runCommandCapture('rg', ['--version'], { cwd: WORK_DIR, timeout: 5000 });
        results['ripgrep'] = r.exitCode === 0;
        break;
      }
      case 'docker': {
        const r = await runCommandCapture('docker', ['info', '--format', '{{.ServerVersion}}'], { cwd: WORK_DIR, timeout: 5000 });
        results['docker'] = r.exitCode === 0;
        break;
      }
      default:
        results[service] = false;
    }
  }

  return { jsonrpc: '2.0', id: rpc.id, result: results };
}
```

---

## 6. Test Coverage

```
tests/unit/
├── fs-read-dir.test.js
│   ├── returns entries sorted (dirs first, then files alphabetically)
│   ├── depth=1: only immediate children (no grandchildren)
│   ├── depth=2: includes children of subdirs
│   ├── path not a directory → error -32602
│   └── file has type='file', dir has type='directory'
├── fs-read-file.test.js
│   ├── reads text file → encoding='utf-8'
│   ├── reads binary file → encoding='base64'
│   ├── file > 1MB → FILE_TOO_LARGE error
│   ├── file not found → error -32602
│   └── maxSize param capped at FS_READ_MAX_SIZE
├── fs-grep.test.js
│   ├── rg available: uses rg --json
│   ├── rg not available: falls back to grep
│   ├── returns matches with file, line, text
│   ├── maxResults=50: truncated=true when exceeded
│   └── exitCode=1 (no matches): empty array returned
└── preflight-check.test.js
    ├── github-cli: gh --version exits 0 → true
    ├── github-cli: not installed → false
    └── unknown service → false
```

**Target:** ≥ 20 tests

---

## v2.1 Integration Note

**Reuse existing files:**
- `src/relay/fs-handler-file-read.ts` → `readRelayFileContent()` 
- `src/relay/fs-handler-list-files.ts` → `listRelayFiles()`
- `src/relay/fs-handler-rg-availability.ts` → rg detection
- `src/relay/fs-handler-utils.ts` → `isBinaryBuffer()`, size constants

`fs.readDir`, `fs.readFile`, `fs.grep` handler code trong `agent-rpc-dispatch.ts` import và gọi các functions trên.

**Test files:** `src/relay/__tests__/fs-handler-*.test.ts` (nhiều file đã có!)

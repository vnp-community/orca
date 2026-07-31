# TASK-DS-004 — Implement `fs.listDirectory` + `fs.stat` + `fs.listWorkspaces`

**Solution:** [SOL-DS-002](../solutions/SOL-DS-002-agent-relay-rpc-methods.md)  
**Bug:** [BUG-DS-002](../BUG-DS-002-rpc-method-mismatch.md)  
**File:** `deploy/dev/agent/agent.js`  
**Phụ thuộc:** TASK-DS-002 (helpers cần có sẵn)  
**Estimated:** 30 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thêm 3 filesystem RPC method handlers vào `dispatchRpc()`:
1. `fs.listDirectory` — list files trong thư mục
2. `fs.stat` — kiểm tra file/directory tồn tại
3. `fs.listWorkspaces` — list git repos trong WORK_DIR

Được gọi bởi `repo-remote-ipc.ts` khi user browse remote filesystem.

---

## Context

Đọc trước:
- `deploy/dev/agent/agent.js` — biến `WORK_DIR` hoặc `WORKSPACE_DIR` (thư mục làm việc mặc định của agent)
- `src/main/ipc/repo-remote-ipc.ts` dòng 40-60 — format response expected

**Format response:**
```javascript
// fs.listDirectory:
{
  entries: Array<{
    name: string,
    path: string,
    isDirectory: boolean,
    isFile: boolean,
    isSymlink: boolean
  }>,
  platform: NodeJS.Platform
}

// fs.stat:
{ exists: boolean, isDirectory?: boolean, isFile?: boolean, size?: number, mtime?: number }

// fs.listWorkspaces:
{ entries: Array<{ name: string, path: string, isGitRepo: boolean }>, platform: NodeJS.Platform }
```

---

## Thay Đổi Cần Thực Hiện

**File:** `deploy/dev/agent/agent.js`

Thêm 3 case này vào `dispatchRpc()` (sau các `preflight.*` cases):

```javascript
    case 'fs.listDirectory': {
      const fs   = require('fs');
      const path = require('path');
      // Default về WORK_DIR nếu không truyền path
      const dirPath = msg.params?.path ?? WORK_DIR ?? process.env.HOME ?? '/';
      try {
        const items   = fs.readdirSync(dirPath, { withFileTypes: true });
        const entries = items.map(item => ({
          name:        item.name,
          path:        path.join(dirPath, item.name),
          isDirectory: item.isDirectory(),
          isFile:      item.isFile(),
          isSymlink:   item.isSymbolicLink(),
        }));
        response = {
          jsonrpc: '2.0', id: msg.id,
          result: { entries, platform: process.platform },
        };
      } catch (err) {
        response = {
          jsonrpc: '2.0', id: msg.id,
          error: { code: -32000, message: `fs.listDirectory failed: ${err.message}` },
        };
      }
      break;
    }

    case 'fs.stat': {
      const fs   = require('fs');
      const statPath = msg.params?.path;
      if (!statPath) {
        response = { jsonrpc: '2.0', id: msg.id,
          error: { code: -32602, message: 'params.path is required' } };
        break;
      }
      try {
        const stat = fs.statSync(statPath);
        response = {
          jsonrpc: '2.0', id: msg.id,
          result: {
            exists:      true,
            isDirectory: stat.isDirectory(),
            isFile:      stat.isFile(),
            size:        stat.size,
            mtime:       stat.mtimeMs,
          },
        };
      } catch {
        // Không throw — trả exists: false
        response = { jsonrpc: '2.0', id: msg.id, result: { exists: false } };
      }
      break;
    }

    case 'fs.listWorkspaces': {
      const fs   = require('fs');
      const path = require('path');
      const rootPath = msg.params?.path ?? WORK_DIR ?? process.env.HOME ?? '/';
      const entries  = [];
      try {
        const items = fs.readdirSync(rootPath, { withFileTypes: true });
        for (const item of items) {
          if (!item.isDirectory()) continue;
          const fullPath = path.join(rootPath, item.name);
          const gitDir   = path.join(fullPath, '.git');
          let isGitRepo  = false;
          try { fs.accessSync(gitDir); isGitRepo = true; } catch { /* not a git repo */ }
          entries.push({ name: item.name, path: fullPath, isGitRepo });
        }
      } catch (err) {
        response = {
          jsonrpc: '2.0', id: msg.id,
          error: { code: -32000, message: `fs.listWorkspaces failed: ${err.message}` },
        };
        break;
      }
      response = {
        jsonrpc: '2.0', id: msg.id,
        result: { entries, platform: process.platform },
      };
      break;
    }
```

> [!IMPORTANT]
> Đảm bảo biến `WORK_DIR` đã được define trong agent.js (thường là `process.env.WORK_DIR || process.env.HOME`). Nếu chưa có, thêm ở đầu file: `const WORK_DIR = process.env.WORK_DIR ?? process.env.HOME ?? '/home/ubuntu';`

---

## Verify

```bash
# Deploy + restart agent
bash deploy/dev/scripts/connect-agent.sh --deploy

# Trong Orca UI:
# Dev Servers → [server] → "Browse" hoặc "Open Folder"
# Expected: hiện file browser với danh sách thư mục ✅

# Test nhanh qua curl nếu có HTTP debug endpoint:
# hoặc xem agent logs khi UI gọi:
bash deploy/dev/scripts/connect-agent.sh --logs | grep "fs\."
```

---

## Definition of Done

- [x] `case 'fs.listDirectory'` trả về `{ entries, platform }` đúng format
- [x] `case 'fs.stat'` trả về `{ exists: false }` thay vì throw khi path không tồn tại
- [x] `case 'fs.listWorkspaces'` phân biệt git repos với thư mục thường
- [x] WORK_DIR fallback về `os.homedir()` nếu không set
- [x] agent.js syntax check OK (`node --check`)

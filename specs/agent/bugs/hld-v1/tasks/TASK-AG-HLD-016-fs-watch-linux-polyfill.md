# TASK-AG-HLD-016 — Linux Recursive-Watch Polyfill Cho `fs.watch`

**Solution:** [SOL-AG-HLD-010](../solutions/SOL-AG-HLD-010-fs-watch-linux-recursive-polyfill.md)
**Bug:** [BUG-AG-HLD-010](../BUG-AG-HLD-010-fs-watch-recursive-linux-top-level-only.md)
**File:** `agent/src/relay/fs-agent-extensions.ts`
**Phụ thuộc:** —
**Estimated:** 75 phút
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

Polyfill recursive watch trên Linux (Node bỏ qua `recursive: true` trên nền tảng này) bằng cách tự walk cây thư mục và gắn `fs.watch(dir, { recursive: false })` cho từng thư mục con, mở rộng động khi có thư mục con mới, cap giới hạn số thư mục theo dõi, và tái sử dụng `WATCHER_IGNORE_DIRS` — không thêm dependency mới, không đụng `agent/build.mjs` hay bất kỳ build pipeline nào.

---

## Context

Đọc trước khi thực thi (không cần đọc thêm gì khác — task này tự đủ):

- `agent/src/relay/fs-agent-extensions.ts` — file duy nhất cần sửa. Chứa `handleFsWatch`, `handleFsUnwatch`, `cleanupAgentWatches`, interface `AgentWatchEntry`, và map `AGENT_WATCH_MAP` (dòng ~587-671 hiện tại).
- `agent/src/main/ipc/filesystem-watcher-ignore.ts` — nguồn `WATCHER_IGNORE_DIRS` (mảng tên thư mục cần bỏ qua khi walk: `.git`, `node_modules`, `dist`, `build`, `.next`, `.cache`, `target`, `.venv`, `__pycache__`). Chỉ import hằng số này, không kéo theo `@parcel/watcher` hay bất kỳ phần nào khác của file đó.
- `agent/src/relay/agent-rpc-dispatch.ts:736-737` — caller duy nhất của `handleFsWatch`, gọi qua dynamic `import()` trong `case 'fs.watch'`. Chữ ký hàm export **không đổi** trong task này nên file đó **không cần sửa**.

**Vì sao không dùng lại cluster `@parcel/watcher` (đã có sẵn cho nhánh SSH-relay trong `relay.ts`)?** Đã kiểm tra và loại bỏ trong solution — 3 lý do độc lập: (1) `agent/build.mjs` chỉ build 1 entry point (`src/relay/agent-entry.ts` → `out/agent.js`), không có step build cho child-process entry mà `WatcherProcessSupervisor` cần; (2) `getWatcherProcessEntryPath()` phụ thuộc cứng vào Electron (`require('electron')`), không tồn tại trên Dev Server chạy `node out/agent.js` thuần; (3) `@parcel/watcher` là `external` trong esbuild config — không được bundle vào `out/agent.js`, và đưa native `.node` binding vào runtime nghĩa là phải ship prebuilds cho mọi Linux distro/arch, một hạng mục devops không tương xứng với mức độ bug 🟢 Low. Vì vậy giải pháp đi theo hướng tự implement walk + `fs.watch` per-directory, không cần dependency mới (`chokidar` bị loại vì over-engineering cho mức độ bug này, xem so sánh trong solution).

**Trade-off cố hữu (chấp nhận được):** mỗi thư mục con tốn 1 inotify watch descriptor (`fs.inotify.max_user_watches`, mặc định 8192-524288 tuỳ distro). Cây quá lớn có thể chạm giới hạn — cap nội bộ `MAX_LINUX_WATCH_DIRS = 4000` dừng mở watcher mới một cách êm ái (không throw) khi chạm giới hạn, coverage giảm dần về đúng hành vi baseline hiện tại (top-level-only) cho phần cây vượt cap — không tệ hơn trạng thái trước khi fix.

---

## Thay Đổi Cần Thực Hiện

### Bước 1 — Thêm import mới

**TÌM** (đầu file `agent/src/relay/fs-agent-extensions.ts`, dòng 6-14):

```typescript
import { readdir, stat, writeFile, mkdir, rmdir as fsRmdir, rm } from 'node:fs/promises'
import { watch as fsWatchSync, type FSWatcher } from 'node:fs'
import { join, isAbsolute, resolve as resolvePath, dirname } from 'node:path'
import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { readRelayFileContent } from './fs-handler-file-read'
import { checkRgAvailable } from './fs-handler-utils'
import { createTracer } from '../shared/trace'
```

**THAY BẰNG:**

```typescript
import { readdir, stat, writeFile, mkdir, rmdir as fsRmdir, rm } from 'node:fs/promises'
import { watch as fsWatchSync, type FSWatcher } from 'node:fs'
import { join, isAbsolute, resolve as resolvePath, dirname, relative } from 'node:path'
import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { readRelayFileContent } from './fs-handler-file-read'
import { checkRgAvailable } from './fs-handler-utils'
import { createTracer } from '../shared/trace'
// Why: same high-churn exclusion list @parcel/watcher subscriptions use
// (main/ipc cluster) — reused here as a plain directory-name filter so the
// Linux per-directory polyfill doesn't crawl node_modules/.git/dist/etc.
import { WATCHER_IGNORE_DIRS } from '../main/ipc/filesystem-watcher-ignore'
```

(Chỉ thêm `relative` vào import `node:path` hiện có, và thêm 1 import mới — không tạo import `node:path` thứ hai.)

---

### Bước 2 — Đổi `AgentWatchEntry.watcher` thành `close`, thêm `watchDirLinux` + `handleLinuxWatchEvent`

**TÌM** (dòng ~593-598):

```typescript
interface AgentWatchEntry {
  watcher: FSWatcher
  refCount: number
}

const AGENT_WATCH_MAP = new Map<string, AgentWatchEntry>()
```

**THAY BẰNG:**

```typescript
interface AgentWatchEntry {
  close: () => void
  refCount: number
}

const AGENT_WATCH_MAP = new Map<string, AgentWatchEntry>()

const MAX_LINUX_WATCH_DIRS = 4000

/**
 * Recursively fs.watch() every subdirectory under rootAbsPath, skipping
 * WATCHER_IGNORE_DIRS entries. Node's fs.watch(recursive:false) only reports
 * events for directories it was given directly — a freshly created
 * subdirectory needs its own watcher, so handleLinuxWatchEvent extends the
 * set dynamically instead of walking once at setup time.
 */
async function watchDirLinux(
  rootAbsPath: string,
  dirAbsPath: string,
  watchers: Map<string, FSWatcher>,
  notify: (method: string, params: Record<string, unknown>) => void,
): Promise<void> {
  if (watchers.has(dirAbsPath) || watchers.size >= MAX_LINUX_WATCH_DIRS) {
    return
  }
  let watcher: FSWatcher
  try {
    watcher = fsWatchSync(dirAbsPath, { recursive: false }, (eventType, filename) => {
      handleLinuxWatchEvent(rootAbsPath, dirAbsPath, eventType, filename, watchers, notify)
    })
  } catch {
    return // dir vanished between readdir and watch — not fatal, skip it
  }
  watcher.on('error', () => {
    watchers.delete(dirAbsPath)
  })
  watchers.set(dirAbsPath, watcher)

  let entries: Awaited<ReturnType<typeof readdir<{ withFileTypes: true }>>>
  try {
    entries = await readdir(dirAbsPath, { withFileTypes: true })
  } catch {
    return
  }
  for (const entry of entries) {
    if (!entry.isDirectory() || WATCHER_IGNORE_DIRS.includes(entry.name)) {
      continue
    }
    await watchDirLinux(rootAbsPath, join(dirAbsPath, entry.name), watchers, notify)
  }
}

function handleLinuxWatchEvent(
  rootAbsPath: string,
  dirAbsPath: string,
  eventType: string,
  filename: string | null,
  watchers: Map<string, FSWatcher>,
  notify: (method: string, params: Record<string, unknown>) => void,
): void {
  const changedAbsPath = filename ? join(dirAbsPath, filename) : dirAbsPath
  const relFromRoot = relative(rootAbsPath, changedAbsPath) || '.'
  // Why: keep the wire shape identical to the macOS/Windows native-recursive
  // branch — filename is root-relative there too, so the client's fs.changed
  // handler doesn't need a platform switch.
  notify('fs.changed', { path: rootAbsPath, eventType, filename: relFromRoot })

  stat(changedAbsPath).then(
    (st) => {
      if (st.isDirectory() && !WATCHER_IGNORE_DIRS.includes(filename ?? '')) {
        void watchDirLinux(rootAbsPath, changedAbsPath, watchers, notify)
      }
    },
    () => {
      // ENOENT: removed. If changedAbsPath was itself a watched directory,
      // drop its watcher so a later re-create under the same name re-walks
      // cleanly instead of reusing a dead descriptor.
      const removed = watchers.get(changedAbsPath)
      if (removed) {
        removed.close()
        watchers.delete(changedAbsPath)
      }
    }
  )
}
```

---

### Bước 3 — Sửa `handleFsWatch`: nhánh theo platform

**TÌM** (dòng ~618-636 — toàn bộ khối `try` trong `handleFsWatch`):

```typescript
  try {
    // Why: recursive:true is only honored by Node on macOS/Windows. On Linux
    // this watches the top-level directory only — still covers the common
    // create/delete/rename-at-this-level case; deeper changes rely on the
    // client's periodic re-read until a Linux-side recursive strategy lands.
    const watcher = fsWatchSync(absPath, { recursive: process.platform !== 'linux' }, (eventType, filename) => {
      notify('fs.changed', { path: absPath, eventType, filename: filename ?? null })
    })
    watcher.on('error', (err: Error) => {
      notify('fs.changed', { path: absPath, eventType: 'error', filename: null, error: err.message })
      AGENT_WATCH_MAP.delete(absPath)
    })
    AGENT_WATCH_MAP.set(absPath, { watcher, refCount: 1 })
    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `fs.watch failed: ${msg}` } }
  }
}
```

**THAY BẰNG:**

```typescript
  try {
    if (process.platform === 'linux') {
      // Why: Node's recursive:true is silently ignored on Linux (Node docs).
      // Polyfill by fs.watch-ing every subdirectory individually — see
      // watchDirLinux/handleLinuxWatchEvent. Zero new deps, stays inside the
      // existing single-file esbuild bundle (agent/build.mjs has no pipeline
      // for a second child-process entry, which the @parcel/watcher cluster
      // used by relay.ts's SSH-relay branch would require).
      const watchers = new Map<string, FSWatcher>()
      await watchDirLinux(absPath, absPath, watchers, notify)
      AGENT_WATCH_MAP.set(absPath, {
        close: () => { for (const w of watchers.values()) w.close() },
        refCount: 1,
      })
      return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
    }

    const watcher = fsWatchSync(absPath, { recursive: true }, (eventType, filename) => {
      notify('fs.changed', { path: absPath, eventType, filename: filename ?? null })
    })
    watcher.on('error', (err: Error) => {
      notify('fs.changed', { path: absPath, eventType: 'error', filename: null, error: err.message })
      AGENT_WATCH_MAP.delete(absPath)
    })
    AGENT_WATCH_MAP.set(absPath, { close: () => watcher.close(), refCount: 1 })
    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `fs.watch failed: ${msg}` } }
  }
}
```

---

### Bước 4 — Sửa `handleFsUnwatch` và `cleanupAgentWatches`

**TÌM** (trong `handleFsUnwatch`, dòng ~646-653):

```typescript
  const entry = AGENT_WATCH_MAP.get(absPath)
  if (entry) {
    entry.refCount--
    if (entry.refCount <= 0) {
      entry.watcher.close()
      AGENT_WATCH_MAP.delete(absPath)
    }
  }
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
```

**THAY BẰNG:**

```typescript
  const entry = AGENT_WATCH_MAP.get(absPath)
  if (entry) {
    entry.refCount--
    if (entry.refCount <= 0) {
      entry.close()
      AGENT_WATCH_MAP.delete(absPath)
    }
  }
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
```

**TÌM** (trong `cleanupAgentWatches`, dòng ~663-668):

```typescript
  for (const [path, entry] of AGENT_WATCH_MAP.entries()) {
    try {
      entry.watcher.close()
    } catch {
      // best effort
    }
    AGENT_WATCH_MAP.delete(path)
  }
```

**THAY BẰNG:**

```typescript
  for (const [path, entry] of AGENT_WATCH_MAP.entries()) {
    try {
      entry.close()
    } catch {
      // best effort
    }
    AGENT_WATCH_MAP.delete(path)
  }
```

---

> [!IMPORTANT]
> **Không đổi:** wire contract của `fs.watch`/`fs.unwatch` (params/response shape), refcounting semantics, `handleFsUnwatch`'s absPath resolution logic, chữ ký export của `handleFsWatch`/`handleFsUnwatch`/`cleanupAgentWatches`, hay bất kỳ chỗ nào trong `agent-rpc-dispatch.ts`. `handleFsWatch` đã là `async function` sẵn — chỉ thêm `await` sâu hơn bên trong.
>
> **Trước khi apply diff:** chạy `impact({ target: 'handleFsWatch', direction: 'upstream' })` (GitNexus MCP) để xác nhận blast radius hiện tại vẫn khớp với ghi nhận trong solution (1 caller duy nhất: `agent-rpc-dispatch.ts` case `'fs.watch'`, risk LOW). Nếu risk trả về HIGH/CRITICAL hoặc xuất hiện caller mới, DỪNG và báo lại trước khi tiếp tục.

---

## Verify

Chạy trên máy Linux thật (không phải qua mock):

```bash
cd agent
pnpm build   # sinh out/agent.js — xác nhận build vẫn pass, không cần thêm bước build nào
```

**1) Unit test mới cho `handleFsWatch`/`handleFsUnwatch`** — thêm vào `src/relay/__tests__/fs-agent-extensions.test.ts`, cạnh các `describe()` khác hiện có:

- Tạo `tmpDir`, `mkdirSync(join(tmpDir, 'sub'))`, gọi `handleFsWatch({path: tmpDir})`.
- `writeFileSync(join(tmpDir, 'sub', 'new-file.txt'), 'x')`.
- Đợi `notify()` được gọi (poll/`setTimeout` ngắn — `fs.watch` là async theo OS).
- Assert `notify` được gọi với `path: tmpDir`, `filename` chứa `'sub/new-file.txt'`.
- Test thêm case: `mkdirSync` một thư mục con **mới** sau khi đã watch, rồi tạo file bên trong thư mục con đó → xác nhận watcher tự mở rộng (dynamic subdir detection) mà không cần `fs.unwatch`/`fs.watch` lại.
- Test `handleFsUnwatch`: gọi 2 lần watch (refCount=2), unwatch 1 lần → xác nhận watcher vẫn sống (đổi file vẫn bắn notify); unwatch lần 2 → watcher đóng (không còn notify sau đó).

```bash
pnpm vitest run src/relay/__tests__/fs-agent-extensions.test.ts
```

**2) Smoke test thủ công trên Dev Server Linux thật** (VM hoặc container Linux):

```bash
node out/agent.js &   # với ORCA_URL/AGENT_TOKEN trỏ tới 1 Orca server test

# Từ Orca client: mở Remote Workspace trỏ vào server này, browse tới 1 thư mục
mkdir -p /tmp/orca-watch-test/sub/nested
# Trong Orca UI, gọi fs.watch cho /tmp/orca-watch-test (qua file explorer mở path này)
echo "hello" > /tmp/orca-watch-test/sub/nested/new.txt
# Kỳ vọng: file explorer tự refresh / fs.changed notification bắn ra với
# filename ~ "sub/nested/new.txt" — KHÔNG cần đợi client polling fallback

mkdir -p /tmp/orca-watch-test/sub/another-new-dir
touch /tmp/orca-watch-test/sub/another-new-dir/marker
# Kỳ vọng: notification bắn ra dù thư mục "another-new-dir" được tạo SAU khi
# đã bắt đầu watch — xác nhận dynamic subdir detection hoạt động trên thư mục
# tạo mới, không chỉ thư mục có sẵn lúc watch bắt đầu

rm -rf /tmp/orca-watch-test/sub/nested
# Kỳ vọng: không có watcher leak — kiểm tra
ls -la /proc/$(pgrep -f 'out/agent.js')/fd | grep inotify | wc -l
# số lượng inotify fd giảm về đúng số dir còn lại sau khi 1 nhánh cây bị xoá
```

**3) Regression cho nhánh SSH-relay** (`relay.ts` / `RelayFilesystemWatchRegistry`) — KHÔNG bị đụng bởi thay đổi này, chỉ chạy lại test hiện có để xác nhận không có regression:

```bash
pnpm vitest run src/relay/fs-handler.test.ts src/relay/integration.test.ts
```

**4) Sau khi apply diff, xác nhận scope thay đổi:**

```
detect_changes({ scope: 'compare', base_ref: 'main' })
```

Phải chỉ liệt kê `handleFsWatch`, `handleFsUnwatch`, `cleanupAgentWatches`, `AgentWatchEntry`, `watchDirLinux`, `handleLinuxWatchEvent` trong `agent/src/relay/fs-agent-extensions.ts` — **không** lan sang `agent/src/main/ipc/*` hay `agent/src/relay/relay.ts`/`fs-handler.ts` (nhánh SSH-relay không đổi).

---

## Definition of Done

- [ ] `agent/src/relay/fs-agent-extensions.ts` biên dịch không lỗi (`pnpm build` từ `agent/` pass, sinh `out/agent.js` mà không cần thêm build step nào mới)
- [ ] macOS/Windows: `handleFsWatch` vẫn dùng `fs.watch(absPath, { recursive: true }, ...)` native — không thay đổi hành vi so với trước (chỉ đổi field `watcher` → `close` trong `AgentWatchEntry`, contract wire giữ nguyên)
- [ ] Linux: `handleFsWatch` route qua `watchDirLinux`, tạo 1 `fs.watch(dir, { recursive: false })` cho mỗi thư mục con (bỏ qua các tên trong `WATCHER_IGNORE_DIRS`)
- [ ] Linux: thư mục con tạo **sau khi** đã bắt đầu watch tự động được thêm vào tập watcher (qua `handleLinuxWatchEvent` gọi lại `watchDirLinux`) mà không cần `fs.unwatch`/`fs.watch` lại
- [ ] Cap `MAX_LINUX_WATCH_DIRS = 4000` hoạt động: khi số watcher đạt cap, `watchDirLinux` dừng mở watcher mới một cách êm ái (không throw, không crash)
- [ ] `WATCHER_IGNORE_DIRS` (từ `agent/src/main/ipc/filesystem-watcher-ignore.ts`) được áp dụng đúng khi walk cây (không watch bên trong `.git`, `node_modules`, `dist`, `build`, `.next`, `.cache`, `target`, `.venv`, `__pycache__`) và khi quyết định có tự-mở-rộng vào thư mục mới tạo hay không
- [ ] `fs.changed` notification trên Linux dùng `filename` là **root-relative path** (qua `relative(rootAbsPath, changedAbsPath)`), giữ shape giống hệt nhánh macOS/Windows để client không cần platform switch
- [ ] `handleFsUnwatch`: refcount giảm về 0 → gọi `entry.close()` (đóng toàn bộ N watcher trên Linux, hoặc 1 watcher trên macOS/Windows) và xoá khỏi `AGENT_WATCH_MAP` — không watcher leak (kiểm tra `/proc/<pid>/fd` không còn inotify fd thừa sau khi unwatch toàn bộ)
- [ ] `cleanupAgentWatches`: gọi `entry.close()` cho mọi entry còn lại khi session kết thúc — không đổi hành vi/thời điểm gọi so với trước
- [ ] Không thêm dependency mới vào `agent/package.json`; không sửa `agent/build.mjs`
- [ ] Chữ ký export của `handleFsWatch`, `handleFsUnwatch`, `cleanupAgentWatches` không đổi — `agent/src/relay/agent-rpc-dispatch.ts` không cần sửa
- [ ] Unit test mới trong `src/relay/__tests__/fs-agent-extensions.test.ts` pass (`pnpm vitest run src/relay/__tests__/fs-agent-extensions.test.ts`), bao gồm case dynamic subdir detection và refcount unwatch
- [ ] Regression test nhánh SSH-relay pass không đổi: `pnpm vitest run src/relay/fs-handler.test.ts src/relay/integration.test.ts`
- [ ] `detect_changes({ scope: 'compare', base_ref: 'main' })` chỉ liệt kê các symbol trong `agent/src/relay/fs-agent-extensions.ts` — không lan sang nhánh SSH-relay (`relay.ts`, `fs-handler.ts`, `agent/src/main/ipc/*`)

---

## Kết Quả Thực Thi (2026-08-09)

Đã implement polyfill đầy đủ trong `fs-agent-extensions.ts`: `watchDirLinux`/`handleLinuxWatchEvent` (walk + fs.watch per-directory, cap `MAX_LINUX_WATCH_DIRS=4000`, tái sử dụng `WATCHER_IGNORE_DIRS`), đổi `AgentWatchEntry.watcher`→`close`, nhánh platform trong `handleFsWatch` (Linux dùng polyfill, macOS/Windows giữ native `recursive:true`). Không thêm dependency mới, không sửa `agent/build.mjs`. Một fix nhỏ ngoài diff gốc: kiểu `Awaited<ReturnType<typeof readdir<...>>>` trong solution không compile được (readdir không generic theo cách đó) — đã sửa thành `Dirent[]` (import từ `node:fs`) để pass typecheck.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.

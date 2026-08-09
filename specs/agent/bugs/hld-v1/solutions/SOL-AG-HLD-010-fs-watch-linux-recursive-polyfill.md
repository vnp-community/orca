# SOL-AG-HLD-010 — Linux Recursive-Watch Polyfill Cho `fs.watch` (WS-agent branch)

**Fixes:** [BUG-AG-HLD-010](../BUG-AG-HLD-010-fs-watch-recursive-linux-top-level-only.md)
**TDD Ref:** TDD-AG-11 §7 (`fs.watch`/`fs.unwatch` — "Known v1 limitation" addendum)
**File:** `agent/src/relay/fs-agent-extensions.ts` (`handleFsWatch`, `handleFsUnwatch`, `cleanupAgentWatches`)
**Effort:** 3-5 giờ (impl + unit tests trên `AGENT_WATCH_MAP` refactor)
**Status:** 🔴 TODO — solution proposed 2026-08-09

---

## Phân Tích

### Bug tóm tắt

`handleFsWatch` (`agent/src/relay/fs-agent-extensions.ts:600-636`) gọi `fs.watch(absPath, { recursive: process.platform !== 'linux' }, ...)`. Trên Linux, Node bỏ qua option `recursive` — chỉ event ở top-level directory được push qua `fs.changed`. Đây là hành vi **đã được document là "known v1 limitation, deliberate"** trong TDD-AG-11 §7, với lý do: client-side (`DevServerFilesystemProvider.watch()`) có polling fallback bù cho phần thiếu. Bug report nâng mức độ vì AGENTS.md yêu cầu hành vi cross-platform không được âm thầm khác nhau, và Linux là platform phổ biến nhất cho Dev Server.

### Đề xuất trong bug report: tái sử dụng cluster `@parcel/watcher`

Bug report đề xuất route `fs.watch` (nhánh WS-agent, dùng bởi `agent.js`) qua cùng cơ chế `@parcel/watcher` cluster mà `agent/src/relay/relay.ts` (nhánh SSH-relay, qua `FsHandler` → `RelayFilesystemWatchRegistry` → `RelayWatcherProcessPool` → `RuntimeWatcherProcessPool` → `WatcherProcessSupervisor`) đã dùng — cơ chế này hỗ trợ recursive đúng trên mọi platform qua native binding, có crash-fuse + quarantine pool.

**Đã kiểm tra và xác nhận: KHÔNG khả thi trong `agent/` hiện tại**, vì 3 lý do độc lập, mỗi lý do đủ để chặn:

1. **`agent/build.mjs` không build artifact cần cho cluster.** Entry point duy nhất được build là `AGENT_ENTRY = src/relay/agent-entry.ts` → `out/agent.js` (một file CJS bundle độc lập). `WatcherProcessSupervisor` cần fork một **process con riêng** chạy `parcel-watcher-process-entry.js` (xem `parcel-watcher-child-launch.ts:23` — `fork(entryPath, ...)`). File entry đó **không tồn tại trong output của `agent/build.mjs`** — không có step nào compile nó. Bug report tự ghi nhận đúng điều này ("không có build pipeline xác nhận được").

2. **`getWatcherProcessEntryPath()` phụ thuộc cứng vào Electron.**
   ```typescript
   // agent/src/main/ipc/parcel-watcher-entry-path.ts:6-12, 31-37
   function loadElectronApp(): ElectronAppPath | null {
     try { return require('electron').app ?? null } catch { return null }
   }
   export function getWatcherProcessEntryPath(): string {
     const app = loadElectronApp()
     return resolveWatcherProcessEntryPath(app?.getAppPath() ?? process.cwd(), app?.isPackaged === true)
   }
   ```
   `agent.js` chạy bằng `node out/agent.js` (`package.json` → `"start": "node out/agent.js"`) trên Dev Server — **không có Electron trong runtime này** (Dev Server là máy Linux/macOS/Windows thuần, không cài Electron). `require('electron')` sẽ throw `MODULE_NOT_FOUND` → catch → `app = null` → fallback về `join(process.cwd(), 'out', 'main', 'parcel-watcher-process-entry.js')`, path không hề tồn tại trên Dev Server. Muốn dùng lại hàm này phải viết một entry-path resolver hoàn toàn khác cho ngữ cảnh standalone-agent, không phải "tái sử dụng" nữa mà là viết mới.

3. **`@parcel/watcher` là `external` trong esbuild config, và là native addon per-platform/arch.**
   ```javascript
   // agent/build.mjs:46
   external: ['node-pty', 'better-sqlite3', 'keytar', '@parcel/watcher', 'electron'],
   ```
   Nó có trong `dependencies` của `agent/package.json` (dòng 24: `"@parcel/watcher": "^2.5.6"`) nhưng bị đánh dấu `external` nên esbuild **không bundle nó vào `out/agent.js`** — nó phải được `require()` từ `node_modules/@parcel/watcher` cạnh `agent.js` lúc runtime, kèm đúng native `.node` binding cho OS/arch của Dev Server. Cơ chế deploy hiện tại của `agent.js` là **một file bundle độc lập** (đúng như mô tả trong `build.mjs`: "Output: out/agent.js (standalone Node.js CJS bundle)") — không có bước nào ship `node_modules` cùng nó. Thêm `@parcel/watcher` vào đường đi thật sự chạy nghĩa là phải build/ship native prebuilds cho mọi tổ hợp Linux distro/arch mà Dev Server có thể chạy — một hạng mục devops mới, không nhỏ, cho một bug **🟢 Low**.

**Kết luận:** route qua cluster `@parcel/watcher` là hướng (a) trong yêu cầu — bị loại vì 3 lý do build/bundle trên. Chọn hướng (b): polyfill recursive watch cho Linux bằng cách tự walk cây thư mục và gắn `fs.watch(dir, { recursive: false })` cho từng thư mục con, duy trì động khi thư mục con được tạo/xoá.

### So sánh (b1) tự implement walk+fs.watch vs (b2) chokidar

| | (b1) Tự implement (khuyến nghị) | (b2) `chokidar` |
|---|---|---|
| Dependency mới | Không — chỉ dùng `node:fs`/`node:path` built-in | Có — thêm gói vào `dependencies`, phải audit + theo dõi CVE |
| Bundle | Nằm gọn trong `out/agent.js` hiện có, không cần build step mới | esbuild bundle được (chokidar là pure-JS, không native) nhưng tăng kích thước `out/agent.js` (~50-100KB) |
| Tích hợp refcount hiện có | Khớp thẳng vào `AGENT_WATCH_MAP` (chỉ đổi field `watcher` → `close`) | Phải viết adapter map `FSWatcher`-shape ↔ chokidar's `FSWatcher` API (khác interface) |
| Độ trưởng thành xử lý edge case (atomic rename, watch limit backoff) | Tự chịu trách nhiệm, rủi ro thiếu case | Tốt hơn — chokidar đã xử lý nhiều edge case qua nhiều năm |
| Phù hợp mức độ bug | Có — 🟢 Low, đã có polling fallback phía client bù | Over-engineering cho mức độ Low |
| Nhất quán style codebase | Khớp — `fs-agent-extensions.ts` đã tự viết walk đệ quy (`readDirRecursive`, `handleFsGlob` dùng `find` CLI) thay vì kéo thư viện | Lệch style |

**Chọn (b1).** Lý do quyết định: (1) không đụng vào `agent/build.mjs`/thêm dependency mới — rủi ro build/bundle bằng 0; (2) TDD-AG-11 §7 đã tự nhận đây là completeness gap có polling fallback bù, không phải correctness bug, nên effort bỏ ra nên tỉ lệ thuận — polyfill ~100 dòng trong 1 file, không phải một build pipeline mới; (3) tái dùng được `WATCHER_IGNORE_DIRS` (`agent/src/main/ipc/filesystem-watcher-ignore.ts`) — một hằng số thuần, không kéo theo `@parcel/watcher`.

**Trade-off cần chấp nhận với (b1):** mỗi thư mục con watch tốn 1 inotify watch descriptor (`fs.inotify.max_user_watches`, mặc định phân phối Linux thường 8192-524288). Cây quá lớn (hàng chục nghìn thư mục, sau khi trừ `WATCHER_IGNORE_DIRS`) có thể chạm giới hạn hệ thống hoặc bộ đếm nội bộ `MAX_LINUX_WATCH_DIRS`. Đây là giới hạn cố hữu của mọi giải pháp dùng 1-inotify-watch-per-directory (kể cả `chokidar`); cluster `@parcel/watcher` tránh được vì dùng `fanotify`/kernel recursive API khi có, nhưng hướng đó đã bị loại ở trên vì lý do build/bundle. Cap được set generously (4000 thư mục) và khi chạm cap sẽ dừng mở watcher mới (không throw), ghi log — coverage giảm dần về đúng hành vi hiện tại (top-level-only) cho phần cây vượt cap, không tệ hơn baseline.

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/fs-agent-extensions.ts`

**1. Thêm import** (đầu file, cạnh các import `node:fs`/`node:path` hiện có):

```diff
 import { readdir, stat, writeFile, mkdir, rmdir as fsRmdir, rm } from 'node:fs/promises'
 import { watch as fsWatchSync, type FSWatcher } from 'node:fs'
 import { join, isAbsolute, resolve as resolvePath, dirname } from 'node:path'
+import { relative } from 'node:path'
 import { spawn } from 'node:child_process'
 import type { AgentConfig } from './agent-config'
 import { AgentErrorCode } from '../shared/agent-wire-protocol'
 import { readRelayFileContent } from './fs-handler-file-read'
 import { checkRgAvailable } from './fs-handler-utils'
 import { createTracer } from '../shared/trace'
+// Why: same high-churn exclusion list @parcel/watcher subscriptions use
+// (main/ipc cluster) — reused here as a plain directory-name filter so the
+// Linux per-directory polyfill doesn't crawl node_modules/.git/dist/etc.
+import { WATCHER_IGNORE_DIRS } from '../main/ipc/filesystem-watcher-ignore'
```

**2. Thay `AgentWatchEntry.watcher: FSWatcher` bằng `close: () => void`** — để một entry có thể đại diện cho 1 watcher (macOS/Windows) hoặc N watcher (Linux polyfill) mà không đổi shape của `AGENT_WATCH_MAP`/refcount logic:

```diff
 interface AgentWatchEntry {
-  watcher: FSWatcher
+  close: () => void
   refCount: number
 }

 const AGENT_WATCH_MAP = new Map<string, AgentWatchEntry>()
+
+const MAX_LINUX_WATCH_DIRS = 4000
+
+/**
+ * Recursively fs.watch() every subdirectory under rootAbsPath, skipping
+ * WATCHER_IGNORE_DIRS entries. Node's fs.watch(recursive:false) only reports
+ * events for directories it was given directly — a freshly created
+ * subdirectory needs its own watcher, so handleLinuxWatchEvent extends the
+ * set dynamically instead of walking once at setup time.
+ */
+async function watchDirLinux(
+  rootAbsPath: string,
+  dirAbsPath: string,
+  watchers: Map<string, FSWatcher>,
+  notify: (method: string, params: Record<string, unknown>) => void,
+): Promise<void> {
+  if (watchers.has(dirAbsPath) || watchers.size >= MAX_LINUX_WATCH_DIRS) {
+    return
+  }
+  let watcher: FSWatcher
+  try {
+    watcher = fsWatchSync(dirAbsPath, { recursive: false }, (eventType, filename) => {
+      handleLinuxWatchEvent(rootAbsPath, dirAbsPath, eventType, filename, watchers, notify)
+    })
+  } catch {
+    return // dir vanished between readdir and watch — not fatal, skip it
+  }
+  watcher.on('error', () => {
+    watchers.delete(dirAbsPath)
+  })
+  watchers.set(dirAbsPath, watcher)
+
+  let entries: Awaited<ReturnType<typeof readdir<{ withFileTypes: true }>>>
+  try {
+    entries = await readdir(dirAbsPath, { withFileTypes: true })
+  } catch {
+    return
+  }
+  for (const entry of entries) {
+    if (!entry.isDirectory() || WATCHER_IGNORE_DIRS.includes(entry.name)) {
+      continue
+    }
+    await watchDirLinux(rootAbsPath, join(dirAbsPath, entry.name), watchers, notify)
+  }
+}
+
+function handleLinuxWatchEvent(
+  rootAbsPath: string,
+  dirAbsPath: string,
+  eventType: string,
+  filename: string | null,
+  watchers: Map<string, FSWatcher>,
+  notify: (method: string, params: Record<string, unknown>) => void,
+): void {
+  const changedAbsPath = filename ? join(dirAbsPath, filename) : dirAbsPath
+  const relFromRoot = relative(rootAbsPath, changedAbsPath) || '.'
+  // Why: keep the wire shape identical to the macOS/Windows native-recursive
+  // branch — filename is root-relative there too, so the client's fs.changed
+  // handler doesn't need a platform switch.
+  notify('fs.changed', { path: rootAbsPath, eventType, filename: relFromRoot })
+
+  stat(changedAbsPath).then(
+    (st) => {
+      if (st.isDirectory() && !WATCHER_IGNORE_DIRS.includes(filename ?? '')) {
+        void watchDirLinux(rootAbsPath, changedAbsPath, watchers, notify)
+      }
+    },
+    () => {
+      // ENOENT: removed. If changedAbsPath was itself a watched directory,
+      // drop its watcher so a later re-create under the same name re-walks
+      // cleanly instead of reusing a dead descriptor.
+      const removed = watchers.get(changedAbsPath)
+      if (removed) {
+        removed.close()
+        watchers.delete(changedAbsPath)
+      }
+    }
+  )
+}
```

**3. Sửa `handleFsWatch`** — nhánh theo platform, giữ nguyên contract input/output:

```diff
   const existing = AGENT_WATCH_MAP.get(absPath)
   if (existing) {
     existing.refCount++
     return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
   }

   try {
-    // Why: recursive:true is only honored by Node on macOS/Windows. On Linux
-    // this watches the top-level directory only — still covers the common
-    // create/delete/rename-at-this-level case; deeper changes rely on the
-    // client's periodic re-read until a Linux-side recursive strategy lands.
-    const watcher = fsWatchSync(absPath, { recursive: process.platform !== 'linux' }, (eventType, filename) => {
-      notify('fs.changed', { path: absPath, eventType, filename: filename ?? null })
-    })
-    watcher.on('error', (err: Error) => {
-      notify('fs.changed', { path: absPath, eventType: 'error', filename: null, error: err.message })
-      AGENT_WATCH_MAP.delete(absPath)
-    })
-    AGENT_WATCH_MAP.set(absPath, { watcher, refCount: 1 })
-    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
+    if (process.platform === 'linux') {
+      // Why: Node's recursive:true is silently ignored on Linux (Node docs).
+      // Polyfill by fs.watch-ing every subdirectory individually — see
+      // watchDirLinux/handleLinuxWatchEvent. Zero new deps, stays inside the
+      // existing single-file esbuild bundle (agent/build.mjs has no pipeline
+      // for a second child-process entry, which the @parcel/watcher cluster
+      // used by relay.ts's SSH-relay branch would require).
+      const watchers = new Map<string, FSWatcher>()
+      await watchDirLinux(absPath, absPath, watchers, notify)
+      AGENT_WATCH_MAP.set(absPath, {
+        close: () => { for (const w of watchers.values()) w.close() },
+        refCount: 1,
+      })
+      return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
+    }
+
+    const watcher = fsWatchSync(absPath, { recursive: true }, (eventType, filename) => {
+      notify('fs.changed', { path: absPath, eventType, filename: filename ?? null })
+    })
+    watcher.on('error', (err: Error) => {
+      notify('fs.changed', { path: absPath, eventType: 'error', filename: null, error: err.message })
+      AGENT_WATCH_MAP.delete(absPath)
+    })
+    AGENT_WATCH_MAP.set(absPath, { close: () => watcher.close(), refCount: 1 })
+    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
   } catch (err: unknown) {
     const msg = err instanceof Error ? err.message : String(err)
     return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `fs.watch failed: ${msg}` } }
   }
 }
```

**4. Sửa `handleFsUnwatch` và `cleanupAgentWatches`** — đổi `entry.watcher.close()` → `entry.close()`:

```diff
   const entry = AGENT_WATCH_MAP.get(absPath)
   if (entry) {
     entry.refCount--
     if (entry.refCount <= 0) {
-      entry.watcher.close()
+      entry.close()
       AGENT_WATCH_MAP.delete(absPath)
     }
   }
   return { jsonrpc: '2.0', id, result: { ok: true } }
 }

 export function cleanupAgentWatches(): void {
   for (const [path, entry] of AGENT_WATCH_MAP.entries()) {
     try {
-      entry.watcher.close()
+      entry.close()
     } catch {
       // best effort
     }
     AGENT_WATCH_MAP.delete(path)
   }
 }
```

**Không đổi:** wire contract (`fs.watch`/`fs.unwatch` params/response shape), refcounting semantics, `handleFsUnwatch`'s absPath resolution, hay bất kỳ chỗ nào khác trong `agent-rpc-dispatch.ts`. `handleFsWatch` chỉ trở thành `async` sâu hơn (đã là `async function` sẵn) — chữ ký hàm export không đổi, nên impact analysis (xem dưới) không lan ra caller.

---

## Impact Analysis (GitNexus)

`handleFsWatch` — 1 caller: `agent/src/relay/agent-rpc-dispatch.ts:736-737` (dynamic `import()` trong `case 'fs.watch'`), **không có test coverage hiện tại** cho `handleFsWatch`/`handleFsUnwatch` (`src/relay/__tests__/fs-agent-extensions.test.ts` chỉ cover `handleFsReadDir`/`handleFsReadFile`/`handlePreflightCheck`/`handleFsStat`/`handleFsGlob`/`handleFsWriteFile`). Risk: **LOW** — thay đổi cô lập trong 1 file, chữ ký export không đổi, không có caller nào khác phụ thuộc vào shape nội bộ `AgentWatchEntry` (nó không được export). `WATCHER_IGNORE_DIRS` import mới là read-only, không side effect.

Vì đây là nhiệm vụ viết tài liệu giải pháp (không sửa code thật), impact analysis này ghi nhận blast radius để người thực thi sau chạy `impact({ target: 'handleFsWatch', direction: 'upstream' })` xác nhận lại trước khi apply diff.

---

## Verification

Chạy trên máy Linux thật (không phải qua mock):

```bash
cd agent
pnpm build   # sinh out/agent.js — xác nhận build vẫn pass, không cần thêm bước build nào

# 1) Unit test mới cho handleFsWatch/handleFsUnwatch (thêm vào
#    src/relay/__tests__/fs-agent-extensions.test.ts, cạnh các describe() khác):
#    - tạo tmpDir, mkdirSync(join(tmpDir, 'sub')), gọi handleFsWatch({path: tmpDir})
#    - writeFileSync(join(tmpDir, 'sub', 'new-file.txt'), 'x')
#    - đợi notify() được gọi (poll/setTimeout ngắn — fs.watch là async theo OS)
#    - assert notify được gọi với path: tmpDir, filename chứa 'sub/new-file.txt'
#    - test thêm case: mkdirSync một thư mục con MỚI sau khi đã watch, rồi tạo
#      file bên trong thư mục con đó → xác nhận watcher tự mở rộng (dynamic
#      subdir detection) mà không cần fs.unwatch/fs.watch lại
#    - test handleFsUnwatch: gọi 2 lần watch (refCount=2), unwatch 1 lần →
#      xác nhận watcher vẫn sống (đổi file vẫn bắn notify); unwatch lần 2 →
#      watcher đóng (không còn notify sau đó)
pnpm vitest run src/relay/__tests__/fs-agent-extensions.test.ts

# 2) Smoke test thủ công trên Dev Server Linux thật (VM hoặc container Linux):
node out/agent.js &   # với ORCA_URL/AGENT_TOKEN trỏ tới 1 Orca server test
# Từ Orca client: mở Remote Workspace trỏ vào server này, browse tới 1 thư mục
mkdir -p /tmp/orca-watch-test/sub/nested
# Trong Orca UI, gọi fs.watch cho /tmp/orca-watch-test (qua file explorer mở path này)
echo "hello" > /tmp/orca-watch-test/sub/nested/new.txt
# Kỳ vọng: file explorer tự refresh / fs.changed notification bắn ra với
# filename ~ "sub/nested/new.txt" — KHÔNG cần đợi client polling fallback
touch /tmp/orca-watch-test/sub/another-new-dir/marker  # (mkdir trước) test tạo dir mới sau khi đã watch
rm -rf /tmp/orca-watch-test/sub/nested
# Kỳ vọng: không có watcher leak (kiểm tra `ls -la /proc/<pid>/fd | grep inotify | wc -l`
# giảm về đúng số dir còn lại sau khi 1 nhánh cây bị xoá)

# 3) Regression cho nhánh SSH-relay (relay.ts / RelayFilesystemWatchRegistry)
#    — KHÔNG bị đụng bởi thay đổi này, chỉ chạy lại test hiện có để xác nhận:
pnpm vitest run src/relay/fs-handler.test.ts src/relay/integration.test.ts
```

Sau khi apply diff thật: `detect_changes({ scope: 'compare', base_ref: 'main' })` phải chỉ liệt kê `handleFsWatch`, `handleFsUnwatch`, `cleanupAgentWatches`, `AgentWatchEntry`, `watchDirLinux`, `handleLinuxWatchEvent` trong `agent/src/relay/fs-agent-extensions.ts` — không lan sang `agent/src/main/ipc/*` hay `agent/src/relay/relay.ts`/`fs-handler.ts` (nhánh SSH-relay không đổi).

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/fs-agent-extensions.ts` | File duy nhất cần sửa — `handleFsWatch`, `handleFsUnwatch`, `cleanupAgentWatches`, `AgentWatchEntry`, thêm `watchDirLinux`/`handleLinuxWatchEvent` |
| `agent/src/relay/agent-rpc-dispatch.ts:732-753` | Caller `fs.watch`/`fs.unwatch` — không đổi (dynamic import, chữ ký giữ nguyên) |
| `agent/src/main/ipc/filesystem-watcher-ignore.ts` | Nguồn `WATCHER_IGNORE_DIRS` được tái sử dụng làm bộ lọc walk (chỉ đọc hằng số, không kéo theo `@parcel/watcher`) |
| `agent/build.mjs` | Xác nhận: KHÔNG đổi — polyfill nằm gọn trong entry point hiện có (`src/relay/agent-entry.ts`), không cần build step mới |
| `agent/package.json` | Xác nhận: KHÔNG cần thêm dependency — `@parcel/watcher` (dòng 24) tiếp tục chỉ phục vụ nhánh SSH-relay's Electron main-process cluster, không bị kéo vào `agent.js` |
| `agent/src/relay/relay.ts`, `agent/src/relay/fs-handler.ts`, `agent/src/relay/relay-filesystem-watch-registry.ts`, `agent/src/main/ipc/runtime-watcher-process-pool.ts`, `agent/src/main/ipc/parcel-watcher-process-supervisor.ts` | Nhánh SSH-relay (cluster `@parcel/watcher`, chạy trong Electron main process) — KHÔNG bị đụng bởi giải pháp này; giữ nguyên làm tài liệu tham khảo cho lý do (a) bị loại |
| `agent/src/relay/__tests__/fs-agent-extensions.test.ts` | Nơi thêm test coverage mới cho `handleFsWatch`/`handleFsUnwatch` (hiện chưa có, xác nhận qua GitNexus blast-radius) |
| `specs/agent/tdd/v5/11-fs-handler-extension.md §7` | TDD gốc — cần cập nhật "Known v1 limitation" thành "Linux: polyfilled via per-directory fs.watch, capped at 4000 dirs" sau khi fix merge |

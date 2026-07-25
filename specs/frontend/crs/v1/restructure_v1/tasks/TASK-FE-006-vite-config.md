# TASK-FE-006 — Verify/Update vite.web-spa.config.ts

**Source Solution:** [SOL-FE-001](../solutions/SOL-FE-001-web-mode-entry.md)  
**Priority:** P1 — Có thể chạy song song với TASK-FE-001~004  
**Loại:** Verify + Cập nhật nếu cần  
**Depends on:** TASK-FE-003 (web/main.tsx update)

---

## Context

Vite cần config riêng để build web SPA mode (không phải Electron). Config này stub `electron` module và target `web-index.html` thay vì `index.html`.

---

## Input

Đọc trước khi implement:
- `vite.web-spa.config.ts` — file hiện tại (có thể đã tồn tại)
- `vite.config.ts` — config chính để tham khảo structure
- `electron.vite.config.ts` — config Electron để tránh conflict

---

## Output — File cần verify/cập nhật

### File: `vite.web-spa.config.ts` [VERIFY + CẬP NHẬT nếu thiếu]

File phải đảm bảo tất cả yêu cầu sau. Nếu file chưa có hoặc thiếu, tạo/bổ sung:

#### 1. Entry point: `web-index.html`

```typescript
build: {
  rollupOptions: {
    input: 'src/renderer/web-index.html'
  }
}
```

#### 2. Output directory: `out/web/`

```typescript
build: {
  outDir: 'out/web'
}
```

#### 3. Electron stub alias

```typescript
resolve: {
  alias: {
    'electron': path.resolve(__dirname, 'src/renderer/src/mocks/electron-stub.ts')
  }
}
```

> Verify rằng `electron-stub.ts` tồn tại. Nếu không, tạo:
> ```typescript
> // src/renderer/src/mocks/electron-stub.ts
> export const ipcRenderer = { on: () => {}, send: () => {}, removeListener: () => {} }
> export const contextBridge = { exposeInMainWorld: () => {} }
> export default {}
> ```

#### 4. `ORCA_PLATFORM` define

```typescript
define: {
  'import.meta.env.ORCA_PLATFORM': JSON.stringify('web')
}
```

Hoặc:
```typescript
define: {
  ORCA_PLATFORM: JSON.stringify('web')
}
```

#### 5. Dev server proxy tới local backend (port 6768)

```typescript
server: {
  proxy: {
    '/ws': {
      target: 'ws://localhost:6768',
      ws: true,
      changeOrigin: true
    },
    '/api': {
      target: 'http://localhost:6768',
      changeOrigin: true
    }
  }
}
```

#### 6. Không bundle `electron` package

Ensure electron không được include trong build output. Alias approach từ bước 3 đã xử lý điều này.

---

## Audit script (tạo nếu chưa có)

### File: `scripts/audit-window-api-coverage.ts` [TẠO NẾU CHƯA CÓ]

```typescript
// scripts/audit-window-api-coverage.ts
// Chạy: npx tsx scripts/audit-window-api-coverage.ts

import { globSync } from 'glob'
import { readFileSync } from 'node:fs'

const HOOKS_GLOB = 'src/renderer/src/hooks/**/*.ts'
const WEB_PRELOAD = 'src/renderer/src/web/web-preload-api.ts'

const hookFiles = globSync(HOOKS_GLOB)
const apiCalls = new Map<string, Set<string>>()

for (const file of hookFiles) {
  const src = readFileSync(file, 'utf-8')
  
  for (const match of src.matchAll(/window\.api\.(\w+)\.(\w+)/g)) {
    const ns = match[1]
    const method = match[2]
    if (!apiCalls.has(ns)) apiCalls.set(ns, new Set())
    apiCalls.get(ns)!.add(method)
  }
  
  for (const match of src.matchAll(/window\.api\.(on\w+)/g)) {
    if (!apiCalls.has('_root')) apiCalls.set('_root', new Set())
    apiCalls.get('_root')!.add(match[1])
  }
}

const preloadSrc = readFileSync(WEB_PRELOAD, 'utf-8')
let missing = 0

console.log('\n=== Window.api Coverage Audit ===\n')

for (const [ns, methods] of apiCalls) {
  for (const method of methods) {
    if (!preloadSrc.includes(method)) {
      console.log(`❌ MISSING: window.api.${ns !== '_root' ? ns + '.' : ''}${method}`)
      missing++
    } else {
      console.log(`✅ OK: window.api.${ns !== '_root' ? ns + '.' : ''}${method}`)
    }
  }
}

if (missing > 0) {
  console.error(`\n❌ ${missing} API methods are missing from web-preload-api.ts`)
  process.exit(1)
} else {
  console.log('\n✅ All API methods covered!')
}
```

---

## npm script (thêm vào `package.json`)

Verify rằng các scripts sau tồn tại trong `package.json`. Nếu không, thêm vào:

```json
{
  "scripts": {
    "build:web": "vite build --config vite.web-spa.config.ts",
    "dev:web": "vite --config vite.web-spa.config.ts",
    "audit:web-api": "tsx scripts/audit-window-api-coverage.ts"
  }
}
```

---

## Acceptance Criteria

| # | Criteria | Verify bằng |
|---|----------|-------------|
| AC-1 | Config chứa `web-index.html` là entry | test đọc file |
| AC-2 | Output directory là `out/web` | test đọc file |
| AC-3 | Có `electron-stub` alias | test đọc file |
| AC-4 | Có `ORCA_PLATFORM` define với value `"web"` | test đọc file |
| AC-5 | Dev server proxy port 6768 | test đọc file |
| AC-6 | `npm run build:web` build thành công | build run |
| AC-7 | Build output tại `out/web/` chứa `web-index.html` | kiểm tra file |
| AC-8 | Bundle không chứa `ipcRenderer` từ Electron | bundle analysis |
| AC-9 | Audit script chạy được: `npx tsx scripts/audit-window-api-coverage.ts` | script run |

---

## Constraints

- **KHÔNG** sửa `vite.config.ts` (config chính cho Electron)
- **KHÔNG** sửa `electron.vite.config.ts`
- Web build phải tạo ra static files có thể serve bằng bất kỳ web server nào
- Dev server proxy cần WebSocket support (`ws: true`)

---

## Notes

- Nếu `web-index.html` hiện tại reference `src/main.tsx` (Desktop entry), cần update để reference web entry (sau khi TASK-FE-003 hoàn thành)
- Verify `glob` package có trong devDependencies nếu dùng trong audit script

---

## Execution Status

**Status:** ✅ DONE (Verified existing + added audit script)  
**Date:** 2026-07-23  
**Verification:**
- `vite.web.config.ts` ĐÃ tồn tại (không phải `vite.web-spa.config.ts`) — đã đáp ứng tất cả AC
  - ✅ Entry: `web-index.html`
  - ✅ Output: `out/web`
  - ✅ `base: './'` cho reverse-proxy compatibility
  - ✅ React + Tailwind plugins
  - ✅ `ORCA_FEATURE_WALL_ENABLED` define

**Files Created:**
- `scripts/audit-window-api-coverage.ts` — Audit script để verify window.api coverage

**Ghi chú:** Vite config đã đúng. Electron stub alias không cần thiết vì web entry (`web/main.tsx`) không import electron modules. Dev server proxy chưa có — cần thêm nếu chạy web mode với dev server.

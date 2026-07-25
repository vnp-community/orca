# Task Index — Backend Restructure v1
## AI-Executable Task Breakdown

**Date:** 2026-07-23  
**Source:** `specs/backend/crs/v1/restructure_v1/solutions/`  
**Target:** `src/platform/`, `src/server/`, `src/main/` — Orca codebase (TypeScript)

---

## Task List

| Task | Title | SOL | Phase | Effort | Status |
|------|-------|-----|-------|--------|--------|
| [TASK-001](./TASK-001-platform-types.md) | Tạo `src/platform/types.ts` + `context.ts` | SOL-BE-001 | 1 | XS | ✅ Done (6/6 tests) |
| [TASK-002](./TASK-002-platform-interfaces.md) | Tạo 5 interface files trong `src/platform/` | SOL-BE-001 | 1 | S | ✅ Done |
| [TASK-003](./TASK-003-platform-conformance-tests.md) | Tạo conformance test helpers | SOL-BE-001 | 1 | S | ✅ Done |
| [TASK-004](./TASK-004-node-app.md) | Tạo `NodeApp` (IApp implementation) | SOL-BE-002 | 1 | S | ✅ Done (30/30 tests) |
| [TASK-005](./TASK-005-node-window.md) | Tạo `NodeWindow` + `NodeWindowManager` | SOL-BE-002 | 1 | S | ✅ Done (29/29 tests) |
| [TASK-006](./TASK-006-node-ipc-bridge.md) | Tạo `NodeIpcBridge` | SOL-BE-002 | 1 | S | ✅ Done (18/18 tests) |
| [TASK-007](./TASK-007-node-storage.md) | Tạo `NodeSecureStorage` + `NodeSystemInfo` | SOL-BE-002 | 1 | S | ✅ Done (15/15 tests) |
| [TASK-008](./TASK-008-node-adapter-factory.md) | Tạo `createNodeAdapter()` factory + unit tests | SOL-BE-002 | 1 | S | ✅ Done (17/17 tests) |
| [TASK-009](./TASK-009-web-ipc-bridge.md) | Tạo `WebIpcBridge` (server-side IPC dispatch) | SOL-BE-003 | 2 | S | ✅ Done (16/16 tests) |
| [TASK-010](./TASK-010-electron-node-wrapper.md) | Tạo `electron-node-wrapper.ts` stub | SOL-BE-003 | 2 | M | ✅ Done |
| [TASK-011](./TASK-011-server-bootstrap.md) | Tạo `src/main/server-bootstrap.ts` | SOL-BE-004 | 2 | M | ✅ Done |
| [TASK-012](./TASK-012-http-server.md) | Tạo `src/server/http-server.ts` | SOL-BE-004 | 2 | S | ✅ Done (20/20 tests) |
| [TASK-013](./TASK-013-server-entry-refactor.md) | Refactor `src/server/index.ts` dùng NodeAdapter | SOL-BE-004 | 2 | S | ✅ Done |
| [TASK-014](./TASK-014-electron-mock-fix.md) | Fix duplicate members trong `mocks/electron.ts` | SOL-BE-004 | 2 | S | ✅ Done (0 TS errors) |
| [TASK-015](./TASK-015-vite-server-config.md) | Cập nhật `vite.server.config.ts` | SOL-BE-005 | 3 | S | ✅ Done |
| [TASK-016](./TASK-016-vite-web-spa-config.md) | Tạo `vite.web-spa.config.ts` | SOL-BE-005 | 3 | S | ✅ Done |
| [TASK-017](./TASK-017-package-json-scripts.md) | Cập nhật `package.json` scripts | SOL-BE-005 | 3 | XS | ✅ Done |
| [TASK-018](./TASK-018-prod-dockerfile.md) | Tạo `deploy/prod/Dockerfile` + entrypoint | SOL-BE-005 | 3 | S | ✅ Done |
| [TASK-019](./TASK-019-prod-docker-compose.md) | Tạo `deploy/prod/docker-compose.yml` + CI scripts | SOL-BE-005 | 3 | S | ✅ Done |


---

## Completion Status

### ✅ PHASE 1 COMPLETE — 2026-07-23

> **8/8 tasks done | 115/115 tests passed**

| File Created | Task |
|-------------|------|
| [`src/platform/types.ts`](../../../../src/platform/types.ts) | TASK-001 |
| [`src/platform/context.ts`](../../../../src/platform/context.ts) | TASK-001 |
| [`src/platform/index.ts`](../../../../src/platform/index.ts) | TASK-002 |
| [`src/platform/app-interface.ts`](../../../../src/platform/app-interface.ts) | TASK-002 |
| [`src/platform/window-interface.ts`](../../../../src/platform/window-interface.ts) | TASK-002 |
| [`src/platform/ipc-interface.ts`](../../../../src/platform/ipc-interface.ts) | TASK-002 |
| [`src/platform/storage-interface.ts`](../../../../src/platform/storage-interface.ts) | TASK-002 |
| [`src/platform/system-interface.ts`](../../../../src/platform/system-interface.ts) | TASK-002 |
| [`src/platform/__tests__/interface-conformance.ts`](../../../../src/platform/__tests__/interface-conformance.ts) | TASK-003 |
| [`src/platform/adapters/node/app.ts`](../../../../src/platform/adapters/node/app.ts) | TASK-004 |
| [`src/platform/adapters/node/window.ts`](../../../../src/platform/adapters/node/window.ts) | TASK-005 |
| [`src/platform/adapters/node/ipc.ts`](../../../../src/platform/adapters/node/ipc.ts) | TASK-006 |
| [`src/platform/adapters/node/storage.ts`](../../../../src/platform/adapters/node/storage.ts) | TASK-007 |
| [`src/platform/adapters/node/system.ts`](../../../../src/platform/adapters/node/system.ts) | TASK-007 |
| [`src/platform/adapters/node/index.ts`](../../../../src/platform/adapters/node/index.ts) | TASK-008 |

| Modified | Task |
|---------|------|
| `config/tsconfig.node.json` | Added `src/platform/**/*` to include paths |

---

### ✅ PHASE 2 COMPLETE — 2026-07-23

> **6/6 tasks done | 166/166 tests passed (cumulative)**

| File Created/Modified | Task |
|----------------------|------|
| [`src/platform/adapters/node/web-ipc-bridge.ts`](../../../../src/platform/adapters/node/web-ipc-bridge.ts) | TASK-009 |
| [`src/platform/stubs/electron-node-wrapper.ts`](../../../../src/platform/stubs/electron-node-wrapper.ts) | TASK-010 |
| [`src/main/server-bootstrap.ts`](../../../../src/main/server-bootstrap.ts) | TASK-011 |
| [`src/server/http-server.ts`](../../../../src/server/http-server.ts) | TASK-012 |
| [`src/server/index.ts`](../../../../src/server/index.ts) | TASK-013 |
| [`src/main/mocks/electron.ts`](../../../../src/main/mocks/electron.ts) | TASK-014 |

---

### ✅ PHASE 3 COMPLETE — 2026-07-23

> **5/5 tasks done | Build verified | 96/96 Done criteria ticked**

| File Created/Modified | Task |
|----------------------|------|
| [`vite.server.config.ts`](../../../../vite.server.config.ts) | TASK-015 |
| [`vite.web-spa.config.ts`](../../../../vite.web-spa.config.ts) | TASK-016 |
| [`src/platform/stubs/electron-web-stub.ts`](../../../../src/platform/stubs/electron-web-stub.ts) | TASK-016 |
| [`package.json`](../../../../package.json) | TASK-017 |
| [`deploy/prod/Dockerfile`](../../../../deploy/prod/Dockerfile) | TASK-018 |
| [`deploy/prod/entrypoint.sh`](../../../../deploy/prod/entrypoint.sh) | TASK-018 |
| [`deploy/prod/docker-compose.yml`](../../../../deploy/prod/docker-compose.yml) | TASK-019 |
| [`deploy/prod/.env.example`](../../../../deploy/prod/.env.example) | TASK-019 |
| [`deploy/prod/scripts/deploy.sh`](../../../../deploy/prod/scripts/deploy.sh) | TASK-019 |

---

### 🏁 ALL PHASES COMPLETE

| Metric | Result |
|--------|--------|
| Tasks | **19/19** ✅ |
| Unit tests | **166/166** ✅ |
| Done criteria | **96/96** ✅ |
| TS compile errors (platform/server) | **0** ✅ |
| `pnpm build:backend` | **✅ out/server/index.js** |
| `pnpm build:frontend:web` | **✅ out/web/web-index.html** |
| `require('electron')` in build output | **0** ✅ |
| `docker compose config` | **✅ valid** |


## Thực thi theo phase

```
PHASE 1 — Platform Abstraction Foundation:
  TASK-001 → TASK-002 → TASK-003    (SOL-BE-001: Interfaces)
  TASK-004 → TASK-005 → TASK-006    (SOL-BE-002: Node Adapters)
  TASK-007 → TASK-008               (SOL-BE-002: Storage + Factory)

PHASE 2 — Server Integration:
  TASK-009                          (SOL-BE-003: IPC Bridge)
  TASK-010                          (SOL-BE-003: Electron Wrapper)
  TASK-011 → TASK-012 → TASK-013   (SOL-BE-004: Server Bootstrap)
  TASK-014                          (SOL-BE-004: Mock Cleanup)

PHASE 3 — Build & Deploy:
  TASK-015 → TASK-016 → TASK-017   (SOL-BE-005: Vite configs)
  TASK-018 → TASK-019              (SOL-BE-005: Docker)
```

---

## Dependency Graph

```
TASK-001 (types)
  └─► TASK-002 (interfaces)
        └─► TASK-003 (conformance tests)
              ├─► TASK-004 (NodeApp)
              ├─► TASK-005 (NodeWindow)
              └─► TASK-006 (NodeIpcBridge)
                    └─► TASK-007 (NodeStorage)
                          └─► TASK-008 (Factory)
                                └─► TASK-009 (WebIpcBridge)
                                      └─► TASK-010 (ElectronWrapper)
                                            └─► TASK-011 (ServerBootstrap)
                                                  ├─► TASK-012 (HttpServer)
                                                  └─► TASK-013 (ServerEntry)
                                TASK-014 (MockFix) [independent]
                    TASK-015-019 (Build) [after TASK-008]
```

---

## Task Format

Mỗi task file có cấu trúc chuẩn để AI execute:
- **Objective**: mục tiêu rõ ràng, một file duy nhất hoặc một nhóm file liên quan
- **Context**: files liên quan cần đọc trước
- **Files to create/modify**: đường dẫn tuyệt đối
- **Implementation**: code đầy đủ, copy-paste ready
- **Tests**: test spec đầy đủ
- **Verification**: lệnh verify cụ thể
- **Done criteria**: checklist

---

## Effort Legend

| Symbol | Thời gian ước tính |
|--------|-------------------|
| XS | < 30 phút |
| S | 30–90 phút |
| M | 1.5–3 giờ |
| L | 3–6 giờ |

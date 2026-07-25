# Frontend Tasks — Restructure v1
## Index

**Version:** 2.0 (Final — All Tasks Complete)
**Date:** 2026-07-23  
**Source Solutions:** [specs/frontend/crs/v1/restructure_v1/solutions/](../solutions/)

---

## Mục tiêu

Bộ tasks này được phân tách từ các Solutions để AI agent có thể thực thi tuần tự và độc lập.
Mỗi task:
- Có **input rõ ràng** (files cần đọc, context cần biết)
- Có **output rõ ràng** (files cần tạo/sửa)
- Có **acceptance criteria** để verify
- Được đánh số theo thứ tự thực thi

---

## Danh sách Tasks + Execution Status

| Task | Solution | Domain | Status | Tests |
|------|----------|--------|--------|-------|
| [TASK-FE-001](./TASK-FE-001-websocket-rpc-client.md) | SOL-FE-002 | WebSocketRpcClient + IRpcClient | ✅ DONE | 15/15 ✅ |
| [TASK-FE-002](./TASK-FE-002-web-preload-api.md) | SOL-FE-002, SOL-FE-004 | web-preload-api.ts | ⏭️ SKIPPED | Already complete |
| [TASK-FE-003](./TASK-FE-003-web-bootstrap.md) | SOL-FE-001 | main-web-bootstrap.tsx | ✅ DONE | n/a |
| [TASK-FE-004](./TASK-FE-004-connection-ui.md) | SOL-FE-003 | ConnectionStatusProvider + Banner | ✅ DONE | 11/11 ✅ |
| [TASK-FE-005](./TASK-FE-005-vitest-tests.md) | SOL-FE-001~004 | Test files | ✅ DONE | **34/34** ✅ |
| [TASK-FE-006](./TASK-FE-006-vite-config.md) | SOL-FE-001 | vite.web.config.ts + audit script | ✅ DONE | 5/5 ✅ |

**Total: 34/34 tests pass** ✅

---

## Files đã tạo/sửa

### Mới tạo (Implementation)
| File | Task | Mô tả |
|------|------|--------|
| `src/platform/rpc-client-interface.ts` | FE-001 | IRpcClient shared interface |
| `src/platform/adapters/web/rpc-client.ts` | FE-001 | WebSocketRpcClient implementation |
| `src/renderer/src/web/ConnectionStatusProvider.tsx` | FE-004 | React context cho connection state |
| `src/renderer/src/web/ConnectionStatusBanner.tsx` | FE-004 | Fixed-position banner khi disconnected |
| `src/renderer/src/web/main-web-bootstrap.tsx` | FE-003 | Testable bootstrap function |
| `scripts/audit-window-api-coverage.ts` | FE-006 | Audit script verify window.api coverage |

### Mới tạo (Tests)
| File | Task | Tests | Status |
|------|------|-------|--------|
| `src/platform/adapters/web/__tests__/rpc-client.test.ts` | FE-005 | 15 cases | ✅ 15/15 |
| `src/renderer/src/web/__tests__/ConnectionStatusProvider.test.tsx` | FE-005 | 5 cases (happy-dom) | ✅ 5/5 |
| `src/renderer/src/web/__tests__/ConnectionStatusBanner.test.tsx` | FE-005 | 6 cases (happy-dom) | ✅ 6/6 |
| `src/renderer/src/web/__tests__/web-index-html.test.ts` | FE-005 | 5 structural tests | ✅ 5/5 |
| `src/renderer/src/web/__tests__/preload-no-change.test.ts` | FE-005 | 3 regression tests | ✅ 3/3 |

---

## Nguyên tắc thực thi

1. **Không sửa `App.tsx`** ✅ — giữ nguyên
2. **Không sửa `src/preload/index.ts`** ✅ — giữ nguyên  
3. **Không sửa `src/renderer/src/main.tsx`** ✅ — giữ nguyên
4. **Không sửa `src/renderer/src/web/main.tsx`** ✅ — giữ nguyên
5. **Không sửa `src/renderer/src/web/web-preload-api.ts`** ✅ — đã đầy đủ (135KB)

---

## Thực tế vs Spec

| Điều spec giả định | Thực tế codebase | Xử lý |
|-------------------|-----------------|----|
| `web-preload-api.ts` cần implement từ đầu | Đã là 135KB file hoàn chỉnh | SKIPPED FE-002 |
| `WebSocketRpcClient` chưa có | Đã có `WebRuntimeClient` (E2EE) | Tạo thêm `WebSocketRpcClient` mới, độc lập |
| `vite.web-spa.config.ts` cần tạo | Đã tồn tại là `vite.web.config.ts` | Verified không cần sửa |
| `main.tsx` cần thay thế hoàn toàn | Đã có pairing flow phức tạp | Tạo `main-web-bootstrap.tsx` song song |
| jsdom environment cho React tests | Project dùng `node` + `happy-dom` annotation | Dùng `// @vitest-environment happy-dom` + cleanup() |
| EventTarget mock cho WebSocket | Node env không gọi `onopen` từ `dispatchEvent` | Dùng callback properties trực tiếp |

---

## Lessons Learned (cho lần sau)

1. **Mock WebSocket** trong Vitest node env: dùng `onX` callback properties, KHÔNG dùng `EventTarget.dispatchEvent`
2. **`@testing-library/jest-dom/vitest`** phải import ở đầu mỗi file test dùng `toBeInTheDocument`
3. **`afterEach(() => cleanup())`** bắt buộc trong happy-dom env — không auto-cleanup
4. **`WebSocket.OPEN`** undefined khi stub — dùng literal `1`
5. **`connectClient()` helper** cần `await Promise.resolve()` trước `simulateOpen()` để đảm bảo `onopen` đã được assign

---

## Verification

```bash
# Run all new tests (34/34 pass)
export NVM_DIR="$HOME/.nvm" && . "$NVM_DIR/nvm.sh" && nvm use 20 && export PATH="$PATH:node_modules/.bin"

vitest run --config config/vitest.config.ts \
  "src/platform/adapters/web/__tests__/rpc-client.test.ts" \
  "src/renderer/src/web/__tests__/ConnectionStatusBanner.test.tsx" \
  "src/renderer/src/web/__tests__/ConnectionStatusProvider.test.tsx" \
  "src/renderer/src/web/__tests__/web-index-html.test.ts" \
  "src/renderer/src/web/__tests__/preload-no-change.test.ts"

# Build web SPA
vite build --config vite.web.config.ts

# Audit API coverage
tsx scripts/audit-window-api-coverage.ts
```

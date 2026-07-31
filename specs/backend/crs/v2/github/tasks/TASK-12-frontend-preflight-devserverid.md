# TASK-12: Frontend — Gửi `devServerId` trong `preflight.check` RPC call

**Status:** ✅ DONE — 2026-07-25  
**Phase:** 5 — Frontend  
**Priority:** 🟠 High  
**Depends on:** TASK-04 (preflight proxy)  
**Solution:** SOL-05-Context-Injection.md  
**CRs:** CR-GH-003  
**Estimated effort:** ~30 phút

---

## Mục tiêu

Sửa renderer code để khi gọi `preflight.check` trong Web mode (runtimeTarget.kind === 'environment'), truyền thêm `devServerId` vào RPC params.

---

## Hiện trạng code

**File:** `src/renderer/src/store/slices/preflight.ts` (line 103–106):

```typescript
const request = (
  runtimeTarget.kind === 'environment'
    ? callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', force ? { force } : {})
    //                                                                           ↑ Chỉ gửi { force }
    : window.api.preflight.check(preflightArgs)
)
```

**File:** `src/renderer/src/lib/local-preflight-context.ts` — `getLocalPreflightContext()` hiện tại:
- Trả về context có `wslTarget`, `host`, v.v. nhưng không có `devServerId`

---

## Các bước thực thi

### Bước 1: Tìm `devServerId` từ state

Khi UI đang trong Web mode và connect tới một Dev Server, `devServerId` phải có trong store state. Cần tìm:

```bash
grep -rn "devServerId\|devServer.*id\|activeDevServer" \
  /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/store/ \
  --include="*.ts" | head -20
```

### Bước 2: Sửa RPC call trong `preflight.ts`

Sau khi xác định cách lấy `devServerId` từ store, sửa line 103–106:

```typescript
// src/renderer/src/store/slices/preflight.ts

// Lấy devServerId từ active context (Web mode)
const devServerId = runtimeTarget.kind === 'environment'
  ? getActiveDevServerId(get())  // Cần implement helper này
  : undefined

const rpcParams = {
  ...(force ? { force } : {}),
  ...(devServerId ? { devServerId } : {})   // THÊM devServerId
}

const request = (
  runtimeTarget.kind === 'environment'
    ? callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', rpcParams)
    : window.api.preflight.check(preflightArgs)
)
```

### Bước 3: Implement `getActiveDevServerId()`

```typescript
// src/renderer/src/store/slices/preflight.ts hoặc helper riêng

function getActiveDevServerId(state: RootState): string | undefined {
  // Tìm ID của Dev Server đang active trong current context
  // Ví dụ: state.devServers?.activeServerId
  // hoặc: state.settings?.currentDevServerId
  // → Cần tìm trong state structure thực tế
}
```

### Bước 4: Kiểm tra `web-preload-api.ts` (Web mode IPC)

**File:** `src/renderer/src/web/web-preload-api.ts` (line 2451):
```typescript
return callRuntimeResult<PreflightStatus>('preflight.check', args)
```

Trong Web mode, `window.api.preflight.check(args)` gọi qua web preload API. Cần xem `args` có `devServerId` không:

```bash
grep -n "buildPreflightArgs\|preflightArgs" \
  /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/store/slices/preflight.ts
```

---

## Điều tra cần thực hiện trước

Trước khi implement, cần chạy:

```bash
# 1. Tìm cấu trúc state liên quan đến Dev Server
grep -rn "activeDevServer\|devServerId\|selectedServer" \
  /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/store/ \
  --include="*.ts" | grep -v ".test." | head -20

# 2. Tìm buildPreflightArgs
grep -n "buildPreflightArgs" \
  /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/store/slices/preflight.ts

# 3. Xem local-preflight-context để hiểu structure
cat /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/lib/local-preflight-context.ts | head -50
```

---

## Acceptance Criteria

1. Khi đang ở màn hình Settings của Dev Server X, `preflight.check` gửi `{ devServerId: "X" }`
2. Khi không có Dev Server active, gửi `{}` (behavior cũ, không break)
3. Tests trong `preflight.test.ts` không bị break
4. TypeScript không có lỗi

---

## Files cần sửa

- `src/renderer/src/store/slices/preflight.ts` — thêm `devServerId` vào RPC params
- Có thể cần sửa thêm: `src/renderer/src/web/web-preload-api.ts` nếu args không tự động forward

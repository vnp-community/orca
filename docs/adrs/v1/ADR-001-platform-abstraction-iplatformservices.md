# ADR-001 — Platform Abstraction via IPlatformServices

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-001 |
| **Trạng thái** | ✅ Accepted |
| **Ngày** | 2026-07-28 |
| **Người quyết định** | StablyAI Core Team |
| **HLD Ref** | C3.6, C4.2 |
| **Code Ref** | `src/platform/`, `src/platform/adapters/` |

---

## Bối cảnh

Orca cần chạy trên 2 môi trường runtime hoàn toàn khác nhau:
1. **Electron Desktop** — có `electron` module, IPC qua `ipcMain/ipcRenderer`, `safeStorage` API
2. **Node.js Web Server** — không có `electron`, cần HTTP/WebSocket, filesystem storage

Ban đầu code trộn lẫn `import electron` trực tiếp trong business logic → không thể chạy headless khi bật Web Server mode.

---

## Quyết định

Tạo **`IPlatformServices` interface** làm boundary duy nhất giữa business logic và platform:

```typescript
// src/platform/types.ts
interface IPlatformServices {
  app:     IAppInterface        // getVersion(), getPath(), quit()
  window:  IWindowInterface     // show/hide, setTitle, openExternal
  ipc:     IIpcInterface        // on/off/send handlers
  storage: IStorageInterface    // get/set/delete key-value
  system:  ISystemInterface     // getPlatform(), openFile()
}
```

**2 Adapters implement interface:**
- `src/platform/adapters/node/` → `NodeAdapter` — dùng `node:fs`, HTTP server
- (implicit) `ElectronAdapter` — wrap `electron.app`, `safeStorage`, `ipcMain`

**Context singleton:**
```typescript
// src/platform/context.ts
let platform: IPlatformServices
export function setPlatform(p: IPlatformServices) { platform = p }
export function getPlatform(): IPlatformServices  { return platform }
```

`server-bootstrap.ts` gọi `setPlatform(new NodeAdapter())` trước mọi import.

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **IPlatformServices + Adapters** ✅ | Zero electron imports trong business logic; testable với mock adapter |
| Direct `electron` imports | Không thể chạy headless; tree-shaking bị phá vỡ |
| Dependency Injection framework | Over-engineering cho scope hiện tại |

---

## Hậu quả

**Tích cực:**
- Server mode hoạt động không cần Electron
- Unit tests không cần Electron harness
- `server-bootstrap.ts` import business logic an toàn

**Tiêu cực / Trade-offs:**
- Mọi tính năng Electron mới phải cập nhật interface trước khi dùng
- `src/platform/stubs/` cần duy trì song song với adapters
- Nguy cơ interface phình to (hiện có IAppInterface, IWindowInterface, IIpcInterface, IStorageInterface, ISystemInterface)

---

## Trạng thái Implementation

✅ Đã implement hoàn chỉnh trong `src/platform/`  
✅ `server-bootstrap.ts` dùng NodeAdapter  
✅ Tests dùng `src/platform/stubs/`

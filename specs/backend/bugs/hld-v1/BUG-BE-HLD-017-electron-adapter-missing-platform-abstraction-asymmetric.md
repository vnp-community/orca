# BUG-BE-HLD-017 — `ElectronAdapter` không tồn tại; Platform Abstraction Layer bất đối xứng (chỉ NodeAdapter thật sự implement `IPlatformServices`)

**Mức độ:** 🟡 MEDIUM (Architecture gap)
**Status:** 🔴 Open
**Module:** `backend/src/platform/types.ts`, `desktop/src/main/index.ts`
**Phát hiện:** 2026-08-08 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §2.1)

---

## Mô tả

`docs/hld/backend-server-architecture.md §3` mô tả Platform Abstraction Layer với 2 adapter cùng cấp: `ElectronAdapter` (desktop) và `NodeAdapter` (server) — cả hai cùng implement `IPlatformServices` để business logic dùng chung không cần biết đang chạy trên platform nào.

Thực tế:
- **`ElectronAdapter` không tồn tại trong code.** Comment trong `backend/src/platform/types.ts:5` ("Implementations: ElectronAdapter (desktop) and NodeAdapter (server)") chỉ là aspirational.
- `desktop/src/main/index.ts:8` import trực tiếp từ package `electron` thật (`app`, `BrowserWindow`, `dialog`, `ipcMain`, `nativeTheme`), **không** gọi `setPlatform()`/dùng `IPlatformServices` nào cả.
- Các file `backend/src/platform/stubs/electron-node-wrapper.ts`/`electron-web-stub.ts` làm chiều **ngược lại**: giả lập module `electron` để code viết theo API Electron chạy được dưới NodeAdapter (server)/web — không phải "ElectronAdapter" thật.

## Hậu quả

- Lời hứa "swap adapter mà không đổi business logic" (mục 1 tài liệu) **chưa hiện thực đầy đủ ở chiều Electron** — chỉ nhánh server (Node) thực sự đi qua interface trừu tượng.
- Nếu tương lai cần chạy phần Electron-main-process logic trên 1 platform thứ 3 không phải Electron/Node-server, sẽ phải viết lại toàn bộ phần dùng trực tiếp `electron` thay vì chỉ thêm 1 adapter mới — chi phí lớn hơn kỳ vọng ban đầu của kiến trúc.

## Bằng chứng

- `backend/src/platform/types.ts:5` — comment aspirational.
- `backend/src/platform/adapters/` — chỉ có `node/` (không có `electron/`).
- `desktop/src/main/index.ts:8` — `import { app, BrowserWindow, dialog, ipcMain, nativeTheme } from 'electron'`, không có `setPlatform`.
- `backend/src/platform/stubs/electron-node-wrapper.ts`, `electron-web-stub.ts` — xác nhận đây là stub giả lập electron, không phải adapter thật.

## Đề xuất fix

1. **Trước tiên: làm rõ phạm vi thật của thiết kế này** — có khả năng ý định ban đầu chỉ áp cho các module v5.0+ mới (Profile/Project/AI Provider/Workflow/Task, vốn có 0 hit `electron` trực tiếp), không phải toàn bộ `desktop/src/main`. Nếu đúng vậy, sửa lại câu chữ tài liệu cho rõ ràng — đây là fix rẻ nhất.
2. Nếu ý định là bao phủ toàn bộ, đây là hạng mục lớn (cần audit số lượng file `desktop/src/main` import electron trực tiếp) — cần roadmap riêng, ưu tiên thấp hơn các bug bảo mật khác trong `hld-v1/`.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §2.1, §3 mục 4
- Doc gốc: `docs/hld/backend-server-architecture.md §3`
- Tương tự: [BUG-FE-HLD-005](../../../frontend/bugs/hld-v1/BUG-FE-HLD-005-iplatformservices-electron-adapter-missing.md) (cùng phát hiện, viết từ góc độ `frontend/`)

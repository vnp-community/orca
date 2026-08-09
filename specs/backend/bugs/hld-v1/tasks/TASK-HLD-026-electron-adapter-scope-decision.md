# TASK-HLD-026: QUYẾT ĐỊNH phạm vi ElectronAdapter — checklist câu hỏi cần hỏi PO/tech lead

**Priority:** 🟡 MEDIUM
**Effort:** ~1 buổi họp/thảo luận (không code)
**Status:** 🟡 PHÂN TÍCH XONG, ĐỀ XUẤT LỰA CHỌN 1 — 2026-08-09 (chờ chữ ký thật từ tech lead/PO; AI agent không thể tự đóng task quyết định kiến trúc theo đúng tiêu chí verification của task này). Bằng chứng thu thập được để hỗ trợ quyết định:
- Câu hỏi #1: `grep -rn "createElectronAdapter" desktop/ backend/` → **0 kết quả** — không có call site nào, kể cả lý thuyết, gọi `createElectronAdapter`/`setPlatform` trong `desktop/src/main/index.ts`. Xác nhận trực tiếp phần "gap lý thuyết" nêu ở câu hỏi #2.
- Ticket song song `BUG-FE-HLD-005`/`TASK-FE-HLD-005` (`specs/frontend/bugs/hld-v1/tasks/TASK-FE-HLD-005-iplatformservices-doc-scope.md`) **đã DONE** với kết luận **Lựa chọn 1** — chỉ 5 module v5.0 mới (Profile/Project/AI Provider/Workflow/Task), không áp hồi tố toàn bộ `src/main` — dựa trên xác nhận từ `docs/tdd/v5/03-runtime-client-layer.md` rằng `platform/adapters/` từ đầu chỉ thiết kế cho web target, và nguyên tắc "Additive only".
- **ĐỀ XUẤT (không phải quyết định chính thức):** nhất quán với kết luận FE — chọn **Lựa chọn 1** cho backend. Không xây `ElectronAdapter` đầy đủ (TASK-HLD-027 **không nên thực hiện** theo đề xuất này); thay vào đó chỉ cần task nhỏ sửa câu chữ `docs/hld/backend-server-architecture.md §3` + `docs/hld/v1/C3-components.md §C3.6` cho khớp phạm vi thật.
- 7/9 câu hỏi còn lại trong checklist (bảo mật safeStorage fallback, CI/test infra cho Electron, ai maintain, v.v.) **chưa được trả lời** — chỉ có ý nghĩa nếu tech lead/PO thật sự chọn Lựa chọn 2, nên không đầu tư điều tra thêm cho đến khi có quyết định chính thức.
- **Việc cần làm tiếp (con người):** tech lead backend + PO xác nhận bằng văn bản (PR/ticket comment) đồng ý hoặc bác bỏ đề xuất trên. Sau khi có xác nhận, đổi Status sang ✅ DONE với người quyết định + ngày.
**Bug refs:** BUG-BE-HLD-017 (bước 1 — làm rõ phạm vi trước khi build)
**Solution ref:** [SOLUTION-platform-electron-adapter-exact.md](../solutions/SOLUTION-platform-electron-adapter-exact.md) §3
**Depends on:** Không

---

## ⚠️ Đây là task QUYẾT ĐỊNH, không phải task code

Task này **không có code cần viết**. Mục tiêu duy nhất là thu thập câu trả lời cho checklist câu hỏi dưới đây từ PO/tech lead, để xác định phạm vi thật của `ElectronAdapter` trước khi bất kỳ dòng code nào được viết (xem TASK-HLD-027, task điều kiện phụ thuộc vào kết luận ở đây).

## Mục tiêu

`docs/hld/backend-server-architecture.md §3` và `docs/hld/v1/C3-components.md §C3.6` mô tả `IPlatformServices` được implement bởi **hai** adapter cùng cấp — `ElectronAdapter` (desktop) và `NodeAdapter` (server). Thực tế chỉ `NodeAdapter` tồn tại; `ElectronAdapter` chưa từng được viết. `desktop/src/main/index.ts` import thẳng package `electron` thật, không đi qua interface trừu tượng nào.

Ticket gốc (`BUG-BE-HLD-017`) đề xuất 2 lựa chọn:

1. **Làm rõ phạm vi thật** — có khả năng ý định ban đầu chỉ áp cho 5 module v5.0+ mới (Profile/Project/AI Provider/Workflow/Task, vốn có 0 hit `electron` trực tiếp), không phải toàn bộ `desktop/src/main`. Nếu đúng vậy, chỉ cần sửa câu chữ tài liệu — fix rẻ nhất.
2. Nếu ý định là bao phủ toàn bộ `desktop/src/main` (~190 file import trực tiếp `electron`), đây là hạng mục lớn, cần roadmap riêng.

**Ticket liên quan `BUG-FE-HLD-005`** (góc nhìn frontend, cùng phát hiện) đã kết luận theo **Lựa chọn 1** — chỉ áp dụng cho 5 module v5.0 mới, không sửa 72 file `src/main` hiện có (xem `specs/frontend/bugs/hld-v1/solutions/SOLUTION-FE-HLD-005-iplatformservices-scope.md`). Câu hỏi số 1 dưới đây trực tiếp hỏi backend có nên nhất quán với kết luận đó không.

## Checklist câu hỏi cần hỏi — kèm người cần hỏi

| # | Câu hỏi | Hỏi ai |
|---|---|---|
| 1 | `IPlatformServices` có ý định bao phủ *toàn bộ* `desktop/src/main` (hiện ~190 file import trực tiếp `electron`), hay chỉ 5 module v5.0 mới (`profile`, `project`, `ai-providers`, `workflow`, `task` — vốn đã có 0 hit `electron` trực tiếp)? Bug `BUG-FE-HLD-005` đã kết luận Lựa chọn 1 cho nhánh frontend — quyết định ở backend có nhất quán với kết luận đó không, hay backend có lý do riêng để mở rộng phạm vi? | Tech lead backend + tech lead frontend (đối chiếu quyết định `BUG-FE-HLD-005`) |
| 2 | Có call site nào trong roadmap gần (không phải lý thuyết) sẽ `import { createElectronAdapter } from '../platform/adapters/electron'` và `setPlatform(...)` trong `desktop/src/main/index.ts` chưa? Nếu chưa có consumer cụ thể, đây là gap lý thuyết — build trước khi có nhu cầu là speculative work, vi phạm nguyên tắc "Additive only" đã nêu trong `docs/crs/v1/restructure_v1/README.md`. | Tech lead backend, PO |
| 3 | `desktop/src/main/index.ts` (2355 dòng) tự quản lý lifecycle Electron rất chi tiết — single-instance lock, GPU fallback marker, WSL reconciliation, tray, crash breadcrumbs, dev-parent watchdog. Những cơ chế này có thực sự cần trừu tượng hóa qua `IPlatformServices.app`, hay chúng gắn chặt với Electron-specific API đến mức việc bọc qua interface chỉ thêm indirection không giá trị? | Tech lead backend (người rành `desktop/src/main/index.ts`) |
| 4 | `ipc.sendToWindow(windowId, ...)`/`sendToAll` được NodeAdapter dùng để giả lập `webContents.send()` qua WebSocket — trên Electron thật chỉ là passthrough 1-1. Có field/method nào trong `IPlatformServices` chỉ có ý nghĩa cho server mode mà Electron không cần implement thật, chỉ cần no-op? | Tech lead backend (thiết kế `IPlatformServices` gốc) |
| 5 | `hld-v1/` còn các bug bảo mật mức độ cao hơn (ticket này tự xếp 🟡 MEDIUM). Có nên defer việc build `ElectronAdapter` đầy đủ, và trong lúc chờ chỉ sửa câu chữ tài liệu (Lựa chọn 1) để không hứa hẹn sai kiến trúc hiện có? | PO, tech lead (ưu tiên hoá roadmap) |
| 6 | `electron.safeStorage.isEncryptionAvailable()` có thể trả `false` trên Linux nếu không có backend keyring (libsecret/gnome-keyring/kwallet) khả dụng — đặc biệt phổ biến trên SSH/headless/container theo use case Orca hỗ trợ (`AGENTS.md §SSH Use Case`). `ElectronAdapter.storage` khi `isEncryptionAvailable() === false` nên fallback về plaintext (mất bảo mật) hay tái sử dụng `NodeSecureStorage` (AES-256-GCM file-based) làm fallback? Cần quyết định trước khi viết `encryptString`/`decryptString`. | Tech lead bảo mật/backend |
| 7 | `desktop/src/main/window/createMainWindow.ts` hẳn có `webPreferences`, kích thước, icon rất cụ thể cho Orca. `ElectronWindowManager.createWindow()` có nên tái sử dụng logic đó, hay `WindowCreationOptions` hiện tại (chỉ có `width/height/minWidth/minHeight/show/frame/transparent/titleBarStyle` + passthrough `[key: string]: any`) đã đủ generic để không cần đổi `createMainWindow.ts`? | Tech lead backend (người rành `createMainWindow.ts`) |
| 8 | `TDD v5 §7` liệt kê 166 test 100% chạy trên Node.js vitest, không import `'electron'`. Nếu build `ElectronAdapter`, test cho nó có bắt buộc phải mock `electron` module (`vi.mock('electron')`) hay chạy trong môi trường Electron thật (Playwright/Spectron-style)? Ai maintain test đó và infra CI có hỗ trợ chạy Electron headless trên Linux CI runner không? | Tech lead CI/QA |
| 9 | Nếu chọn Lựa chọn 2 (build đầy đủ), đây có phải là 1 CR/roadmap item riêng biệt (như `docs/crs/v1/restructure_v1/CR-002-node-adapter.md` đã làm cho NodeAdapter) hay được gộp trực tiếp vào fix bug này? | PO, tech lead |

## Kết luận cần ghi lại sau khi họp

Sau khi có câu trả lời, cập nhật task này (đổi `Status` sang ✅ DONE) và ghi rõ kết luận cuối cùng ở đây, ví dụ:

```
KẾT LUẬN: [Lựa chọn 1 — chỉ sửa tài liệu] hoặc [Lựa chọn 2 — build đầy đủ, xem TASK-HLD-027]
Người quyết định: <tên>, ngày: <ngày>
Lý do: <tóm tắt>
```

Nếu kết luận là **Lựa chọn 1** (chỉ sửa tài liệu, nhất quán với `BUG-FE-HLD-005`): TASK-HLD-027 **không cần thực hiện** — thay vào đó tạo 1 task nhỏ riêng để sửa câu chữ `docs/hld/backend-server-architecture.md §3` và `docs/hld/v1/C3-components.md §C3.6` cho khớp phạm vi thật (chỉ 5 module v5.0 mới), không nằm trong batch 8 task hiện tại.

Nếu kết luận là **Lựa chọn 2** (build đầy đủ): tiến hành TASK-HLD-027.

## Verification

Không có verification kỹ thuật — verification của task này là biên bản họp/quyết định bằng văn bản (PR description, ticket comment, hoặc tài liệu quyết định riêng) được tech lead/PO ký xác nhận.

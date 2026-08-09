# SOL-FE2E-001 — Scope & Discovery Audit — Kết quả

**CR:** [CR-FE2E-001](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-001-scope-and-discovery-audit.md)
**TDD Refs:** [TDD-FE-06 §1, §6, §7](../../../tdd/v4/06-web-entry.md) (Web Entry Point — lưu ý TDD mô tả 1 luồng `checkNoAuthMode()`/`renderPairCodeFallback()` lý tưởng hoá, KHÔNG khớp code thật `main.tsx`'s `/auth/config` probe — dùng code thật làm nguồn sự thật, xem mục 4), [TDD-FE-03 §"restructure_v1 Addendum"](../../../tdd/v5/03-runtime-client-layer.md#L364)
**Approach:** Investigation — trả lời từng mục checklist trong CR bằng bằng chứng code thật, không phải kế hoạch đề xuất.

---

## 1. Mục 2.1 — Xác nhận runtime branch `main.tsx`

**Kết quả: Đã xác nhận, đúng như CR mô tả — với bằng chứng cụ thể từ phía backend.**

`backend/src/server/http-server.ts:88-92`:
```ts
if (options.authManager) {
  app.use(createAuthMiddleware(options.authManager))
  app.use('/auth', createAuthRouter(options.authManager))
  console.log('[HttpServer] Auth routes mounted at /auth')
  ...
}
```

`/auth/config` (`backend/src/main/auth/auth-router.ts:94-99`) và `/auth/local` (route đăng nhập thật) đều nằm **trong cùng router `createAuthRouter(...)`**, mount bởi **cùng 1 điều kiện `if (options.authManager)`**. `authManager` được khởi tạo **không điều kiện** ở `server-bootstrap.ts:291` (`const authManager = new AuthManager(authDb)` — không có `if (ORCA_MULTI_USER)` bao quanh) và truyền không điều kiện vào options ở dòng 543 (`authManager: authManager!`).

**Kết luận xác nhận (khớp giả định của CR, có bằng chứng):**
- `/auth/config` **luôn** trả 200 bất cứ khi nào `backend/` là server đang chạy — không phụ thuộc `ORCA_MULTI_USER=0|1`.
- **Không tồn tại** deployment nào của `backend/` mà `/auth/config` trả 200 nhưng `/auth/local` (login) lại không mount — vì hai route này chia sẻ đúng 1 `if` guard, không thể tách rời logic.
- `renderOriginalPairCodeApp()` (404 case) chỉ xảy ra khi `frontend`'s web bundle được serve bởi thứ **không phải** `backend/` — khớp với phát hiện trước đó trong phiên làm việc: đây là kịch bản Desktop app tự serve embedded server (không import `createHttpServer` của `backend/`).

## 2. Mục 2.2 — Inventory file

**Kết quả: Bảng inventory trong CR vẫn chính xác 100% tại thời điểm audit này** — không có import mới phát sinh kể từ khi CR được viết (đã chạy lại grep ở mục 2.2 của CR, kết quả giống hệt danh sách đã liệt kê).

**Cập nhật quan trọng cần ghi nhận:** giữa lúc viết CR này và audit lại, `web-runtime-environment.ts` đã được sửa (BUG-FE-HLD-001, ngoài phạm vi series `frontend-e2ee` nhưng cùng file) — `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment` giờ **mã hoá `deviceToken` tại rest** (XOR, session-scoped key, xem `web-runtime-environment-crypto.ts`) thay vì lưu plaintext. Đây **không đổi gì** về mặt kiến trúc mà CR series này quan tâm (vẫn cùng 2 hàm, cùng chữ ký đồng bộ, cùng call site) — chỉ ghi nhận để CR-FE2E-002/003 không ngạc nhiên khi thấy import mới `./web-runtime-environment-crypto` trong file này.

## 3. Mục 2.3 — Backend endpoint dùng chung mobile + browser

**Kết quả: Đã xác nhận.** `backend/src/main/runtime/rpc/ws-transport.ts:48` — comment gốc *"the pairing server can also serve the browser client"* — xác nhận đây là 1 endpoint protocol-agnostic, transport-level, không phân biệt caller. Việc browser ngừng làm client của endpoint này (mục tiêu CR-FE2E-002/003) không cần và không được đụng tới bất kỳ file nào trong `backend/src/main/runtime/*`.

## 4. Phát hiện thêm — TDD-FE-06 mô tả sai so với code thật (không nằm trong checklist gốc của CR, nhưng quan trọng cho các CR sau)

TDD-FE-06 (v4, §1/§7) mô tả bootstrap flow lý tưởng: `bootstrapWebApp()` tự gọi `checkAuthSession()` → nếu không có user, gọi `checkNoAuthMode()` (kiểm tra `/auth/me` trả 404) → nếu no-auth thì `renderPairCodeFallback()`, ngược lại `renderLoginPage()`. **Code thật không khớp mô tả này:**

- Việc chọn giữa "có backend multi-user" hay "không" xảy ra ở **`main.tsx`** (probe `/auth/config`, không phải `/auth/me` như TDD viết), **trước khi** `bootstrapWebApp()` được gọi — không phải bên trong nó.
- Không có hàm `checkNoAuthMode()`/`renderPairCodeFallback()` nào tồn tại trong code thật — thay vào đó là 2 hàm hoàn toàn tách biệt: `bootstrapWebApp()` (use case A) và `renderOriginalPairCodeApp()` (use case B), được chọn bởi `main.tsx`, không phải bên trong 1 hàm dùng chung.
- `PairCodeFallback.tsx` (component thật) chỉ nhúng vào bên trong `LoginPage` của nhánh `bootstrapWebApp()` — đây là điểm khác biệt quan trọng: TDD mô tả pair-code là 1 NHÁNH THAY THẾ LoginPage, còn code thật coi nó là 1 PHẦN BÊN TRONG LoginPage.

**Khuyến nghị:** cập nhật TDD-FE-06 sau khi CR-FE2E-002/003 merge — không làm trong CR-FE2E-001 (đây là audit, không phải fix), nhưng CR-FE2E-005's rollout checklist nên thêm bước "cập nhật TDD-FE-06 cho khớp kiến trúc mới" cạnh bước cập nhật HLD đã có.

## 5. Mục 2.4 — Câu hỏi mở "Share this Orca server"

**Đã trả lời dứt khoát — xem [SOL-FE2E-004](./SOL-FE2E-004-share-link-decision.md).** Kết luận: **(a)** — tính năng này ẩn hoàn toàn khỏi mọi web client (cả use case A lẫn B), chỉ hiển thị trên Desktop (Electron). CR-FE2E-002/003 không cần né tránh gì.

## Acceptance Criteria — Kết quả

| # | Criteria | Kết quả |
|---|---|---|
| AC-1 | Bảng inventory đầy đủ, review bởi người hiểu cả FE/BE | ✅ Xác nhận lại, khớp 100%, có thêm bằng chứng backend cho AC ngầm định của CR-001 |
| AC-2 | Câu trả lời dứt khoát cho câu hỏi 2.4 trước CR-FE2E-004 | ✅ (a) — xem mục 5, chi tiết ở SOL-FE2E-004 |
| AC-3 | Không phát sinh thay đổi code | ✅ Solution này thuần investigation, 0 file code thay đổi |

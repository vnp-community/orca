# 01 — Security Conformance vs `docs/hld/v1/security.md`

Đối chiếu: §3 (Credential Management), §4 (Mobile E2E Encryption), §6 (Renderer Sandbox), §7 (IPC Security), §8 (Multi-User Auth Security), §9 (Admin Audit Log), §11 (WebCredentialStore).

---

## 1. `deviceToken` lưu plaintext trong `localStorage`

**Mức độ: 🔴 High**

`security.md` §3 quy định: *"Relay session token: In-memory, ephemeral … Invalidated after session"*. Nhưng cơ chế E2EE pairing của browser lưu token vĩnh viễn, không mã hoá:

- [frontend/src/renderer/src/web/web-runtime-environment.ts:34-36](../../frontend/src/renderer/src/web/web-runtime-environment.ts#L34) — `saveStoredWebRuntimeEnvironment()` gọi `window.localStorage.setItem(ENVIRONMENT_STORAGE_KEY, JSON.stringify(environment))`, trong đó `environment.endpoints[].deviceToken` ([định nghĩa dòng 12](../../frontend/src/renderer/src/web/web-runtime-environment.ts#L12)) là **plaintext**.
- Token này không phải chỉ để hiển thị — nó là bearer credential thật, gửi trên **mọi** RPC call và bắt tay `e2ee_auth`: [web-runtime-client.ts:125, 300, 313, 432, 758](../../frontend/src/renderer/src/web/web-runtime-client.ts#L125).
- Chính code cũng biết đây là secret — comment tại [web-pairing.ts:79-80](../../frontend/src/renderer/src/web/web-pairing.ts#L79): *"pairing payloads include the runtime auth token"*.
- **Đối chứng tốt trong cùng file**: nhánh `session-auth` (`createSessionWebRuntimeEnvironment`, [web-runtime-environment.ts:117-160](../../frontend/src/renderer/src/web/web-runtime-environment.ts#L117)) cố ý để `deviceToken: ''` — chứng tỏ team đã biết pattern đúng (cookie-only, không lưu token) nhưng nhánh E2EE pairing (dùng cho use case B — xem [CR-FE2E series](../../docs/crs/v2/frontend-e2ee/)) là ngoại lệ chưa được đưa vào khuôn khổ này.

**Rủi ro thực tế:** bất kỳ XSS nào trên trang cũng đọc được token, dùng để giả mạo phiên pairing — không có cơ chế thu hồi (revoke) nào phía browser vì token tồn tại vô thời hạn trong `localStorage`.

**Gợi ý:** tối thiểu hoá thời gian sống — mã hoá bằng key theo phiên (session-scoped, không persist qua reload), hoặc chuyển hẳn sang cơ chế session-cookie khi có thể (đã có sẵn hướng đi qua [CR-FE2E series](../../docs/crs/v2/frontend-e2ee/) cho phần multi-user).

---

## 2. `git.push` streaming dùng `sessionStorage` Bearer token

**Mức độ: 🔴 High**

[frontend/src/renderer/src/runtime/runtime-rpc-stream.ts:38-51, 83-87](../../frontend/src/renderer/src/runtime/runtime-rpc-stream.ts) — `webStream()` build header `Authorization: Bearer ${sessionToken}`, với `getSessionToken()` đọc `sessionStorage.getItem('orca_session_token')`. Được gọi thật từ luồng Git panel push: [hooks/useGit.ts:71-83](../../frontend/src/renderer/src/hooks/useGit.ts#L71) → `callRuntimeRpcStream('git.push', ...)`.

`security.md` §8.2 nói rõ session token **chỉ tồn tại server-side** (`orca_sessions` table, "không phải JWT"), lộ ra browser duy nhất qua cookie `HttpOnly` — mô hình `sessionStorage` Bearer token đi ngược hoàn toàn nguyên tắc này (giá trị token phải readable bởi JS thì mới set vào `sessionStorage` được).

**Phát hiện thêm:** grep toàn bộ renderer cho `sessionStorage.setItem('orca_session_token', ...)` — **không có nơi nào set giá trị này**. Nghĩa là hiện tại `git.push` streaming luôn gửi `Authorization: Bearer ` (rỗng) — hoặc tính năng đang lỗi âm thầm, hoặc đang "ăn may" nhờ `fetch` mặc định gửi kèm cookie same-origin nên request vẫn qua được bất chấp header sai.

**File này chưa được commit** (`git status --porcelain` báo `??` cho cả `runtime-rpc-stream.ts` và liên quan) — đây là regression phát sinh **ngoài baseline đã review**, cần chặn trước khi merge chứ không phải "nợ kỹ thuật cũ".

**Gợi ý:** loại bỏ hoàn toàn pattern Bearer/sessionStorage cho luồng này, dùng `credentials: 'include'` như mọi request khác trong `auth-api-client.ts`.

---

## 3. Credential key derivation không theo `userId`

**Mức độ: 🟠 Medium**

`security.md` §11 tuyên bố: *"Encryption: AES-256-GCM … Per-user key từ userId + server secret"*.

[frontend/src/main/credentials/web-credential-store.ts:59, 77, 92](../../frontend/src/main/credentials/web-credential-store.ts#L77) — constructor nhận `userId` nhưng **chỉ dùng để build đường dẫn thư mục** (`userCredDir`); khoá mã hoá thật là:

```
scryptSync(this.serverSecret, salt, 32)                              // V2, dòng 77
scryptSync(serverSecret, 'orca-web-credential-store-v1', 32)         // V1 legacy, dòng 59
```

`userId` **không hề đi vào KDF**. Cách ly giữa các user hiện chỉ dựa vào quyền thư mục filesystem (`mode: 0o700`/`0o600`), không phải mật mã học như doc mô tả. Nếu `serverSecret` (`ORCA_SERVER_SECRET`/`ORCA_CREDENTIAL_KEY`) rò rỉ cùng 1 credential blob (salt lưu plaintext trong chính blob, dòng 86-91), kẻ tấn công giải mã được credential của **bất kỳ user nào**, không chỉ user sở hữu secret bị lộ.

**Phát hiện phụ (Low):** [web-credential-store.ts:28](../../frontend/src/main/credentials/web-credential-store.ts#L28) — IV dài **16 byte**, trong khi §11 ghi rõ *"IV: Random 12 bytes per encryption op"*. Vẫn hợp lệ về mặt kỹ thuật (Node `createCipheriv` chấp nhận IV không phải 96-bit) nhưng lệch so với spec đã viết.

**Gợi ý:** đưa `userId` (hoặc hash của nó) vào input của `scryptSync`, đúng như thiết kế đã công bố; đồng thời cập nhật §11 để phản ánh đúng độ dài IV thực tế, hoặc sửa code về đúng 12 byte nếu 16 byte không có lý do kỹ thuật đặc biệt.

---

## 4. E2EE pairing crypto — ✅ Khớp thiết kế

[frontend/src/renderer/src/web/web-e2ee.ts](../../frontend/src/renderer/src/web/web-e2ee.ts) dùng đúng những gì §4 mô tả:

| Yêu cầu doc | Code thực tế |
|---|---|
| Curve25519 keypair | `nacl.box.keyPair()` (dòng 10) |
| Shared secret qua X25519 | `nacl.box.before(peerPublicKey, ourSecretKey)` (dòng 14) |
| Seal/open bằng NaCl box | `nacl.box.after` / `nacl.box.open.after` (dòng 43, 59) |
| Nonce 24 byte, không tái sử dụng | `nacl.randomBytes(nacl.box.nonceLength)` = 24 byte (dòng 42) |
| CSPRNG | `crypto.getRandomValues` (dòng 3-6) |

Không có sai lệch.

---

## 5. Renderer sandbox & raw `fetch()` — ✅ Khớp thiết kế

Toàn bộ `fetch()` call trong `src/renderer/src` (đã loại test) đều same-origin, thuộc 1 trong 3 nhóm: auth (`/auth/*`, `credentials:'include'`), admin REST (`/admin/api/*`, guard 401/403 đúng §8.3), hoặc web-push (`/api/vapid-public-key`, `/api/push-*`). Không có request nào ra ngoài, không có truy cập filesystem/Node trực tiếp trong renderer.

Ngoại lệ duy nhất đã ghi nhận ở mục 2 (`runtime-rpc-stream.ts`) — không phải "raw fetch ra ngoài" mà là auth header sai kiểu trên cùng origin.

---

## 6. Admin audit log — ✅ Khớp thiết kế

`AdminApp.tsx` → `AuditPage.tsx` gọi `fetchAdminAudit()` → `GET /admin/api/audit` thật (không mock), có `credentials: 'include'`, throw đúng khi 401/403 (khớp §8.3 `requireAdmin()`). Không có API xoá audit entry nào — khớp nguyên tắc append-only ở §9. CSV export chỉ render lại dữ liệu đã fetch, không có endpoint export riêng không cần auth.

---

## 7. Session cookie — ✅ Khớp thiết kế (ngoại trừ mục 2)

`auth-api-client.ts` — mọi request dùng `credentials: 'include'`, không bao giờ đọc giá trị cookie qua JS. `main-web-bootstrap.tsx`/`useLogout.ts` chỉ **enumerate để expire** cookie lúc logout (không đọc value) — khớp thiết kế `HttpOnly` ở §8.2.

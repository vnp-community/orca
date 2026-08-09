# BUG-FE-HLD-002 — `git.push` streaming dùng `sessionStorage` Bearer token thay vì cookie session, không nơi nào set giá trị

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Module:** `frontend/src/renderer/src/runtime/runtime-rpc-stream.ts`, `frontend/src/renderer/src/hooks/useGit.ts`
**Phát hiện:** 2026-08-08 (audit `frontend/` code vs thiết kế — `audit/frontend/01-security-conformance.md` §2)

---

## Mô tả

`docs/hld/v1/security.md` §8.2 quy định session token **chỉ tồn tại server-side** (`orca_sessions` table, "không phải JWT"), lộ ra browser duy nhất qua cookie `HttpOnly`. Mọi request RPC khác trong codebase tuân theo đúng pattern này (`auth-api-client.ts`, `admin-api-client.ts` — đều dùng `credentials: 'include'`, không đọc/lưu giá trị token).

Nhưng luồng streaming cho `git.push` lại đi khác hẳn:

- `webStream()` (`runtime-rpc-stream.ts:38-51`) build header `Authorization: Bearer ${sessionToken}`.
- `getSessionToken()` (`runtime-rpc-stream.ts:83-87`) đọc `sessionStorage.getItem('orca_session_token')`.
- Được gọi thật từ Git panel: `useGit.ts:71-83` → `callRuntimeRpcStream('git.push', ...)`.

**Grep toàn bộ renderer cho `sessionStorage.setItem('orca_session_token', ...)` — không có nơi nào set giá trị này.** Nghĩa là request hiện tại luôn gửi `Authorization: Bearer ` (rỗng).

## Hậu quả

Một trong hai khả năng, cả hai đều là bug cần xác nhận:

1. **Nếu server có kiểm tra header `Authorization`** cho endpoint stream này: mọi lệnh `git push` qua UI streaming path sẽ **fail xác thực** (401/403) — tính năng đang chết âm thầm.
2. **Nếu server bỏ qua header sai và chỉ dựa vào cookie same-origin** (do `fetch` mặc định gửi kèm cookie): tính năng vẫn chạy được, nhưng code hiện tại là **dead/misleading code** — trông như đang dùng bearer token auth trong khi thực chất đang dựa hoàn toàn vào cookie, gây hiểu lầm nghiêm trọng cho người đọc/maintain sau này, và là một điểm không nhất quán duy nhất trong toàn bộ hệ thống auth.

Cả hai trường hợp đều cần sửa. **File này chưa được commit vào git** (`git status --porcelain` báo `??`) — đây là regression phát sinh ngoài baseline đã review, nên chặn trước khi merge sẽ rẻ hơn nhiều so với debug sau khi đã lên production.

## Bằng chứng

```
runtime-rpc-stream.ts:38-51  → Authorization: Bearer ${sessionToken}
runtime-rpc-stream.ts:83-87  → getSessionToken() đọc sessionStorage.getItem('orca_session_token')
hooks/useGit.ts:71-83        → gọi callRuntimeRpcStream('git.push', ...) — luồng thật, không phải test
grep sessionStorage.setItem('orca_session_token' ...) toàn renderer → 0 kết quả
```

## Đề xuất fix

1. Xoá pattern `Authorization: Bearer` + `sessionStorage` khỏi `runtime-rpc-stream.ts`.
2. Đổi `webStream()` sang dùng `credentials: 'include'` giống mọi request khác — cookie `orca_session` tự động đính kèm same-origin, không cần header thủ công.
3. Thêm test cho `git.push` streaming xác nhận request thật sự thành công qua cookie (hiện 0 test bao phủ luồng này theo audit `04-code-health-and-standards.md`).

## Tham khảo

- Audit: `audit/frontend/01-security-conformance.md` §2
- Doc gốc: `docs/hld/v1/security.md` §8.2
- Đối chứng pattern đúng: `frontend/src/renderer/src/auth/auth-api-client.ts` (mọi call `credentials:'include'`)

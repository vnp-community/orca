# BUG-FE-HLD-001 — E2EE pairing `deviceToken` lưu plaintext trong `localStorage`

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Module:** `frontend/src/renderer/src/web/web-runtime-environment.ts`, `web-runtime-client.ts`, `web-pairing.ts`
**Phát hiện:** 2026-08-08 (audit `frontend/` code vs thiết kế — `audit/frontend/01-security-conformance.md` §1)

---

## Mô tả

`docs/hld/v1/security.md` §3 (Credential Management) quy định: *"Relay session token: In-memory, ephemeral … Invalidated after session"*.

Nhưng cơ chế E2EE pairing của browser (dùng khi browser pair trực tiếp vào 1 Desktop app/bare relay — không qua Orca Web Server multi-user) lưu `deviceToken` **vĩnh viễn, không mã hoá** trong `localStorage`:

- `saveStoredWebRuntimeEnvironment()` (`web-runtime-environment.ts:34-36`) gọi `window.localStorage.setItem(ENVIRONMENT_STORAGE_KEY, JSON.stringify(environment))`, trong đó `environment.endpoints[].deviceToken` (định nghĩa dòng 12, populate ở dòng 63) là plaintext.
- Token này không chỉ là nhãn hiển thị — nó là bearer credential thật, gửi trên **mọi** RPC call và bắt tay `e2ee_auth`: `web-runtime-client.ts:125, 300, 313, 432, 758` đều gửi `deviceToken: this.pairing.deviceToken`.
- Chính tác giả code cũng biết đây là secret — comment tại `web-pairing.ts:79-80`: *"pairing payloads include the runtime auth token"*.

**Đối chứng cho thấy team biết cách làm đúng:** nhánh `session-auth` (`createSessionWebRuntimeEnvironment`, `web-runtime-environment.ts:117-160`) cố ý để `deviceToken: ''` — nghĩa là pattern "không lưu token nhạy cảm" đã tồn tại trong cùng codebase, chỉ chưa được áp dụng cho nhánh E2EE pairing.

## Hậu quả

- Bất kỳ XSS nào trên trang browser (kể cả từ 1 dependency bị compromise) đọc được `localStorage` sẽ lấy được `deviceToken`, dùng để giả mạo phiên pairing tới runtime đích.
- Không có cơ chế thu hồi (revoke) phía browser — token tồn tại vô thời hạn cho tới khi user tự tay "Remove instance" trong `OrcaInstanceSwitcher`/`AddInstanceForm`.
- Khác hẳn model `HttpOnly` cookie (§8.2) đang dùng đúng cho luồng multi-user — đây là ngoại lệ bảo mật yếu hơn hẳn phần còn lại của hệ thống.

## Bằng chứng

```
web-runtime-environment.ts:34-36   → localStorage.setItem(..., JSON.stringify(environment)) — plaintext
web-runtime-environment.ts:12       → deviceToken field trong StoredWebRuntimeEnvironment
web-runtime-client.ts:125,300,313,432,758 → deviceToken gửi trên mọi RPC call
web-pairing.ts:79-80                → comment tự nhận "pairing payloads include the runtime auth token"
web-runtime-environment.ts:117-160  → session-auth branch dùng deviceToken:'' — đối chứng pattern đúng
```

## Đề xuất fix

1. Tối thiểu: rút ngắn thời gian sống của token lưu trong `localStorage` (yêu cầu re-pair định kỳ), hoặc mã hoá tại rest bằng key không lưu cùng nơi (vd. derive từ 1 giá trị chỉ có trong session, không persist).
2. Dài hạn: với phần browser nói chuyện với Orca Web Server multi-user (use case A), loại bỏ hẳn cơ chế deviceToken theo hướng [CR-FE2E series](../../../../docs/crs/v2/frontend-e2ee/) đã đề xuất — chỉ còn use case B (Desktop Pair Code sharing) cần giữ token kiểu này, phạm vi rủi ro thu hẹp lại.

## Tham khảo

- Audit: `audit/frontend/01-security-conformance.md` §1
- Doc gốc: `docs/hld/v1/security.md` §3, §4
- Liên quan: [CR-FE2E-001..005](../../../../docs/crs/v2/frontend-e2ee/) (kế hoạch bỏ E2EE pairing khỏi luồng multi-user)

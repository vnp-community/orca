# BUG-BE-HLD-014 — AI Provider key rotation (grace period, status 'rotating', audit log) hoàn toàn không tồn tại

**Mức độ:** 🟡 MEDIUM (Feature gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/ai-providers/AIProviderService.ts`, `backend/src/shared/ai-provider-types.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.14/F35)

---

## Mô tả

`docs/features/F35-ai-provider-account-management.md` mô tả key rotation với grace period 30 giây, status trung gian `'rotating'`, và ghi audit log cho từng lần rotate.

Thực tế:
- Không có method `rotateKey` ở đâu trong `AIProviderService`.
- `AIProviderStatus` (`backend/src/shared/ai-provider-types.ts:29-34`) chỉ có `'pending'|'active'|'invalid'|'quota_exceeded'|'unreachable'` — **không có `'rotating'`**.
- Không có RPC method `rotateKey` (namespace `aiProvider.*` thực tế có 9 method: `list, create, get, update, delete, writeCredential, testConnection, getUsageToday, resolve` — không `rotateKey`).
- Update key hiện tại chỉ có thể qua `aiProvider.writeCredential` — **ghi đè trực tiếp**, không có grace period, không invalidate cache trước, không audit log.

## Hậu quả

- Đổi API key cho 1 account đang active có thể làm gián đoạn request đang chạy dở (không có grace period để request cũ hoàn tất với key cũ trước khi chuyển hẳn sang key mới).
- Không có audit trail cho việc ai đã đổi key lúc nào — khó điều tra khi có sự cố liên quan credential.

## Bằng chứng

- Grep `rotateKey` toàn `backend/src`: 0 kết quả.
- `backend/src/shared/ai-provider-types.ts:29-34` — enum status thiếu `'rotating'`.
- `backend/src/main/ai-providers/ai-provider-rpc-handler.ts:82-194` — danh sách 9 method thật, xác nhận không có rotateKey.
- Grep `audit_log`/`auditLog` trong `backend/src/main/ai-providers/`: 0 kết quả.

## Đề xuất fix

1. Thêm `'rotating'` vào `AIProviderStatus`.
2. Implement `AIProviderService.rotateKey(accountId, newCredential)`: set status `'rotating'`, giữ key cũ hoạt động song song trong grace period (30s cấu hình được), sau đó chuyển hẳn sang key mới + set `'active'`.
3. Ghi audit log (dùng lại `AuditLogger` đã có ở domain Auth, `backend/src/main/auth/audit-logger.ts`) cho mọi thao tác CRUD/rotate của AI Provider — hiện tại toàn bộ domain này không có audit log nào (xem thêm [BUG-BE-HLD-015](./BUG-BE-HLD-015-ai-provider-quota-alert-not-implemented.md) cho gap liên quan khác).

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.14 (F35)
- Doc gốc: `docs/features/F35-ai-provider-account-management.md`
- Liên quan: [BUG-BE-HLD-015](./BUG-BE-HLD-015-ai-provider-quota-alert-not-implemented.md)

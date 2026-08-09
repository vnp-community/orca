# BUG-BE-HLD-011 — Auto-respawn session khi crash (max 3) và cấu hình idle-timeout qua env var đều chưa cài đặt

**Mức độ:** 🟡 MEDIUM (Reliability gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/session/session-manager.ts`, `session-types.ts`
**Phát hiện:** 2026-08-08/09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §2.8, §5.3/F24)

---

## Mô tả

`docs/hld/backend-server-architecture.md §7` và `docs/features/F24-per-user-sandbox.md` liệt kê 2 tiêu chí đã hoàn thành:

1. **Auto-respawn khi process crash, tối đa 3 lần** — field `respawnCount` (`session-types.ts:18`) và `maxRespawnAttempts` (`session-types.ts:30`, dùng ở `session-manager.ts:32`) được định nghĩa nhưng **không có bất kỳ nơi nào đọc hay tăng giá trị này**. Handler `child.on('exit', ...)` (`session-manager.ts:161-166`) chỉ xoá process khỏi map và dọn socket file — không gọi lại `spawnUserProcess`. Process con chỉ được khởi động lại khi có kết nối WS mới của user đó (qua `getOrSpawnUserProcess`), không phải cơ chế auto-respawn có giới hạn số lần.

2. **Idle timeout cấu hình được qua `SESSION_IDLE_TIMEOUT_MS`** — grep toàn `backend/src/` cho chuỗi này: **0 kết quả**. `SessionManager` được khởi tạo ở cả 2 nơi (`server-bootstrap.ts:311-316`, `backend/src/server/index.ts:137-142`) **không truyền `idleTimeoutMs`** — nên luôn dùng cứng `DEFAULT_IDLE_TIMEOUT_MS = 4h` (`session-manager.ts:19`), không có cách nào override qua env var.

## Hậu quả

- Nếu 1 user process crash (OOM, uncaught exception...), user đó **mất kết nối hoàn toàn** cho tới khi tự reconnect thủ công (không có auto-recovery), thay vì được tự động respawn trong nền như tài liệu hứa.
- Vận hành production không thể tinh chỉnh idle-timeout theo nhu cầu (vd rút ngắn xuống 1h để tiết kiệm resource, hoặc kéo dài cho use-case đặc thù) mà không sửa code.

## Bằng chứng

- `backend/src/main/session/session-types.ts:18,30` — field tồn tại nhưng dead.
- `backend/src/main/session/session-manager.ts:161-166` — `child.on('exit')` không respawn.
- `backend/src/main/session/session-manager.ts:19` — `DEFAULT_IDLE_TIMEOUT_MS` hard-code.
- `backend/src/main/server-bootstrap.ts:311-316`, `backend/src/server/index.ts:137-142` — cả 2 nơi khởi tạo không đọc env var.

## Đề xuất fix

1. Trong `child.on('exit', ...)`, thêm logic: nếu `proc.respawnCount < config.maxRespawnAttempts` và exit không phải do shutdown chủ động, tăng `respawnCount` và gọi lại `spawnUserProcess(userId)` sau backoff ngắn; nếu vượt quá, log cảnh báo và không respawn nữa (tránh crash loop).
2. Đọc `process.env['SESSION_IDLE_TIMEOUT_MS']` khi khởi tạo `SessionManager` ở cả 2 entry point, fallback về `DEFAULT_IDLE_TIMEOUT_MS` nếu không set.
3. Viết test: kill child process giả lập, assert respawn xảy ra đúng ≤3 lần rồi dừng.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §2.8, §5.3 (F24), §6 mục 8 (Top 10)
- Doc gốc: `docs/hld/backend-server-architecture.md` §7, `docs/features/F24-per-user-sandbox.md`
- Liên quan: [BUG-AUTH-003](../auth/BUG-AUTH-003-session-manager-no-idle-timeout.md) (bug cũ, đã đánh dấu FIXED — cần đối chiếu lại vì finding hiện tại cho thấy vẫn thiếu cấu hình qua env var)

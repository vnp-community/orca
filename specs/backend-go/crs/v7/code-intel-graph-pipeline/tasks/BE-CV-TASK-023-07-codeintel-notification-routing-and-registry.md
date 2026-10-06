# BE-CV-TASK-023-07: Định tuyến thông báo `codeintel.*`/`quality.*`, sổ đăng ký ở `Client`, `resync` và `overflow`

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/codeintel_event.go` (mới), `.../adapter/devserveragent/codeintel_events.go` (mới), `.../adapter/devserveragent/codeintel_notification_decoding.go` (mới), `.../adapter/devserveragent/session.go` (sửa `routeNotification` dòng 434–466, `newSession` dòng 198, `attachTransport` dòng 245, struct `session`), `.../adapter/devserveragent/client.go` (4 chỗ gọi `newSession`), `.../usecase/ports.go` (cổng `CodeIntelEventSource`), test
**Depends on:** TASK-023-01
**Status:** [ ] TODO

---

## Context

`routeNotification` bỏ im lặng mọi method lạ (`default: return`). PQ-17: chuyển 4 thông báo, `workspaceRoot` bắt buộc. Sổ đăng ký phải ở `Client` vì `direct-websocket` chỉ có phiên sau lần agent nối đầu (`getInboundSession`). Mẫu fan-out không chặn: `routeFileWatchNotification`/`subscribeFileWatch` (`session.go` ~740–870), `portevents.Broadcaster`.

## Việc cần làm

1. `domain/codeintel_event.go`: `CodeIntelEvent` (20 trường tương ứng proto; `Percent *int`), hằng `Kind*` (`index_changed, reindex_progress, quality_progress, quality_finished, resync, overflow`).
2. `usecase/ports.go`: `CodeIntelEventSource` (SOL-023 mục 2.D). **Không** thêm vào `DevServerAgentClient`.
3. `codeintel_notification_decoding.go`: `decodeCodeIntelNotification(n JSONRPCNotification, now time.Time) (domain.CodeIntelEvent, bool)` theo ánh xạ mục 2.D; `percent` JSON `null` hoặc vắng → `nil`; `payload_json` = `params` nếu ≤ 64 KiB; thiếu `workspaceRoot` (hoặc `jobId`/`runId` tương ứng) → `ok=false`.
4. `codeintel_events.go`: `Client.codeIntelSubs`, `Client.SubscribeCodeIntelEvents(devServerID) (<-chan CodeIntelEvent, func())`, `publishCodeIntelEvent(devServerID, ev)` (gửi không chặn; đầy → đặt cờ `dropped`; bơm phát đúng một `overflow` sau khi rút cạn), `emitResync(devServerID)`.
5. `session.go`: thêm `devServerID`, `onCodeIntel`, `onAttached`; `routeNotification` thêm `case` bốn method; `attachTransport` gọi `onAttached` cuối hàm (sau khi `go s.readLoop`). `Client` gắn hai hook tại bốn nơi gọi `newSession`.
6. `unsubscribe` idempotent, đóng kênh đúng một lần (khuôn `unsubscribeFileWatch`).

## Kiểm thử

- `cd backend-go/services/infra-fleet-service && go test ./internal/adapter/devserveragent/ -run 'CodeIntel' -race -count=3`.
- `fakeAgent` gửi `codeintel.indexChanged {workspaceRoot,tool,commit,indexedAt,reason,headCommit,stale,indexScope,mergeBase,trigger}` → subscriber của dev server đó nhận đúng một `index_changed` với mọi trường; subscriber dev server khác không nhận.
- `reindexProgress` với `percent:null` → `Percent == nil`; số `37` → 37; `quality.progress`, `quality.finished` (kèm `summary` lớn > 64 KiB → `payload_json` rỗng).
- Đăng ký **trước** khi agent kết nối (direct-websocket) rồi `AttachInboundSession` → nhận `resync`.
- 200 thông báo không đọc → đọc được ≤ 64 sự kiện + đúng một `overflow`; `readLoop` không bị chặn (đo bằng gửi thêm một request và nhận phản hồi trong 1 s).
- Race: 50 subscribe/unsubscribe song song với phát (`-race`).

## Tiêu chí hoàn thành

- [ ] Bốn thông báo được giải mã và định tuyến theo `dev_server_id` của phiên.
- [ ] `resync` khi agent nối lại; `overflow` khi đầy; không chặn `readLoop`.
- [ ] Không log `params` ở mức info.
- [ ] Không file `helpers/utils/common/misc`; `session.go` không thêm `max-lines` disable (đưa logic mới vào file riêng).

## Rủi ro và lưu ý

- `session.go` đã dài (1 429 dòng); đừng thêm logic lớn vào đó.
- Thông báo chỉ tới sau khi backend gọi một method `codeintel.*`/`quality.*` (agent contract §6): kiểm thử ở đây dùng `fakeAgent` phát trực tiếp, không chứng minh hành vi thật của agent.
- Mất sự kiện lúc agent offline không khôi phục được ngoài `resync`.

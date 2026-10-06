# BE-CV-SOL-040-codeintel-write-and-stream-channels: 10 kênh ghi/trạng thái và stream `codeIntel.subscribe`

> **Proposed.** Chưa triển khai, chưa chạy test nào. Nhóm kênh đổi trạng thái (reindex, dismiss, review state, C4, bind, settings) và kênh push duy nhất.

**CR:** [CR-CV-040](../../../../../../docs/crs/v7/code-intel-gateway/CR-CV-040-api-gateway-codeintel-channels.md)
**Service:** `api-gateway` (`internal/adapter/wscompat`)
**TDD tham chiếu:** [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (tenant, audit), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (stream, deadline), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (rò goroutine, resilience), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md) mục 2

---

## Hợp đồng áp dụng

| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-ui-api.md` | §3.1 (10 kênh: `reindex, reindexStatus, dismissFinding, reviewState.get, reviewState.save, c4.get, c4.save, bindRepo, settings.get, settings.set` + `subscribe`), §4.1 (`ReindexJob`, `Settings`), §4.6 (`ReviewState`), §5 (push), §2.4 (cỡ), §6 (cờ), §1 U2, U7, U8 |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-01, PQ-05 (dismiss `disposition`, `note`), PQ-11 (subscribe), PQ-16 (reindex), PQ-22 (`review_states`, `version:0`), PQ-24 (cờ, ngoại lệ `GetReindexJob`), PQ-14 (cỡ), §3.1 (RPC `RequestReindex, GetReindexJob, DismissFinding, GetReviewState, SaveReviewState, GetC4Overrides, SaveC4Overrides, BindRepo, GetSettings, SetSettings, StreamCodeIntelEvents`), §6.1 |

## Lệch giữa CR và hợp đồng

| # | CR-CV-040 | Hợp đồng | Theo |
|---|---|---|---|
| W1 | `subscribe` nhận `worktreeId?` | `{selectors?: {projectId, worktreeId}[]}` 0..50 (PQ-11) | hợp đồng |
| W2 | push chỉ `changed`, `reindexProgress` | thêm `quality.progress|finished|gateChanged` (5 kênh push), mỗi khung có `event`, `projectId`, `worktreeId`, `occurredAt` (UI-API §5) | hợp đồng |
| W3 | `reviewState.save` 28 KiB; `c4.save` `document ≤ 24 KiB` | 256 KiB (`readingProgress ≤ 64 KiB`, `notes ≤ 256 KiB`) và 96 KiB (`document ≤ 64 KiB`) | hợp đồng |
| W4 | `reviewState.save` gồm `readingProgress, notes, expectedVersion` | thêm `turnMarkers?`, `status?` (≤ 5 marker) | hợp đồng |
| W5 | `dismissFinding(findingKey, reason)` | thêm `action` dismiss\|restore, `disposition`, `note` (PQ-05) | hợp đồng |
| W6 | `settings.set {enabled}` | 9 trường tuỳ chọn, ≥ 1 trường (UI-API 3.1) | hợp đồng |
| W7 | `reindex.mode` bắt buộc | tuỳ chọn, mặc định `incremental` (service áp) | hợp đồng |
| W8 | `codeIntel.status`-giống: `reviewState.get` lỗi khi chưa có | trả `version:0` mặc định, không lỗi (PQ-22) | hợp đồng |

## Phụ thuộc chéo khu vực

| Hướng | Solution | Ghi chú |
|---|---|---|
| BE trước | `BE-CV-SOL-040-codeintel-channel-foundation` | runner, args, lỗi, catalog |
| BE trước (RPC) | `BE-CV-SOL-021-agent-collector` (`RequestReindex`, `GetReindexJob`), `BE-CV-SOL-037-…` (`DismissFinding`), `BE-CV-SOL-011-repositories-and-maintenance` + CR-052/060 (`Get/SaveReviewState`), `BE-CV-SOL-033-c4-overrides-yaml`, `BE-CV-SOL-012-target-resolution-and-bindings` (`BindRepo`), `BE-CV-SOL-073-settings-flag-and-rollout` (`Get/SetSettings`), `BE-CV-SOL-024-event-distribution` (`StreamCodeIntelEvents`) | |
| FE sau | `FE-CV-SOL-050-store-and-query-hooks`, 052, 060, 073-FE | tải lại khi `VERSION_CONFLICT`; không dùng `send` |
| AG | không | |

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (cùng phiên, 2026-10-06): `wscompat/{handler.go (100-260, 340-411), push_bridge.go, channels_push.go (40-188), file_watch_stream_registry.go (1-70), registry.go, session_dialect.go, envelope.go, channels_mcp_approval.go (100-115)}`.

Xác nhận:
- `handleSubscribe` (`handler.go:~351`) gọi `sh(ctx, identity, msg.Args)` với **ctx kết nối** (không timeout), ack `nil` ngay sau khi `sh` trả về không lỗi, rồi `pipePushForDialect`; lỗi `sh` ghi bằng `writeDialectError`.
- `pipePush` chỉ kết thúc khi ctx kết nối xong hoặc kênh `events` đóng; session-client nhận thêm khung `{"type":"end"}` khi đóng (`push_bridge.go`).
- Đăng ký stream bằng `Registry.RegisterStream` (`registry.go:115`); không có cơ chế "một subscribe mỗi kết nối" sẵn; các registry theo kết nối nằm trong `handler.go` (`terminalStreamsContext`, `fileWatchStreamsContext`...) và dựng bằng `...Context(ctx, newXRegistry())`.
- `workspacePorts.subscribe` (`channels_push.go:81`) mở gRPC stream, gắn identity **không có Role**, không kiểm `DeviceID`.
- Với gRPC server-streaming, lỗi từ chối của server thường chỉ xuất hiện ở `Recv` đầu (hành vi grpc-go theo kiến thức thư viện, chưa chạy thử trên repo này): mẫu hiện tại sẽ ack thành công rồi đóng ngầm.

### Correction relative to CR-CV-040

| # | CR nói | Thực tế | Xử lý |
|---|---|---|---|
| C1 | "lỗi quyền trả ở ack" (2.5) | ack ghi sau khi `sh()` trả; nếu `sh` không chờ phản hồi server thì lỗi quyền chỉ hiện ở `Recv` | `sh()` chờ `stream.Header()` tối đa 3 s trước khi trả kênh (xem 2.3); yêu cầu service gửi header ngay sau kiểm quyền (BE-CV-SOL-024) |
| C2 | "mỗi socket tối đa 1 subscribe, lần hai thay lần đầu" | không có chỗ lưu theo kết nối cho kênh này | thêm `codeIntelSubscriptionRegistry` theo kết nối, nối vào `ServeHTTP` như `fileWatchStreamsContext` |
| C3 | khung `resync` mô tả cho `codeIntel.changed` | `PushBase` bắt buộc `projectId`, `worktreeId` | khung resync toàn luồng dùng chuỗi rỗng cho hai trường (Q1) |

## 2. Giải pháp

### 2.1 Kênh ghi/trạng thái (mô tả tham số, đầu vào cho task)

| Kênh | Trường (ngoài sel) | Kiểm gateway | RPC | Cỡ |
|---|---|---|---|---|
| `reindex` | `mode?` incremental\|full | enum | `RequestReindex` | 16 KiB |
| `reindexStatus` | `jobId` | bắt buộc, `checkOpaqueID` | `GetReindexJob` | |
| `dismissFinding` | `findingKey`, `action?`, `disposition?`, `reason? ≤ 500`, `note? ≤ 500` | `findingKey` 1..128; `action` dismiss\|restore; `disposition` ignored\|resolved | `DismissFinding` | |
| `reviewState.get` | `baseCommit?`, `headCommit?` | `checkGitRefLike` (cho phép rỗng) | `GetReviewState` | |
| `reviewState.save` | `baseCommit`, `headCommit` (khoá bắt buộc, giá trị có thể rỗng), `readingProgress`, `notes?`, `turnMarkers?` (≤ 5), `status?` open\|reviewed, `expectedVersion` (≥ 0) | cỡ `readingProgress ≤ 64 KiB`, `notes ≤ 256 KiB`; tổng ≤ 256 KiB | `SaveReviewState` | 256 KiB |
| `c4.get` | `container?` | ≤ 512 | `GetC4Overrides` | |
| `c4.save` | `container`, `document` (chuỗi YAML ≤ 64 KiB), `expectedVersion` | UTF-8 hợp lệ, không NUL | `SaveC4Overrides` | 96 KiB |
| `bindRepo` | (sel) | — | `BindRepo` | |
| `settings.get` | `{}` | cho phép `DeviceID` | `GetSettings` | |
| `settings.set` | 9 trường tuỳ chọn, ≥ 1 | bool/enum/`hotspotWindowDays` 30..365 | `SetSettings` | |

Ghi chú: `reviewState.*` forward JSON con (`readingProgress`, `notes`, `turnMarkers`) nguyên văn sau khi kiểm cỡ; kiểu proto (chuỗi JSON hay message) do CR-011/052/060 chốt (Q2); gateway không diễn giải cấu trúc. Kênh ghi không idempotent: không retry. `CODEINTEL_VERSION_CONFLICT | {"currentVersion":N}` đi qua nguyên. `reindex` trả ngay `{jobId,status,mode,trigger}`; `GetReindexJob` được phép khi cờ tắt (PQ-24) nên **không** chặn ở gateway.

### 2.2 Cờ tắt

Gateway không giữ cờ. `CODEINTEL_DISABLED` do service trả (trừ `settings.get|set`, `reindexStatus`); client `kind:'disabled'` (U7).

### 2.3 `codeIntel.subscribe`

```go
r.RegisterStream("codeIntel.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
    // 1 client nil => UNAVAILABLE; 2 DeviceID => NOT_AUTHORIZED; 3 decode {selectors?} (<=50, mỗi sel hợp lệ)
    // 4 limiter: replica đang giữ >= MaxStreams => RATE_LIMITED | {"scope":"replica"}
    // 5 subCtx, cancel := context.WithCancel(ctx); registry theo kết nối: replace(cancel) huỷ lần trước
    // 6 subCtx = gatewaygrpc.AttachIdentity(subCtx, usecase.Identity{TenantID, UserID, Role})
    // 7 stream, err := core.StreamCodeIntelEvents(subCtx, req); chờ stream.Header() <= 3 s (lỗi => codeIntelChannelError, huỷ)
    // 8 goroutine Recv -> translate -> select{out<-ev | subCtx.Done()}; Recv lỗi và subCtx chưa huỷ => phát một khung changed{resync:true} rồi đóng out
})
```
- **Dịch khung** (`CodeIntelPush.kind` -> kênh native, `event`): `changed` -> `codeIntel.changed`/`changed`; `reindex_progress` -> `codeIntel.reindexProgress`/`reindexProgress`; `quality_progress` -> `codeIntel.quality.progress`/`quality.progress`; `quality_finished` -> `codeIntel.quality.finished`/`quality.finished`; `quality_gate_changed` -> `codeIntel.quality.gateChanged`/`quality.gateChanged`. Kiểu `kind` lạ: bỏ, đếm. Payload object theo UI-API §5: `PushBase{event, projectId, worktreeId, occurredAt}` + trường riêng; `percent` null khi `optional` vắng; các trường quality lấy từ `payload_json` bằng struct **whitelist** (không chuyển nguyên); mỗi khung ≤ 1 KiB (cắt `message`); không đồ thị, không mã nguồn, không đường dẫn tuyệt đối, không `tenantId`.
- **Một subscribe/kết nối**: `codeIntelSubscriptionRegistry` (mutex + `cancel`) gắn bằng `codeIntelSubscriptionContext(ctx, new...)` ở `ServeHTTP` cạnh `fileWatchStreamsContext`; lần hai gọi `cancel` cũ rồi thay.
- **Giới hạn luồng**: bộ đếm nguyên tử cấp tiến trình, tăng khi mở, giảm trong `defer` của goroutine; vượt `CODE_INTEL_MAX_STREAMS` => `CODEINTEL_RATE_LIMITED: too many streams | {"scope":"replica"}`.
- **Áp lực ngược**: `out` không đệm như mẫu; chặn ở `Recv` nhờ flow control; coalesce là việc của service.
- **Resync**: khi stream/service đứt (`Recv` lỗi, ctx chưa huỷ): đúng **một** `changed{projectId:"", worktreeId:"", tools:[], stale:true, reason:"resync", resync:true, occurredAt}` rồi đóng; gateway không tự mở lại. Khi `subCtx` bị huỷ (thay thế/ngắt kết nối): đóng im lặng.

### 2.4 Tệp mới

`channels_codeintel_state_reindex.go`, `..._state_review.go` (dismiss, reviewState, c4), `..._state_settings.go` (bindRepo, settings), `channels_codeintel_stream.go` (đăng ký + limiter), `channels_codeintel_stream_events.go` (dịch khung), `codeintel_subscription_registry.go`, `handler.go` (một dòng context), các `_test.go`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | `Header()` handshake 3 s | lỗi quyền/cờ phải ở ack, không thành `resync` vô hạn |
| D2 | Registry subscribe theo kết nối | CR 2.5 "lần hai thay lần đầu" |
| D3 | Whitelist khung push, ≤ 1 KiB | không rò dữ liệu/tenant (UI-API §5) |
| D4 | Không retry kênh ghi | không idempotent; `expectedVersion` xử lý xung đột |
| D5 | Gateway không giữ cờ, không chặn `reindexStatus` khi tắt | PQ-24 |
| D6 | Resync dùng chuỗi rỗng cho `projectId/worktreeId` | khung toàn luồng không thuộc worktree (Q1) |

## 4. Phụ thuộc và thứ tự

TASK-040-16, 17, 18, 19 song song (đều chỉ cần 040-07); 20 (stream mở + registry) trước 21 (dịch khung); 22 cuối (kiểm tra rò goroutine, hai dialect).

## 5. Kiểm thử

Bảng validate cho từng kênh ghi (biên cỡ, enum, `expectedVersion` âm, `turnMarkers` 6 phần tử, `settings.set {}` rỗng => `INVALID_PARAMS`); fake client (deadline 8 s; metadata Role); `Aborted` => `VERSION_CONFLICT`; `reviewState.get` `version:0`; tenant isolation (metadata). Stream: ack, từng loại khung, resync, thay thế, giới hạn, huỷ ctx không rò goroutine (`goleak` hoặc đếm `runtime.NumGoroutine` có dung sai), hai dialect, `DeviceID`. Chưa chạy bất kỳ test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- `send` (fire-and-forget) nuốt lỗi kênh ghi (`handleSend`, `handler.go:397`): chỉ giảm bằng quy ước FE (U2).
- Mỗi socket một gRPC stream: chưa đo tải; 500/replica là đề xuất.
- `Header()` handshake cần service gửi header sớm; nếu không, hết 3 s ta coi là đã mở và lỗi quyền hiện ở `Recv` đầu (`resync`): chấp nhận có điều kiện, ghi PR.
- Định dạng `payload_json` (camelCase hay snake_case) chưa chốt (Q3).
- Số proto field và kiểu JSON-in-proto chưa chốt.

## 7. Câu hỏi mở

- **Q1.** Xác nhận khung `resync` toàn luồng với `projectId/worktreeId` rỗng.
- **Q2.** Kiểu proto của `readingProgress/notes/turnMarkers` (chuỗi JSON hay message).
- **Q3.** Quy ước khoá của `payload_json` (camelCase đề xuất).
- **Q4.** `reviewState.save`: `baseCommit/headCommit` rỗng hợp lệ chỉ cho dòng mức worktree (PQ-22); gateway chỉ kiểm có mặt khoá.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` (§3.1, §4.1, §4.6, §5)
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-05/11/14/16/22/24, §3.1)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/{handler.go,push_bridge.go,channels_push.go,file_watch_stream_registry.go}`

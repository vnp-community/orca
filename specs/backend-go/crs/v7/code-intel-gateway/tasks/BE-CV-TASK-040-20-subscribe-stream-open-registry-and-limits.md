# BE-CV-TASK-040-20: `codeIntel.subscribe`: mở stream, registry theo kết nối, giới hạn luồng, handshake header

**From Solution:** BE-CV-SOL-040-codeintel-write-and-stream-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_stream.go` (mới), `codeintel_subscription_registry.go` (mới), `handler.go` (thêm một dòng context), `channels_codeintel_stream_test.go` (mới)
**Depends on:** TASK-040-07, TASK-040-04; stub `StreamCodeIntelEvents` (BE-CV-SOL-024-event-distribution)
**Status:** [ ] TODO

---

## Context

Mẫu: `registerWorkspacePortsStreamChannel` (`channels_push.go:81`) nhưng (1) phải gắn `AttachIdentity` **có Role** (stream không qua `Dispatch`), (2) `handleSubscribe` truyền ctx kết nối không timeout và ack ngay khi `sh()` trả, (3) CR 2.5 yêu cầu 1 subscribe/kết nối, `CODE_INTEL_MAX_STREAMS`/replica. PQ-11: `{selectors?: {projectId, worktreeId}[]}` 0..50; rỗng = mọi worktree được đọc (service lọc, cache 10 s). Chạy `gitnexus_impact` trên `handleSubscribe`/`ServeHTTP` trước khi sửa.

## Việc cần làm

1. `codeIntelSubscriptionRegistry{mu; cancel context.CancelFunc}`: `replace(cancel) (old)`, `clear(cancel)`; `codeIntelSubscriptionContext(ctx, reg)`/`...FromContext` (khuôn `file_watch_stream_registry.go`); trong `ServeHTTP` thêm `ctx = codeIntelSubscriptionContext(ctx, newCodeIntelSubscriptionRegistry())` cạnh `fileWatchStreamsContext`. Nếu ctx không có registry (test), vẫn hoạt động nhưng không thay thế.
2. `codeIntelStreamGate{n atomic.Int64; max int}`: `tryAcquire()/release()` cấp tiến trình, `max` từ `CodeIntelLimits.MaxStreams`.
3. `registerCodeIntelSubscribe(r, d)` dùng `RegisterStream` theo SOL 2.3: bước 1–7 (client nil => `UNAVAILABLE`; `DeviceID` => `NOT_AUTHORIZED`; decode `{selectors?}` bằng `decodeCodeIntelArgs` với `MaxArgsBytes 16 KiB`, ≤ 50 selector, mỗi selector hợp lệ; `tryAcquire` thất bại => `CODEINTEL_RATE_LIMITED: too many streams | {"scope":"replica"}`; `subCtx` dẫn xuất từ ctx kết nối; `AttachIdentity` có `Role`; mở stream; `Header()` chờ ≤ 3 s trong goroutine, lỗi trả qua `codeIntelChannelError` và huỷ/`release`; hết 3 s coi là mở).
4. Goroutine `Recv` chỉ khung xương ở task này: đọc `CodeIntelPush` và chuyển cho `translateCodeIntelPush` (TASK-040-21; ở task này hàm giữ chỗ trả `nil, false`), `select {out<-ev | subCtx.Done()}`; `defer close(out)`, `defer release()`, `defer registry.clear`.
5. Thay thế: gọi `registry.replace(cancel)` trước khi mở stream mới; huỷ stream cũ (goroutine cũ thoát, `release` chạy).

## Kiểm thử

- Fake `StreamCodeIntelEvents` (stream giả có kênh điều khiển): ack thành công; metadata có `x-orca-role`; `selectors` 51 => `INVALID_PARAMS`; selector thiếu `projectId`; `DeviceID` => `NOT_AUTHORIZED`; client nil => `UNAVAILABLE`.
- Lỗi `PermissionDenied` trả trước header => lỗi ở ack (không ack thành công); server không gửi header trong 3 s => ack thành công.
- Thay thế: subscribe lần hai huỷ ctx stream đầu (fake thấy `ctx.Done`), bộ đếm giữ 1.
- Giới hạn: `MaxStreams=2`, mở 3 trên 3 kết nối => lần ba `RATE_LIMITED`; đóng một => mở lại được.
- Rò goroutine: sau đóng kết nối/huỷ, số goroutine về mức đầu (dung sai nhỏ).
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelStream(Open|Registry|Limit)' -race`.

## Tiêu chí hoàn thành

- [ ] Một subscribe/kết nối; lần hai thay lần đầu không rò.
- [ ] Giới hạn luồng thi hành; bộ đếm luôn về 0.
- [ ] Lỗi quyền/cờ thấy ở ack khi service gửi header sớm.

## Rủi ro và lưu ý

- Hành vi `Header()` của grpc-go chưa chạy thử ở repo; viết test bằng server gRPC thật trong test (bufconn) để chứng minh.
- Gateway không tự mở lại stream: FE mở lại sau `resync` (backoff 1–30 s).

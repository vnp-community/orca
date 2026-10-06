# BE-CV-TASK-040-21: Dịch khung push (5 loại), whitelist, giới hạn 1 KiB và khung `resync`

**From Solution:** BE-CV-SOL-040-codeintel-write-and-stream-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_stream_events.go` (mới), `channels_codeintel_stream_events_test.go` (mới)
**Depends on:** TASK-040-20
**Status:** [ ] TODO

---

## Context

UI-API §5: khung native `{"type":"push","channel":<tên>,"args":[obj]}` với `channel` ∈ `codeIntel.changed|reindexProgress|quality.progress|quality.finished|quality.gateChanged`; session-client chỉ nhận `obj` (`pushEventResult`, `push_bridge.go`) nên `obj.event` bắt buộc. Payload: `PushBase{event, projectId, worktreeId, occurredAt}` + `PushChanged{tools[], commit?, headCommit?, indexedAt?, stale, indexScope?, freshness?, reason, resync?}`, `PushReindexProgress{jobId, stage, percent|null, message, status?}`, `PushQualityProgress{runId, stage, stepIndex?, stepCount?, percent|null, message}`, `PushQualityFinished{runId, status succeeded|failed|cancelled|interrupted, summary, headCommit}`, `PushGateChanged{headCommit, previousVerdict|null, verdict, profile}`. Không đồ thị, mã nguồn, đường dẫn tuyệt đối, `tenantId`; ≤ 1 KiB. `CodeIntelPush` (proto-and-data-map 2.3): `kind`, ..., `percent optional`, `payload_json ≤ 8 KiB`.

## Việc cần làm

1. `translateCodeIntelPush(p *codeintelv1.CodeIntelPush) (PushEvent, bool)`: bảng `kind` => (`Channel`, `event`) như SOL 2.3; `kind` lạ => `false` (tăng bộ đếm nội bộ; không phát).
2. Dựng object bằng **struct có thẻ camelCase** (không map tự do): `occurredAt` RFC 3339; `percent` là `*int` (`null` khi vắng); `tools` luôn mảng; `projectId` từ proto, `worktreeId` từ `worktree_id`.
3. Quality: giải mã `payload_json` vào struct whitelist (`runId, stage, stepIndex, stepCount, percent, message` / `status, summary{error,warning,info,stepsTotal,...}, headCommit` / `previousVerdict, verdict, profile, headCommit`); khoá lạ bị bỏ; JSON hỏng => bỏ khung + đếm; giá trị enum lạ (`status`, `verdict`) giữ chuỗi (client chuyển `unknown`).
4. Kiểm cỡ: sau marshal > 1024 byte => cắt `message` (rune) đến khi vừa; vẫn vượt => bỏ khung + đếm. Che đường dẫn tuyệt đối trong `message`/`stage` bằng bộ che của TASK-040-04.
5. `resync`: hàm `codeIntelResyncEvent(now)` => `PushEvent{Channel:"codeIntel.changed", Args:[{event:"changed", projectId:"", worktreeId:"", occurredAt, tools:[], stale:true, reason:"resync", resync:true}]}`; goroutine `Recv` phát đúng một khung này khi `Recv` lỗi mà `subCtx` chưa huỷ, rồi đóng `out`.
6. Nối vào goroutine của TASK-040-20.

## Kiểm thử

- Bảng 5 loại `kind` => đúng `Channel`/`event`; `percent` vắng => `null`; `tools` nil => `[]`.
- `payload_json` có khoá lạ/`filePath` tuyệt đối => bị bỏ/che; `message` 5 KiB => khung ≤ 1 KiB; `payload_json` hỏng => không phát.
- Quét khung không chứa `tenantId`, `devServerId`, `/home/...`.
- Dialect: native `type:"push"` đúng tên kênh; session-client nhận `streaming:true` với `result.event`; khi stream đóng nhận `{"type":"end"}` (qua `pipePushForDialect`).
- Resync: giả lập `Recv` lỗi => đúng một khung `resync:true` rồi đóng; huỷ ctx (thay thế) => không khung resync.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelStreamEvents' -race`.

## Tiêu chí hoàn thành

- [ ] 5 kênh push đúng tên/`event`; payload whitelist ≤ 1 KiB.
- [ ] Đúng một `resync` khi service đứt.
- [ ] Không rò dữ liệu/tenant/đường dẫn.

## Rủi ro và lưu ý

- Khoá `payload_json` (camelCase) chưa chốt (Q3 của solution); parser chấp nhận cả `run_id` và `runId` cho tới khi chốt, ghi vào PR.
- Khung `quality.*` chỉ có ý nghĩa khi `quality_gate_enabled`; service quyết việc phát.

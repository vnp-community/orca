# BE-CV-SOL-024: Phân phối sự kiện: ingest từ infra-fleet, huỷ cache, outbox, NATS giữa replica, gRPC stream tới gateway

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Cần SOL-023 (`StreamCodeIntelEvents`), SOL-022 (`InvalidateBinding/Probe`), SOL-021 (`Watch`, `ReindexStatus`, `reindex_jobs`), BE-CV-SOL-011 (outbox, `processed_events`), BE-CV-SOL-012 (binding), BE-CV-SOL-013 (quyền theo selector). Số field proto ngoài hợp đồng là đề xuất.

**CR:** [CR-CV-024](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-024-event-distribution.md)
**Service:** `code-intel-service` (`internal/usecase`, `internal/adapter/{infrafleetclient,eventbus,broadcaster,grpc}`, `cmd/server/main.go`) · `backend-go/proto/orca/codeintel/v1/codeintel_events.proto`
**TDD tham chiếu:** [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) ("Event conventions (NATS JetStream)", dedup theo event ID), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) ("Transactional outbox + async events"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (backpressure stream: bộ đệm có giới hạn, drop/slow-consumer), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (cô lập tenant, quyền từng worktree), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md)
**Hợp đồng:** `CONTRACT-codeintel-proto-and-data-map.md` (PQ-04, PQ-11, PQ-16, PQ-17, PQ-24; §2.1 hàng 7, §2.3, §3.1 `StreamCodeIntelEvents`, §4 T0/T2/T7, §5, §6.1), `CONTRACT-codeintel-ui-api.md` §5, agent contract §6

---

## 0. Hợp đồng áp dụng

| Phán quyết / mục | Áp dụng |
|---|---|
| PQ-11 | RPC `StreamCodeIntelEvents(StreamCodeIntelEventsRequest{selectors 0..50}) returns (stream CodeIntelPush)`; rỗng = mọi worktree người dùng đọc được (lọc từng sự kiện bằng quyền `read`, cache 10 s); `kind ∈ changed\|reindex_progress\|quality_progress\|quality_finished\|quality_gate_changed` |
| PQ-04 | Đăng ký theo `selector{project_id, worktree_ref}` (không `worktree_ids`); push mang `worktree_id` + `project_id`, `repo_binding_id` chỉ trong payload nội bộ |
| PQ-16/17 | `percent` là `optional int32`; `reindex_progress` đi trực tiếp (không outbox/NATS); `quality_*` thêm `workspace_root` định tuyến; `payload_json` ≤ 8 KiB đã lọc |
| PQ-24 | Cờ tenant tắt → bỏ sự kiện/push; `codeintel.indexChanged` vẫn được **xử lý** (huỷ cache) ngay cả khi job đang chạy lúc tắt cờ |
| §2.3 | `CodeIntelPush` 20 field nguyên văn |
| §5 | Subject `orca.codeintel.index.changed`, `reindex.finished` (+ `quality.*`, `agent_turn.recorded` do SOL khác); stream `CODEINTEL` (`orca.codeintel.>`), consumer tạm `SubscribeEphemeral` mỗi replica; `event_id` UUID v5 của `tenant\|binding\|tools\|commit\|indexedAt\|kind`; gộp 2 s (tối đa 10 s); consumer **lọc** subject không thuộc mình |
| §4 T0/T2/T7 | `outbox_events`, `processed_events (tenant_id,event_id)` (dọn 7 ngày); ghép `(tenant, dev_server_id, path_hash)`; `reindex_jobs` |
| ui-api §5 | Đứt stream → gateway phát một `changed{resync:true}` rồi đóng; client mở lại |
| §8.3 (3)(4) | Test hai dialect (`processed_events`/outbox); mọi truy vấn có `tenant_id`; test cô lập tenant |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: ba CONTRACT; CR-CV-024; `common/eventbus/eventbus.go` (`Event`, `Publisher.EnsureStream`, `Consumer.Subscribe`, `SubscribeEphemeral` dòng 196, `consumeUntilDone`: lỗi handler → `Nak`), `common/outbox/outbox.go` (`Store.FetchUnpublished/MarkPublished`, `Relay`, mỗi replica chạy relay được); `notification-service/internal/usecase/ports.go` (dòng ~95–125 `ProcessedEventRepository.MarkProcessed`, ghi chú "atomic INSERT … ON CONFLICT DO NOTHING"), `notification-service/internal/adapter/broadcaster/` (thư mục có); `api-gateway/.../wscompat/{push_bridge.go (StreamHandler, pipePush kết thúc khi kênh đóng), channels_push.go (registerNotificationStreamChannel, registerWorkspacePortsStreamChannel dòng 50–116: `Recv` lỗi → đóng kênh, không mở lại)}`; `infra-fleet-service/internal/adapter/portevents/broadcaster.go` (mẫu "drop on full"); SOL-023 của feature này.

Xác nhận: mọi push UI hiện có là gRPC stream từ service sở hữu; gateway không có consumer NATS cho UI; **gateway không tự mở lại luồng khi `Recv` lỗi** (rủi ro CR mục 6 đúng); `SubscribeEphemeral` cho mỗi replica bản sao riêng của mọi message.

### Correction relative to CR-CV-024 (Lệch giữa CR và hợp đồng)

| # | CR nói | Hợp đồng | Xử lý |
|---|--------|----------|-------|
| C1 | `StreamCodeIntelEventsRequest{worktree_ids}` (1..50, bắt buộc) | PQ-11: `selectors` 0..50 (rỗng hợp lệ) | Theo hợp đồng |
| C2 | `CodeIntelPush` 13 field, `percent int32`, `kind` 2 giá trị | §2.3: 20 field, `optional percent`, 5 `kind` | Theo hợp đồng; file `codeintel_events.proto` |
| C3 | Tên proto trong `code_intel_service.proto` | PQ-07 | `codeintel_events.proto`; rpc trong `codeintel.proto` |
| C4 | Chỉ `index_changed`/`reindex_progress` | PQ-17: thêm `quality_progress/finished`; `quality_gate_changed` do SOL-085 phát qua NATS | Ingest chuyển tiếp; `quality_finished` còn gọi `QualityRunEventSink` (SOL-082) |
| C5 | `reason ∈ index_changed\|reindex_finished\|resync\|overflow` | §2.3: thêm `head_changed`; agent `reason ∈ index\|head\|reindex` | Bảng ánh xạ 2.C (hợp đồng không nêu; ghi báo cáo) |
| C6 | Payload NATS không định nghĩa | §5 định nghĩa payload `index.changed`, `reindex.finished` | Theo §5 |
| C7 | "stage kết thúc" chưa định (Q3) | Agent §6.2: có `state ∈ queued\|running\|succeeded\|failed\|cancelled` | Kết thúc theo `state`; `state` đọc từ `payload_json` (infra-fleet không có field `state`, SOL-023 Q1) |
| C8 | `stream_reconcile` đọc bindings xuyên tenant "kiểu `ListAllForPolling`" | Hợp đồng không liệt kê phương thức | Cổng `BindingStreamTargets` do SOL-011 cài (`withMaintenanceTx`), ghi báo cáo |
| C9 | Sau `resync` chỉ xoá bộ nhớ | agent §6: thông báo chỉ tới sau khi backend gọi một method `codeintel.*` | **Thêm**: sau mỗi mở luồng/`resync` gọi `Watch(true)` + `Status` cho từng binding |
| C10 | Mọi replica nhận sự kiện hai đường (stream + NATS) | — | Push chỉ đi qua NATS (một đường); LRU `event_id` chống giao lặp |

## 2. Giải pháp

### A. Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_events.proto          StreamCodeIntelEventsRequest, CodeIntelPush
backend-go/proto/orca/codeintel/v1/codeintel.proto                 (sửa: rpc StreamCodeIntelEvents)
services/code-intel-service/internal/
  usecase/code_intel_stream_supervisor.go    đối chiếu (tenant, dev server) <-> luồng đang mở
  usecase/index_event_coalescer.go           gộp 2 s / tối đa 10 s, token bucket, throttle tiến trình
  usecase/handle_agent_code_intel_event.go   xử lý theo kind
  usecase/index_event_identity.go            event_id UUID v5
  usecase/push_authorization.go              lọc quyền theo selector (cache 10 s)
  usecase/binding_stream_targets.go          cổng BindingStreamTargets, QualityRunEventSink
  adapter/infrafleetclient/code_intel_event_stream.go   mở/đọc/tái kết nối
  adapter/eventbus/code_intel_consumer.go    SubscribeEphemeral "CODEINTEL"
  adapter/broadcaster/code_intel_push_broadcaster.go
  adapter/grpc/server_code_intel_events.go
  cmd/server/main.go                         (sửa: EnsureStream CODEINTEL, supervisor, consumer)
```

### B. Ingest (giai đoạn A)

`Supervisor` đối chiếu mỗi `CODEINTEL_STREAM_RECONCILE` (60 s) và khi SOL-012 báo binding mới/xoá: tập `(tenant_id, dev_server_id)` từ `BindingStreamTargets` (đọc xuyên tenant) so với luồng mở; mở thiếu, đóng thừa; tenant tắt cờ → đóng ở lần sau. **Mọi replica mở mọi luồng** (trạng thái bộ nhớ phải cập nhật trên mọi replica; Q2). Mỗi luồng gắn `grpcmw.MetadataTenantID`. Tái kết nối: backoff `1,2,5,15,30 s` giữ 30 s, ± 20 % jitter; reset khi nhận tin đầu hoặc sống ≥ 10 s; `Unauthenticated/NotFound` → dừng hẳn. Mỗi lần mở lại thành công: coi như `resync` cục bộ (xoá `HeadProbe` + LRU `SYMBOL` mọi binding của dev server, **không xoá** snapshot DB: SOL-022 phát hiện qua stale B), đẩy `changed{reason:"resync"}`, rồi **gọi `Watch(true)` + `Status`** từng binding (C9). Tắt êm bằng `WaitGroup`.

Ghép sự kiện → binding: `(tenant, dev_server_id, path_hash(workspace_root))` (T2). Không binding → bỏ, đếm `events_unmatched_total`. Nếu index dùng chung cho worktree liên kết (`index_scope=repo_root`) thì `workspace_root` của sự kiện có thể là checkout chính: huỷ cache mọi binding cùng `repo_id` (quy tắc cuối thuộc SOL-012/080, Q1).

### C. Xử lý và idempotent

| `kind` | Xử lý |
|---|---|
| `index_changed` | (1) tra binding; (2) **cục bộ mọi replica**: `InvalidateProbe`; (3) **một lần**: giao dịch `processed_events` (xung đột → bỏ) + `InvalidateBinding` + `outbox_events` `orca.codeintel.index.changed` (payload §5; `reason`: agent `index`→`index_changed`, `head`→`head_changed`, `reindex`→`reindex_finished`; `tool:"git"` chỉ cập nhật `stale`, không huỷ snapshot, không `analyze`); (4) phát push qua đường NATS (2.D) |
| `reindex_progress` | cập nhật `reindex_jobs` (`stage`, `percent`, `message`; ≤ 1 lần/2 s trừ đổi `stage`/kết thúc); push **trực tiếp** (≤ 2 tin/giây/job + tin đổi `stage` + tin kết thúc), không outbox; khi `state ∈ succeeded\|failed\|cancelled`: giao dịch `processed_events` + `Finish` + `InvalidateBinding` + outbox `orca.codeintel.reindex.finished` (`interrupted`→`failed`/`CODEINTEL_REINDEX_INTERRUPTED`) |
| `quality_progress` | push trực tiếp (≤ 1/giây/run, không outbox) |
| `quality_finished` | push trực tiếp **và** gọi `QualityRunEventSink.OnFinished` (SOL-082 nạp findings rồi `Finish` run, phát `quality.run_finished`) |
| `resync`, `overflow` | như tái kết nối: xoá bộ nhớ, `changed{reason}` cho mọi binding của dev server |

`event_id` xác định: UUID v5 (`github.com/google/uuid`) của `tenant|binding|tools|commit|indexedAt-lớn-nhất|kind`. `processed_events`: PG `INSERT … ON CONFLICT DO NOTHING`, MySQL `INSERT IGNORE`/`ON DUPLICATE KEY UPDATE`; hai bước DB trong **một** giao dịch. Giai đoạn B không dùng bảng chung (mỗi replica phát cho luồng gateway của nó): LRU 1 024 `event_id`.

### D. Chống bão và phân phối

Gộp theo `(tenant, binding)`: huỷ cache ngay ở tin đầu, outbox/push sau 2 s yên lặng (`CODEINTEL_EVENT_DEBOUNCE`), tối đa 10 s (`CODEINTEL_EVENT_MAX_WAIT`); sự kiện gộp mang commit/indexedAt mới nhất và hợp `tools`. Token bucket 20 sự kiện/giây mỗi dev server (vượt → một `resync`); hàng đợi ingest 256 (đầy → `resync`, `events_dropped_total{reason="queue_full"}`); bộ đệm mỗi người đăng ký push 64 (đầy → bỏ, thay bằng một `changed{reason:"overflow"}`, `Publish` không chặn). Phát giữa replica: outbox → `outbox.Relay` → NATS `CODEINTEL` → `SubscribeEphemeral(ctx,"CODEINTEL","orca.codeintel.>",…)` **mỗi replica** → broadcaster cục bộ; consumer lọc subject không thuộc mình (`review.saved`…), bỏ sự kiện thiếu `TenantID`.

### E. RPC tới gateway

```proto
// codeintel_events.proto (số 1..13 theo hợp đồng §2.3 / CR §2.6; 14..20 hợp đồng)
message StreamCodeIntelEventsRequest { repeated WorktreeSelector selectors = 1; }   // 0..50
message CodeIntelPush { /* nguyên văn hợp đồng §2.3 */ }
```
Handler `StreamCodeIntelEvents` (server-stream): lấy identity từ metadata bằng hàm tương đương `withTenantFromStreamMetadata` (`ChainUnary` chỉ gắn cho RPC đơn; bản sao viết lại trong service); cờ `code_intel_enabled`; mỗi `selector` kiểm quyền `read` (`PushAuthorization`) — selector không được phép → `PermissionDenied` toàn yêu cầu (không lộ id hợp lệ); selectors rỗng → lọc **từng sự kiện** bằng quyền (cache 10 s, lỗi tra cứu = từ chối); chỉ gửi `CodeIntelPush` thuộc worktree đã đăng ký; ≤ 1 KiB, không đồ thị/mã nguồn/đường dẫn tuyệt đối; `payload_json` quality chỉ chứa `summary`, `status`, `headCommit` (allowlist, ≤ 8 KiB).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Gateway nhận push bằng gRPC stream | Khớp `StreamNotifications`/`StreamPortForwardEvents`; gateway chưa có consumer NATS cho UI |
| Mọi replica mở mọi luồng infra-fleet | Trạng thái bộ nhớ (probe, LRU) phải cập nhật mọi replica |
| `index_changed` qua outbox + NATS, `*_progress` trực tiếp | Một đường push duy nhất cho thay đổi bền; tiến trình quá dày |
| `processed_events` chỉ ở giai đoạn ingest | Giai đoạn B cần mỗi replica phát cho luồng của nó |
| `resync` không xoá DB | SOL-022 tự phát hiện; tránh mất cache mỗi lần deploy |
| `Watch(true)` sau mỗi nối lại | Agent chỉ gửi thông báo tới notifier của lần gọi gần nhất |

## 4. Phụ thuộc chéo khu vực

| Hướng | Solution | Quan hệ |
|---|---|---|
| Trước | `BE-CV-SOL-023` (stream), `BE-CV-SOL-022` (invalidate), `BE-CV-SOL-021` (`Watch`, `reindex_jobs`), `BE-CV-SOL-011-*`, `BE-CV-SOL-012-target-resolution-and-bindings`, `BE-CV-SOL-013-authorization-flags-and-audit` | |
| Sau | `BE-CV-SOL-040-codeintel-write-and-stream-channels` (`codeIntel.subscribe`, đóng + `resync:true` khi đứt), `BE-CV-SOL-082-quality-run-storage-and-ingest` (`QualityRunEventSink`), `BE-CV-SOL-085-*` (`gate_changed` qua cùng consumer), `BE-CV-SOL-080-auto-refresh-index`, `BE-CV-SOL-089` | |
| Agent | `AG-CV-SOL-004-reindex-and-index-notifications` (notification, `codeintel.watch`), `AG-CV-SOL-081-quality-runner-core` | tên/tham số theo agent contract §6 |
| Frontend | `FE-CV-SOL-050-store-and-query-hooks` (đăng ký một lần/kết nối, backoff 1→30 s, chip "Có dữ liệu mới") | ui-api §5 |

## 5. Kiểm thử

- **Unit (đồng hồ giả):** coalescer (cửa sổ, max-wait, hai công cụ), token bucket, backoff + jitter, `event_id` xác định, ánh xạ reason, throttle tiến trình, `PushAuthorization` (cache 10 s, lỗi = từ chối).
- **Tích hợp (`-tags=integration`, PG + MySQL + NATS testcontainer):** hai bản `code-intel-service` in-process chung DB/NATS + nguồn infra-fleet giả: **một** dòng `outbox_events`, huỷ cache + push ở **cả** hai replica, race `processed_events`, giao lặp NATS không làm UI nhận hai push; giết giữa chừng sau `processed_events` không để dòng mồ côi; 30 `index_changed`/1 s → một `changed` ≤ 2 s sau tin cuối; `reindex_progress` 100 tin/s → ≤ 2/s + tin đổi stage + kết thúc, `reindex_jobs` ≤ 1 ghi/2 s, một `reindex.finished`; tenant tắt cờ không sinh push; tenant A không nhận sự kiện của B.
- **gRPC in-process:** selector không có quyền → `PermissionDenied`; bộ đệm 64 đầy → đúng một `overflow`; huỷ ctx giải phóng người đăng ký (`goleak`/đếm).
- **Bão:** 10 000 sự kiện/10 s: bộ nhớ và goroutine ổn định.
- Chưa chạy test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Gateway không tự mở lại luồng**: nếu code-intel-service khởi động lại, UI ngừng nhận push tới khi client đăng ký lại (SOL-040/FE-050 phải xử lý `resync`); chưa kiểm hành vi thật.
- N replica × M dev server goroutine chưa đo (Q2); NATS không sẵn → outbox vẫn bền, replica khác dựa vào thăm dò (suy giảm, không sai dữ liệu).
- Quy tắc ghép `workspaceRoot` với binding khi chỉ mục dùng chung kho chính chưa chốt (Q1).
- `processed_events` MySQL cần cách đếm hàng bị bỏ qua đúng với driver (notification-service có `MarkProcessed`, chưa đọc cài đặt MySQL).
- Cửa sổ gộp 2 s + thăm dò 15 s: UI có thể thấy dữ liệu cũ vài giây (chấp nhận).

## 7. Câu hỏi mở

- **Q1.** Quy tắc ghép `event.workspace_root` → binding (chính xác hay gốc kho dùng chung).
- **Q2.** Mọi replica mở mọi luồng hay phân mảnh theo hash `devServerID`.
- **Q3.** Hợp đồng cần ánh xạ `reason` agent → push (C5) và field `state` (SOL-023 Q1).
- **Q4.** `orca.codeintel.index.changed` có vào `notification-service` không (hiện không).

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-11/16/17/24; §2.3, §4, §5); agent contract §6; ui-api §5
- `/opt/repos/orca/docs/crs/v7/code-intel-graph-pipeline/CR-CV-024-event-distribution.md`
- `/opt/repos/orca/backend-go/common/{eventbus/eventbus.go,outbox/outbox.go}`; `services/notification-service/internal/{usecase/ports.go,adapter/broadcaster}`; `services/api-gateway/internal/adapter/wscompat/{push_bridge.go,channels_push.go}`; `services/infra-fleet-service/internal/adapter/portevents/broadcaster.go`

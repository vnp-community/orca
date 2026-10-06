# CR-CV-024 — Phân phối sự kiện: huỷ cache, outbox, đẩy lên gateway

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-024 |
| **Tên** | `code-intel-service` subscribe `StreamCodeIntelEvents` của infra-fleet, huỷ cache, ghi `outbox_events`, phát cho `api-gateway` qua gRPC stream, tái kết nối, chống bão sự kiện, idempotent |
| **Loại** | Feature (consumer sự kiện + phát push) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-023 (`StreamCodeIntelEvents` của infra-fleet), CR-CV-022 (`InvalidateBinding`, thăm dò HEAD), CR-CV-011 (`outbox_events`, `processed_events`, `reindex_jobs`), CR-CV-012 (binding), CR-CV-010 (relay outbox, NATS) |
| **Mở khoá** | CR-CV-040 (kênh push `codeIntel.changed`, `codeIntel.reindexProgress`), CR-CV-050 (slice nhận push), CR-CV-051 (chip "index đã đổi") |
| **Tác động** | `backend-go/proto/orca/codeintel/v1/code_intel_service.proto` (RPC `StreamCodeIntelEvents` + message), `code-intel-service/internal/{usecase,adapter/infrafleetclient,adapter/eventbus,adapter/broadcaster,adapter/grpc,config}` |

---

## 1. Bối cảnh và vấn đề

1. D4 (README v7): agent chủ động báo `codeintel.indexChanged`/`codeintel.reindexProgress`; backend huỷ cache và báo UI; không đẩy cả graph. CR-CV-023 đưa thông báo ra khỏi agent thành `InfraFleetService.StreamCodeIntelEvents`. Còn thiếu đoạn từ infra-fleet tới màn hình.
2. **Cách `api-gateway` nhận push từ service khác** (đã đọc):
   - `wscompat.StreamHandler` trả `<-chan PushEvent`, `pipePush` ghi từng sự kiện thành khung `push` (`api-gateway/internal/adapter/wscompat/push_bridge.go:18-52`).
   - Các nguồn push hiện có đều là **gRPC server-streaming từ service sở hữu**: `notifications.subscribe` ← `notification-service.StreamNotifications` (`channels_push.go`, `registerNotificationStreamChannel`), `workspacePorts.subscribe` ← `infra-fleet-service.StreamPortForwardEvents` (`registerWorkspacePortsStreamChannel`, `server.go:985-1001`), cộng `ClientEventBus` cục bộ trong tiến trình (`channels_push.go:146-190`, "deliberately NOT cross-replica").
   - NATS trong `api-gateway` chỉ xuất hiện ở phần MCP (`mcpsession/jetstream_event_store.go`, `cmd/server/mcp_sessions_wiring.go`, `internal/config/config.go`); không có consumer NATS nào đẩy push cho UI.
   - Cách phân phối giữa replica ở `notification-service`: service **tự** subscribe NATS bằng consumer **tạm** (`SubscribeEphemeral`, `adapter/eventbus/consumer.go` đầu file, "every replica independently translates and broadcasts… cluster-wide fan-out") rồi phát vào `adapter/broadcaster` cục bộ cho các luồng `StreamNotifications` của chính replica đó.
3. Hai nhu cầu khác nhau cần phân biệt: (a) **huỷ cache dùng chung** (bảng `graph_snapshots` chung DB, chỉ cần làm một lần) và (b) **trạng thái trong bộ nhớ từng replica** (thăm dò HEAD, LRU `SYMBOL` của CR-CV-022) và **luồng gRPC tới gateway** (gắn với đúng một replica) — cần mọi replica đều biết.
4. Chưa có `code-intel-service`; chưa có kênh `codeIntel.*`; không có hạ tầng nào để chống bão sự kiện cho index (reindex Orca mất nhiều phút, research 02 mục 4).

## 2. Giải pháp đề xuất

### 2.1 Quyết định: gateway nhận push qua gRPC stream từ `code-intel-service`

Theo mẫu `StreamNotifications`/`StreamPortForwardEvents` (mục 1 điểm 2):

```
agent ─indexChanged/reindexProgress─▶ infra-fleet ─StreamCodeIntelEvents─▶ code-intel-service (mọi replica)
                                                                              │ A. ingest: huỷ cache + outbox_events (một lần, dedupe bằng processed_events)
                                                                              ▼
                                                           outbox relay ─NATS "CODEINTEL"─▶ mọi replica (consumer tạm)
                                                                              │ B. broadcaster cục bộ
                                                                              ▼
api-gateway ◀─CodeIntelService.StreamCodeIntelEvents (gRPC stream)─ replica mà gateway nối tới ◀─┘
   │ wscompat RegisterStream "codeIntel.subscribe" → push "codeIntel.changed" / "codeIntel.reindexProgress"
   ▼ UI
```

Lý do chọn gRPC stream, không chọn "gateway tự subscribe NATS": theo đúng các push hiện hữu; không thêm phụ thuộc NATS và logic phân quyền theo user vào gateway (gateway hiện không subscribe NATS cho UI); phân quyền theo worktree nằm ở nơi sở hữu dữ liệu. Outbox/NATS vẫn dùng **giữa các replica của `code-intel-service`** và để audit/consumer tương lai (ví dụ `notification-service`).

### 2.2 Giai đoạn A: ingest từ infra-fleet

**Thành phần (mới, tên theo khái niệm):**

| File | Vai trò |
|---|---|
| `internal/adapter/infrafleetclient/code_intel_event_stream.go` | mở và đọc `StreamCodeIntelEvents` cho một `(tenant, dev server)`; tái kết nối (2.3) |
| `internal/usecase/code_intel_stream_supervisor.go` | đối chiếu định kỳ tập `(tenant, dev server)` cần theo dõi với tập luồng đang mở |
| `internal/usecase/index_event_coalescer.go` | gộp/debounce (2.5) |
| `internal/usecase/handle_agent_code_intel_event.go` | xử lý một sự kiện đã gộp (2.4) |

**Giám sát luồng.** Tập cần theo dõi = các `(tenant_id, dev_server_id)` phân biệt trong `repo_bindings` (CR-CV-011/012; cần thêm phương thức đọc xuyên tenant, kiểu `ListAllForPolling` của infra-fleet `usecase/ports.go:50-53`, vì đây không phục vụ một yêu cầu của người dùng). Giám sát chạy khi khởi động, mỗi 60 s (`CODEINTEL_STREAM_RECONCILE`), và khi CR-CV-012 báo binding mới/xoá; mở thêm luồng còn thiếu, đóng luồng thừa. Mỗi luồng gắn `tenant` bằng metadata `grpcmw.MetadataTenantID` (cùng cơ chế `withTenantMetadata` của CR-CV-021). Mọi replica đều mở **tất cả** luồng (cần cho trạng thái trong bộ nhớ, mục 1 điểm 3); chi phí là một goroutine mỗi dev server mỗi replica (Q2).

### 2.3 Tái kết nối

`code_intel_event_stream.go`:

- Vòng lặp: mở luồng → `Recv` đến khi lỗi hoặc EOF → chờ backoff → mở lại. Backoff `1, 2, 5, 15, 30 s` rồi giữ 30 s, cộng jitter ±20 % (khớp lịch reconnect của agent, research 01 mục 3). Thành công (nhận được tin đầu tiên hoặc luồng sống ≥ 10 s) thì đặt lại về 1 s.
- `Unauthenticated`/`NotFound` (dev server đã xoá hoặc binding hết tenant) → **dừng hẳn** luồng đó (giám sát mở lại nếu binding còn).
- Mỗi lần mở lại thành công, sự kiện đã mất trong lúc luồng đứt không thể khôi phục. Xử lý `resync` cục bộ (2.4, loại "resync"): xoá thăm dò HEAD và LRU `SYMBOL` của mọi binding của dev server, và đẩy `changed(reason=resync)` cho UI. **Không** xoá snapshot trong DB: CR-CV-022 mục 2.3 (B) phát hiện chỉ mục đổi qua `indexedAt`/phiên bản trong thăm dò ở lần đọc kế tiếp, nên không cần xoá hàng loạt (tránh làm mất cache của mọi tenant mỗi lần triển khai lại). CR-CV-023 cũng phát `resync` khi agent nối lại infra-fleet; xử lý như nhau.
- Đóng gọn khi tắt tiến trình: huỷ `ctx` gốc, chờ `WaitGroup` (như `relayWG` ở `infra-fleet-service/cmd/server/main.go:~273-282`).

### 2.4 Xử lý sự kiện và idempotent

Sự kiện vào từ luồng là `CodeIntelEvent` (CR-CV-023 mục 2.4). Đi qua bộ gộp (2.5), rồi `HandleAgentCodeIntelEvent`:

| `kind` | Xử lý |
|---|---|
| `index_changed` | (1) tra binding: `dev_server_id` + `workspace_root == event.workspace_root` (hoặc đường dẫn gốc của `gitnexus_repo` đã đăng ký; quy tắc đầy đủ thuộc CR-CV-012, Q1); không có binding → bỏ, đếm `events_unmatched_total`. (2) **Cục bộ, luôn làm trên mọi replica:** xoá thăm dò HEAD và LRU `SYMBOL` của binding. (3) **Dùng chung, làm một lần:** giao dịch DB: `processed_events` chèn `event_id` (xung đột → đã có replica khác xử lý → bỏ bước còn lại); `SnapshotInvalidator.InvalidateBinding` (CR-CV-022 mục 2.7); chèn `outbox_events` subject `orca.codeintel.index.changed`. (4) Phát push cục bộ (2.6, qua đường NATS để mọi replica phát). |
| `reindex_progress` | Cập nhật `reindex_jobs` theo `job_id` (stage, `finished_at`, `error_code`) có giới hạn tần suất (2.5); **không** ghi outbox cho mỗi lần; phát push cục bộ **trực tiếp** (không qua NATS, quá dày). Khi `stage` kết thúc (giá trị cụ thể do CR-CV-004 định nghĩa, Q3): giao dịch `processed_events` + `InvalidateBinding` + `outbox_events` `orca.codeintel.reindex.finished` (`reindex.started` do use case `RequestReindex` ghi, ngoài CR này). |
| `resync`, `overflow` | Như đoạn "tái kết nối" (2.3): xoá thăm dò/LRU, **không** xoá DB; đẩy `changed(reason)` cho mọi binding của dev server. |

**Idempotent.** `event_id` xác định, không ngẫu nhiên: UUID v5 (SHA-1, `github.com/google/uuid`, đã dùng ở infra-fleet) của chuỗi chuẩn tắc `tenant|binding_id|tool-set|commit|indexedAt-lớn-nhất|kind`. Cùng một thay đổi chỉ mục do hai replica (hoặc giao lặp) cho cùng `event_id`; `processed_events (event_id PK, subject, processed_at)` (mẫu `notification-service` `MarkProcessed`: `INSERT … ON CONFLICT DO NOTHING`, "not a racy check-then-insert", `usecase/ports.go:103-118`; MySQL dùng `INSERT IGNORE`/`ON DUPLICATE KEY UPDATE`) trả `alreadyProcessed`. Dọn `processed_events` cũ hơn 7 ngày bằng janitor (cùng quy ước CR-REQ-001 mục 2.5 của v6). Hai bước DB (3) chạy trong **một** giao dịch để crash giữa chừng không để `processed_events` đã ghi mà outbox chưa ghi.

**Giai đoạn B không dùng `processed_events` DB.** Mỗi replica phải phát cho luồng gateway của chính nó; nếu dùng bảng chung, chỉ replica nhanh nhất phát và các replica còn lại bỏ qua (phụ thuộc vào replica nào gateway nối). Thay vào đó mỗi replica giữ LRU trong bộ nhớ 1 024 `event_id` gần nhất để bỏ giao lặp NATS (at-least-once). Giao lặp còn sót chỉ làm UI gọi lại một lần, rẻ nhờ ETag (CR-CV-022).

### 2.5 Chống bão sự kiện

| Nguồn bão | Biện pháp |
|---|---|
| Nhiều `index_changed` liên tiếp (reindex nhiều bước, hai công cụ cùng đổi, agent ghi tệp) | **Gộp theo `(tenant, binding)`**: huỷ cache ngay ở sự kiện đầu (rẻ, idempotent), nhưng `outbox`/push chỉ phát sau cửa sổ **2 s** không có sự kiện mới (`CODEINTEL_EVENT_DEBOUNCE`), tối đa chờ **10 s** (`CODEINTEL_EVENT_MAX_WAIT`) để chuỗi liên tục vẫn phát ra; sự kiện gộp mang `commit`/`indexedAt` mới nhất và danh sách `tools` |
| `reindex_progress` dày (nhiều/giây) | Mỗi `job_id`: cho qua tin đầu, tin đổi `stage`, tin kết thúc; còn lại ≤ **2 tin/giây**; ghi `reindex_jobs` ≤ 1 lần/2 s trừ khi đổi `stage`/kết thúc |
| Dev server lỗi gửi liên tục | Token bucket **20 sự kiện/giây** mỗi dev server; vượt → gộp thành một `resync` cho dev server đó và bỏ phần còn lại trong giây |
| Người nghe chậm | Hàng đợi mỗi luồng ingest kích thước 256; đầy → thay bằng một `resync` (đếm `events_dropped_total{reason="queue_full"}`) |
| Người đăng ký push chậm (gateway/UI) | Broadcaster mỗi người đăng ký có bộ đệm 64; đầy → bỏ bộ đệm, thay bằng một `changed(reason=overflow)` (UI tải lại, ETag giữ chi phí thấp) |
| UI nhận bão push | Push không mang dữ liệu đồ thị (D4); UI có throttle refetch (CR-CV-050/051); gateway không gộp (không có ngữ cảnh) |

### 2.6 Phát tới gateway

**Proto** (`code_intel_service.proto`, thuộc RPC của README v7 mục 3.6; CR này định nghĩa message của RPC này):

```proto
rpc StreamCodeIntelEvents(StreamCodeIntelEventsRequest) returns (stream CodeIntelPush);
message StreamCodeIntelEventsRequest { repeated string worktree_ids = 1; }  // bắt buộc, 1..50
message CodeIntelPush {
  string kind = 1;            // "changed" | "reindex_progress"
  string event_id = 2;
  string worktree_id = 3;
  string repo_binding_id = 4;
  string reason = 5;          // "index_changed" | "reindex_finished" | "resync" | "overflow"
  repeated string tools = 6;
  string commit = 7;
  string indexed_at = 8;
  string job_id = 9;
  string stage = 10;
  int32 percent = 11;
  string message = 12;
  google.protobuf.Timestamp occurred_at = 13;
}
```

**Phía service (mới):** `adapter/eventbus/code_intel_consumer.go` — `eventbus.Consumer.SubscribeEphemeral(ctx, "CODEINTEL", "orca.codeintel.>", handler)` (`common/eventbus/eventbus.go:196`, mỗi replica có con trỏ riêng); stream được tạo ở `main.go` bằng `pub.EnsureStream(ctx, "CODEINTEL", []string{"orca.codeintel.>"})` giống `INFRAFLEET` (`infra-fleet-service/cmd/server/main.go:~265`). `adapter/broadcaster/code_intel_push_broadcaster.go` giữ danh sách người đăng ký theo `(tenant, user)`; handler `StreamCodeIntelEvents` (trong `adapter/grpc`):

1. Lấy identity từ metadata (cần gọi hàm lấy tenant từ metadata cho stream giống `withTenantFromStreamMetadata` của infra-fleet, `server.go:1545`, vì `ChainUnary` chỉ gắn cho RPC đơn: `common/grpcmw/grpcmw.go:124-131`).
2. Kiểm quyền đọc từng `worktree_id` bằng bộ phân quyền của CR-CV-013; id không được phép → `PermissionDenied` toàn yêu cầu (không lộ id nào hợp lệ); đăng ký người nghe chỉ cho các id còn lại.
3. Chuyển `CodeIntelPush` thuộc worktree đã đăng ký; không gửi dữ liệu mã nguồn/đồ thị.

**Phía gateway (CR-CV-040 làm):** `r.RegisterStream("codeIntel.subscribe", …)` mở `StreamCodeIntelEvents` với `AttachIdentity`, ánh xạ `kind="changed"` → `PushEvent{Channel:"codeIntel.changed"}`, `kind="reindex_progress"` → `codeIntel.reindexProgress` (đúng tên README v7 mục 3.7), `Args` một phần tử (mẫu `registerWorkspacePortsStreamChannel`). Tên kênh `codeIntel.subscribe` là đề xuất của CR này, chốt ở CR-CV-040.

### 2.7 Cờ tính năng và cô lập

Sự kiện thuộc tenant `code_intel_enabled=false` bị bỏ ở bước tra binding (không binding thì không có luồng; nếu tenant tắt giữa chừng, bỏ khi xử lý và dừng luồng ở lần đối chiếu kế). Mọi truy vấn DB lọc `tenant_id`; Postgres RLS cho `processed_events`/`outbox_events` như CR-REQ-001. Payload NATS mang `tenant_id` trong `eventbus.Event.TenantID`; consumer bỏ sự kiện thiếu tenant.

### 2.8 Quan sát

`codeintel_events_received_total{kind}`, `codeintel_events_coalesced_total`, `codeintel_events_dropped_total{reason}`, `codeintel_events_unmatched_total`, `codeintel_event_stream_reconnects_total{reason}`, `codeintel_event_stream_connected{devserver}` (gauge, có giới hạn nhãn), `codeintel_event_pipeline_latency_seconds` (agent `indexedAt` đến push), `codeintel_push_subscribers`. Tên chốt ở CR-CV-071. Log không chứa `workspace_root` đầy đủ nếu chứa tên người dùng (che phần đầu).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Gateway nhận push bằng gRPC stream | Khớp `StreamNotifications`/`StreamPortForwardEvents`; gateway chưa có consumer NATS cho UI |
| Mọi replica mở mọi luồng infra-fleet | Trạng thái cục bộ (thăm dò HEAD, LRU) phải cập nhật trên mọi replica |
| Outbox + NATS ephemeral giữa các replica | Mỗi replica phát cho luồng gateway của riêng nó; outbox cho bền và audit |
| `processed_events` chỉ ở giai đoạn A | Bảng chung ở giai đoạn B sẽ làm replica chậm không phát được cho người nghe cục bộ |
| `event_id` xác định (UUID v5) | Hai replica cùng tính ra một id; dedupe không cần điều phối |
| `resync` không xoá snapshot DB | CR-CV-022 mục 2.3 (B) tự phát hiện; tránh mất cache mỗi lần deploy |
| Gộp 2 s / tối đa 10 s, huỷ cache ngay | Cache không bao giờ cũ hơn sự kiện đầu; push không bị bão |
| Push không mang dữ liệu | D4; kích thước nhỏ, ETag giữ chi phí tải lại thấp |
| Đăng ký theo `worktree_ids` bắt buộc | Tránh phát mọi worktree của tenant cho mọi người dùng; kiểm quyền từng id |

## 4. Tiêu chí chấp nhận

- [ ] Infra-fleet giả phát `index_changed` cho binding B: snapshot của B bị xoá, thăm dò HEAD và LRU `SYMBOL` của B bị xoá trên **mọi** replica (hai replica thử nghiệm), `outbox_events` có đúng **một** dòng `orca.codeintel.index.changed`, và người đăng ký push của B nhận đúng một `CodeIntelPush{kind:"changed"}` ở **mỗi** replica.
- [ ] Hai replica nhận cùng sự kiện đồng thời: `processed_events` có một dòng, `outbox_events` một dòng; giao lặp NATS không làm UI nhận hai push (LRU `event_id`).
- [ ] Gửi 30 `index_changed` trong 1 s cho một binding: cache huỷ ngay ở tin đầu; chỉ **một** `changed` phát sau ≤ 2 s kể từ tin cuối (hoặc ≤ 10 s nếu liên tục), mang `commit`/`indexedAt` mới nhất.
- [ ] `reindex_progress` 100 tin/giây: người nghe nhận ≤ 2/giây cộng tin đổi `stage` và tin kết thúc; `reindex_jobs` ghi ≤ 1 lần/2 s trừ khi đổi `stage`; không có `outbox_events` cho tin tiến trình; có một `reindex.finished` khi kết thúc.
- [ ] Luồng đứt: tái kết nối theo `1,2,5,15,30 s` (± jitter); sau khi nối lại, UI nhận `changed(reason=resync)`, snapshot DB **không** bị xoá, và lần đọc kế vẫn phát hiện chỉ mục mới qua thăm dò (CR-CV-022).
- [ ] Binding bị xoá: luồng của dev server đó đóng sau tối đa 60 s nếu không còn binding nào dùng.
- [ ] Người dùng không có quyền đọc một `worktree_id` trong yêu cầu: `PermissionDenied`, không nhận push nào của các id khác trong cùng yêu cầu.
- [ ] Người đăng ký chậm (không đọc): bộ đệm 64 bị thay bằng đúng một `changed(reason=overflow)`; `Publish` không bị chặn.
- [ ] Tenant `code_intel_enabled=false` không sinh sự kiện/push.
- [ ] Chạy ở Postgres và MySQL (`processed_events`, `outbox_events`, transaction); không dùng `RETURNING` ở đường MySQL.
- [ ] Không có tên file `helpers/utils/common/misc`; không `max-lines` disable mới.

## 5. Kiểm thử

- **Unit** (đồng hồ giả): bộ gộp (cửa sổ, max-wait, hai công cụ), token bucket, backoff, `event_id` xác định (cùng đầu vào → cùng UUID), ánh xạ sự kiện → push.
- **Tích hợp (`-tags=integration`, hai dialect + NATS testcontainer):** hai bản `code-intel-service` in-process cùng một DB và NATS, một nguồn infra-fleet giả; kiểm một dòng outbox, phát ở cả hai replica, race `processed_events`; giao dịch crash giữa chừng (giết sau `processed_events`) không để lại dòng mồ côi.
- **gRPC in-process:** `StreamCodeIntelEvents` quyền, bộ đệm đầy, huỷ ctx giải phóng người đăng ký (kiểm rò goroutine bằng `goleak` hoặc đếm).
- **Hồi quy bão:** 10 000 sự kiện giả trong 10 s: bộ nhớ và số goroutine ổn định, số push ≤ ngưỡng.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Gateway không tự mở lại luồng.** `pipePush` kết thúc khi kênh đóng (`push_bridge.go:38-52`) và chưa thấy vòng mở lại luồng stream ở `registerNotificationStreamChannel`/`registerWorkspacePortsStreamChannel` (đóng kênh khi `Recv` lỗi). Nếu `code-intel-service` khởi động lại, UI có thể ngừng nhận push cho đến khi đăng ký lại. CR-CV-040/050 cần: gateway đóng kết nối WS hoặc UI nhận tín hiệu và đăng ký lại, đồng thời refetch. Chưa kiểm hành vi thật.
- Mọi replica mở mọi luồng: N replica × M dev server goroutine; chưa đo. Nếu lớn, phân mảnh theo hash `devServerID` kèm kênh NATS để giữ cập nhật cục bộ cho replica không giữ luồng (Q2).
- `StreamCodeIntelEvents` của infra-fleet có thể mất `tenant` nếu bắt chước `StreamFileChanges` (CR-CV-023 mục 1 điểm 5); ở phía gọi, luôn gắn metadata tenant.
- Quy tắc ghép `workspaceRoot` của sự kiện với binding chưa chốt: GitNexus index theo kho (`<repo>/.gitnexus`), trong khi binding theo worktree (O4); nếu worktree dùng chung chỉ mục kho chính, một `index_changed` ở kho chính phải huỷ cache của mọi worktree của kho đó. Chưa kiểm chứng vị trí chỉ mục khi dùng `git worktree`.
- Cửa sổ gộp 2 s cộng thăm dò HEAD 15 s (CR-CV-022) nghĩa là UI có thể thấy dữ liệu cũ vài giây; chấp nhận.
- `stage` kết thúc của reindex chưa được định nghĩa (CR-CV-004); `reindex.finished` phụ thuộc điều này.
- NATS không sẵn: outbox vẫn ghi (bền), nhưng các replica không nhận NATS thì chỉ replica đang giữ luồng infra-fleet có thông tin; replica khác dựa vào thăm dò (CR-CV-022) để đúng đắn (suy giảm, không sai dữ liệu).
- `processed_events` MySQL cần `INSERT IGNORE`-tương đương; cần xác nhận cách đếm hàng bị bỏ qua với driver đang dùng (`notification-service/internal/adapter/mysql/repository.go` đã có `MarkProcessed`, chưa đọc).

## 7. Câu hỏi mở

- **Q1.** Quy tắc ghép `event.workspaceRoot` → binding (đường dẫn chính xác, hay gốc kho dùng chung mọi worktree) — chốt cùng CR-CV-012 và CR-CV-004.
- **Q2.** Mọi replica mở mọi luồng hay phân mảnh? Cần số liệu số dev server và replica thật.
- **Q3.** Các giá trị `stage` của `reindexProgress` (CR-CV-004), đặc biệt giá trị kết thúc và lỗi.
- **Q4.** Có đưa `orca.codeintel.index.changed` vào `notification-service` (thông báo "index đã sẵn sàng") không? Hiện không.
- **Q5.** Đăng ký `worktree_ids` bắt buộc ≤ 50; tab review nhiều worktree cùng lúc có vượt không? Giữ 50 cho đến khi đo.

## 8. Tham chiếu

- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/push_bridge.go` (dòng 12-52), `channels_push.go` (`registerNotificationStreamChannel`, `registerWorkspacePortsStreamChannel`, `ClientEventBus` dòng 146-190)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/mcpsession/jetstream_event_store.go`, `cmd/server/mcp_sessions_wiring.go` (NATS chỉ cho MCP)
- `/opt/repos/orca/backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (đầu file, `SubscribeEphemeral`), `internal/usecase/ports.go` (dòng 103-118, `MarkProcessed`), `internal/adapter/postgres/repository.go` (dòng 185-195)
- `/opt/repos/orca/backend-go/common/eventbus/eventbus.go` (dòng 24-33, 128, 196), `common/outbox/outbox.go` (`Store`, `Relay`)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/cmd/server/main.go` (dòng 254-282, `EnsureStream` + `outbox.Relay`), `internal/adapter/grpc/server.go` (dòng 985-1001, 1545), `internal/adapter/portevents/broadcaster.go` (mẫu broadcaster)
- `/opt/repos/orca/backend-go/common/grpcmw/grpcmw.go` (dòng 124-131)
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md` (mục 2.5, 2.6: `outbox_events`, `processed_events`)
- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.5-3.8, D4), `/opt/repos/orca/docs/research/view-code/07-architecture-decisions.md` (D4)
- CR-CV-022, CR-CV-023 cùng folder

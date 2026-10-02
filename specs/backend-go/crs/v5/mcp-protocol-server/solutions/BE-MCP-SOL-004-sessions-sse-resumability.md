# BE-MCP-SOL-004: Session bền, SSE, resumability, huỷ/tiến độ, đa replica; kênh `mcp.session.*`

> **Implemented (2026-10-02)** - xem "Ghi chú triển khai" ở cuối. (Trạng thái cũ: 🔲 Designed.) Phụ thuộc [BE-MCP-SOL-001](../../mcp-service-foundation/solutions/BE-MCP-SOL-001-scaffold-mcp-service.md) (mcp-service, outbox) và [BE-MCP-SOL-003](./BE-MCP-SOL-003-streamable-http-and-lifecycle.md) (`SessionStore`, `ProtocolEngine`, `inflight_registry`).

**CR:** [CR-MCP-004](../../../../../../docs/crs/v5/mcp-protocol-server/CR-MCP-004-sessions-sse-resumability.md)
**Service:** `mcp-service` (migration `0002_sessions`, usecase session, reaper), `api-gateway` (SSE, `grpcSessionStore`, `channels_mcp_session.go`), `common/eventbus` (API mới)
**TDD tham chiếu:** [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §5 (stateless/fan-out), §8 · [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (Event conventions) · [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) · **Hợp đồng FE:** [CONTRACT](../../CONTRACT-mcp-ui-api.md) §1 `McpSessionView`, §2.1 `mcp.session.list|close`, §2.2 `mcp.admin.session.list`, §3 `/mcp`

---

## 1. Trạng thái hiện tại (re-verify — xem [README](./README.md))

- **Không có** cơ chế cross-replica ngoài JetStream. `common/eventbus` chỉ có `Publisher.Publish(Event)` (JetStream), `Consumer.Subscribe` (durable, competing) và `SubscribeEphemeral` (consumer JetStream không tên, **cần stream**, mỗi bên nhận bản sao riêng, mất khi `InactiveThreshold` 5 phút). `Handler` chỉ nhận `eventbus.Event` — **không lộ sequence** của JetStream ⇒ không thể replay theo `Last-Event-ID` bằng API hiện có.
- `notification-service` dùng `SubscribeEphemeral` cho fan-out theo *domain event* (stream `NOTIFICATION`…), không phải theo từng kết nối — không phải mẫu cho SSE theo session.
- `wscompat` đã có `RegisterStream` (ack `nil`, sự kiện `Streaming:true`) — dùng cho `mcp.events.subscribe` ở BE-013, không phải SSE MCP.
- Không có Redis (`go.mod` không import).

### Quyết định khác/thêm so với CR gốc

1. **Bỏ ý "NATS ephemeral subject `mcp.session.<id>` bằng `SubscribeEphemeral`"** (xem [README](./README.md) hàng lệch). Thiết kế hai tầng, thay cho T5 hiện hành:
   - **Tín hiệu điều khiển (không bền)** — *core NATS* pub/sub, subject `orca.ephemeral.mcp.session.<sessionRowID>` (cancel, closed, kick) và `orca.ephemeral.mcp.tenant.<tenantID>` (`tools_list_changed`, `resources_updated`). Mất tin được phép (tự lành: client gọi lại `tools/list`; session đóng được dọn bởi reaper).
   - **Vòng đệm resume (có giới hạn)** — một stream JetStream `MCPSSE` giữ sự kiện SSE theo từng luồng, subject `orca.sse.mcp.<sessionRowID>.<streamID>` (ngoại lệ có chủ đích của quy ước `orca.<service>.<entity>.<event>`: phải **không giao** với `orca.mcp.>` của stream `MCP`, vì JetStream cấm hai stream trùng subject). Giới hạn cứng: `MaxMsgsPerSubject=256`, `MaxAge=10m`, `MaxBytes` toàn stream (`MCP_SSE_BUFFER_MAX_BYTES`, mặc định 256 MiB), `Discard=old`. Không cần Redis.
2. **Không dùng session id làm khoá lộ ra ngoài.** `Mcp-Session-Id` (header, UUID v4 từ `crypto/rand`) là bí mật; DB chỉ lưu `SHA-256` của nó. Mọi nơi khác (subject NATS, log, `McpSessionView.id`, audit) dùng **row id** (`mcp.sessions.id`, UUID không bí mật). ⇒ `mcp.session.close sessionId` của UI nhận **row id**.
3. **Mỗi message chỉ đi một luồng** (spec Streamable HTTP cấm gửi cùng message lên nhiều luồng — ghi theo trí nhớ, **chưa đối chiếu lại** văn bản spec). Response/progress của request R đi luồng của R (POST SSE); thông điệp do server khởi xướng đi luồng GET chính của session. Điều kiện CR "POST ở replica A, SSE ở replica B vẫn nhận progress + kết quả" được hiểu là **resume**: client rớt POST-stream rồi `GET` + `Last-Event-ID` vào replica B và nhận phần thiếu (test ở §Kiểm thử); không nhân bản message sang luồng khác.
4. Đếm stream hoạt động theo **DB** (bảng `session_streams` + heartbeat), không per-replica ⇒ hạn mức `MCP_MAX_SSE_STREAMS_PER_USER/TENANT` chính xác theo cluster (nâng cấp T6 so với BE-002).
5. Resume trượt (vòng đệm mất) ⇒ `404` (client `initialize` lại) đúng ý CR "rơi về không resume", không dựng luồng giả.

## 2. Giải pháp

### A. Dữ liệu — `mcp-service/migrations/postgres/0002_sessions.up.sql`

```sql
CREATE TABLE mcp.sessions (
    id                UUID PRIMARY KEY,                  -- row id, hiển thị ở UI/audit
    tenant_id         UUID NOT NULL,
    user_id           UUID NOT NULL,                     -- logical FK auth-service
    secret_hash       BYTEA NOT NULL UNIQUE,             -- SHA-256(Mcp-Session-Id)
    client_id         TEXT,                              -- oauth client (BE-005); NULL nếu PAT
    client_name       TEXT NOT NULL DEFAULT '',
    client_version    TEXT NOT NULL DEFAULT '',
    grant_id          UUID, token_id UUID,
    protocol_version  TEXT NOT NULL,
    capabilities      JSONB NOT NULL DEFAULT '{}',
    log_level         TEXT NOT NULL DEFAULT 'warning',
    state             TEXT NOT NULL CHECK (state IN ('initializing','ready','closed')),
    tool_calls        BIGINT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at         TIMESTAMPTZ,
    close_reason      TEXT                                -- client_delete|idle|user|admin|kill_switch|token_revoked
);
CREATE INDEX idx_sessions_tenant_user ON mcp.sessions (tenant_id, user_id) WHERE state <> 'closed';
CREATE INDEX idx_sessions_idle        ON mcp.sessions (last_seen_at) WHERE state <> 'closed';

CREATE TABLE mcp.session_streams (                      -- sống = heartbeat_at > now() - 90s
    id UUID PRIMARY KEY, session_id UUID NOT NULL REFERENCES mcp.sessions(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL, user_id UUID NOT NULL, replica_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('get','post')),
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(), heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_streams_live ON mcp.session_streams (tenant_id, user_id, heartbeat_at);
-- + RLS FORCE + policy tenant_isolation như 0001 (vòng lặp DO $$ cho 2 bảng)
```

FK trong cùng DB `mcp` được phép (chỉ cấm FK xuyên DB). `down.sql`: drop hai bảng.

### B. Proto (thêm vào `mcp.proto`, additive)

```proto
rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse);   // nhận secret_hash, trả row
rpc GetSessionBySecret(GetSessionBySecretRequest) returns (McpSession);    // lọc tenant theo metadata
rpc TouchSession(TouchSessionRequest) returns (TouchSessionResponse);      // last_seen, tool_calls_delta, state=ready, log_level
rpc CloseSession(CloseSessionRequest) returns (CloseSessionResponse);      // by row id hoặc secret_hash, reason
rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);      // của chính user (metadata)
rpc ListSessionsAdmin(ListSessionsAdminRequest) returns (ListSessionsResponse); // role admin, mọi user của tenant
rpc OpenStream(OpenStreamRequest) returns (OpenStreamResponse);            // kiểm hạn mức + chèn session_streams, atomically
rpc HeartbeatStream(HeartbeatStreamRequest) returns (HeartbeatStreamResponse);
rpc CloseStream(CloseStreamRequest) returns (CloseStreamResponse);
```

`OpenStream` chạy trong một giao dịch có `pg_advisory_xact_lock(hashtextextended(user_id::text,0))` rồi `SELECT count(*) … heartbeat_at > now()-interval '90 seconds'`; vượt `MCP_MAX_SSE_STREAMS_PER_USER`/`_PER_TENANT` ⇒ `apperrors.KindFailedPrecondition` + mã `MCP_STREAM_LIMIT` (**mã ngoài CONTRACT** — gateway biến thành HTTP 429 cho client MCP; không bao giờ tới FE nên không cần đổi CONTRACT). Reaper và job dọn `session_streams` quá hạn chạy trong `mcp-service` (`internal/usecase/reap_idle_sessions.go`, ticker 1 phút, `MCP_SESSION_IDLE_TTL`): `UPDATE mcp.sessions SET state='closed', closed_at=now(), close_reason='idle' WHERE state<>'closed' AND last_seen_at < now()-$ttl RETURNING id, tenant_id, user_id`, mỗi hàng ghi outbox `orca.mcp.session.closed` (payload `{sessionId, userId, reason}`; consumer `mcp.events.subscribe` ở BE-013 chuyển thành `McpEvent{type:'session.closed'}`) và publish tín hiệu `closed` core-NATS.

### C. Gateway — `grpcSessionStore`, SSE, cross-replica (`internal/adapter/mcpserver/`)

Thêm file: `session_store_grpc.go`, `sse_writer.go`, `stream_registry.go`, `event_bridge.go`, `resume_replay.go`, `control_signals.go`.

**Vòng đời request** (mọi bước dùng row id làm khoá nội bộ):

1. `Mcp-Session-Id` → `sha256` → `GetSessionBySecret` (đệm LRU 5 s theo secret_hash để giảm tải; `404` nếu `closed`) → so danh tính với `Principal` (BE-003).
2. `TouchSession` tối đa 1 lần/30 s/session/replica (tránh điểm nóng ghi).
3. Request cần SSE (handler yêu cầu stream hoặc client `Accept: text/event-stream` và có progressToken): `OpenStream(kind='post')` → mở luồng.
4. `GET` ⇒ `OpenStream(kind='get')`; nếu đã có luồng GET sống của session ⇒ 409 (spec: tối đa một luồng GET). `DELETE` ⇒ `CloseSession(reason='client_delete')`, huỷ tool đang chạy, đóng luồng, `204`.

**Ghi & đọc luồng** (đường duy nhất cho mọi sự kiện SSE):

```go
// Phát: replica đang xử lý request R
func (b *EventBridge) Emit(ctx context.Context, st StreamRef, msg jsonrpc.Message) error {
    data, _ := json.Marshal(msg)
    if len(data) > b.maxEventBytes { data = tooLargeErrorEvent(msg.ID) } // MCP_MAX_EVENT_BYTES, mặc định 256 KiB
    _, err := b.js.Publish(ctx, "orca.sse.mcp."+st.SessionRowID+"."+st.StreamID, data) // PubAck.Sequence = event id
    return err
}
// Đọc: replica giữ kết nối SSE — consumer JetStream có thứ tự (ephemeral), filter đúng subject
//   DeliverNew (luồng mới) | DeliverByStartSequence(lastSeq+1) (resume)
// id SSE = "<streamID>.<seq>" ; seq là JetStream stream sequence (đơn điệu, có thể thưa — chỉ cần tăng dần)
```

- Resume (`GET` + `Last-Event-ID: <streamID>.<seq>`): tra `session_streams`/vòng đệm: nếu `seq` ≥ `FirstSeq` của subject ⇒ consumer `ByStartSequence(seq+1)` → phát đúng phần thiếu rồi chuyển sang live, **không lặp, không mất**; nếu `seq` < `FirstSeq` (đã bị vòng đệm xoá) hoặc luồng thuộc session khác ⇒ `404`. Luồng resume gắn lại `session_streams` (row mới, cùng `streamID` logic) để vẫn tính hạn mức.
- Heartbeat: comment `: ping\n\n` mỗi 25 s + `HeartbeatStream` mỗi 30 s; client đi (`r.Context().Done()`) ⇒ `CloseStream`, dừng consumer.
- Progress: `notifications/progress` chỉ khi `_meta.progressToken` có; **coalesce** ≤ 5 sự kiện/s/token (giữ giá trị mới nhất) trước `Emit`.
- Hủy: `notifications/cancelled{requestId}` ⇒ `inflight.Cancel` cục bộ **và** publish core-NATS `orca.ephemeral.mcp.session.<rowID>` `{"t":"cancel","rid":<requestId>}`; replica nào đang chạy request đó huỷ `ctx` (handler phải truyền `ctx` xuống `Registry.Dispatch`). Lưới an toàn 60 s của `dispatchRPCTimeout` không đổi; tool dài (BE-009) có deadline riêng.
- `DELETE`/reaper/kill: tín hiệu `{"t":"closed"}` ⇒ mọi replica huỷ ngữ cảnh in-flight của session, đóng luồng, `stream.Purge` filter `orca.sse.mcp.<rowID>.>`.
- `list_changed`: replica đăng ký **một** subscription core-NATS `orca.ephemeral.mcp.tenant.>`; khi nhận `{"t":"tools_list_changed"}`, ghi trực tiếp `notifications/tools/list_changed` lên kết nối GET chính của các session tenant đó **do replica này giữ** (không qua vòng đệm — là gợi ý tự lành, tránh nhân bản N lần). Nguồn phát: mcp-service/BE-007/BE-012 gọi `eventbus.Ephemeral.Publish`.

### D. Kênh WS (`channels_mcp_session.go`, đăng ký qua `mcpHandler` của BE-002)

| Kênh | Quyền | Xử lý |
|---|---|---|
| `mcp.session.list` | user | `ListSessions` (lọc `user_id` từ metadata Identity — không nhận tham số); map `McpSessionView` |
| `mcp.session.close` | user | `args[0]` = object `{"sessionId": "<row id>"}` (dialect session-client chỉ chuyển **một** tham số object — `session_dialect.go:normalizeInboundMessage`; xem "Đề nghị đổi CONTRACT"); `CloseSession(reason='user')` chỉ khi `user_id` khớp; không khớp/không có ⇒ `MCP_NOT_FOUND: session not found` (không phân biệt) ⇒ phát core-NATS `closed` rồi `{ok:true}` |
| `mcp.admin.session.list` | admin (`mcpHandler(adminOnly=true)`) | `ListSessionsAdmin` (mọi user của tenant) |

Ánh xạ JSON (camelCase, đúng CONTRACT §1):

```go
type McpSessionView struct {
    ID string `json:"id"`; ClientName string `json:"clientName"`; GrantID string `json:"grantId,omitempty"`
    TokenID string `json:"tokenId,omitempty"`; CreatedAt string `json:"createdAt"`; LastSeenAt string `json:"lastSeenAt"`
    ProtocolVersion string `json:"protocolVersion"`; ActiveStreams int `json:"activeStreams"`; ToolCalls int64 `json:"toolCalls"`
    UserID string `json:"userId,omitempty"`; UserName string `json:"userName,omitempty"` // chỉ kênh admin
}
```

`activeStreams` = `count(session_streams WHERE heartbeat_at > now()-90s)` trong cùng truy vấn list (LEFT JOIN LATERAL). `userName` (kênh admin): tra qua client auth-service của gateway nếu có RPC lấy user theo id **(chưa xác minh tên RPC)**; thiếu ⇒ bỏ trường (FE hiển thị `userId` rút gọn). Thời gian RFC 3339 UTC (C7). Danh sách chỉ gồm session `state<>'closed'`.

### E. Thay đổi `common/eventbus` (dùng chung — cần chủ sở hữu duyệt)

File mới `common/eventbus/ephemeral.go` và `common/eventbus/replay.go` (không đụng chữ ký hiện có):

```go
// Ephemeral: core-NATS pub/sub, KHÔNG bền, KHÔNG replay — chỉ cho tín hiệu tự lành/điều khiển.
type Ephemeral struct{ nc *nats.Conn }
func NewEphemeral(url string) (*Ephemeral, func(), error)                       // kết nối riêng, đặt PendingLimits
func (e *Ephemeral) Publish(subject string, data []byte) error
func (e *Ephemeral) Subscribe(subject string, fn func(subject string, data []byte)) (unsubscribe func(), err error)

// Replay: đọc stream JetStream từ sequence cho SSE resume (Handler thường không lộ seq).
type ReplayHandler func(ctx context.Context, subject string, seq uint64, data []byte) error
func (c *Consumer) ReplayFrom(ctx context.Context, stream, subject string, startSeq uint64, fn ReplayHandler) (stop func(), err error)
func (p *Publisher) PublishRaw(ctx context.Context, subject string, data []byte) (seq uint64, err error)
func (p *Publisher) EnsureBoundedStream(ctx context.Context, cfg BoundedStream) error // MaxMsgsPerSubject, MaxAge, MaxBytes
```

Phát sự kiện bền của miền (`orca.mcp.session.closed`) vẫn qua outbox → `Publisher.Publish(Event)` như mọi service (arch/08). Hai thứ trên là *ngoại lệ có tài liệu* cho dữ liệu tạm thời: không chứa secret/token; payload SSE đã qua cùng bộ che secret với audit; NATS cần ACL chỉ cho gateway + mcp-service publish/subscribe `orca.sse.>` và `orca.ephemeral.>` **(chưa xác minh dev NATS có auth không)**.

## Hợp đồng với frontend

- Kênh: `mcp.session.list` → `McpSessionView[]` của chính user; `mcp.session.close(sessionId)` → `{ok:true}`; `mcp.admin.session.list` → `McpSessionView[]` mọi user (kèm `userId`, `userName?`). Đúng CONTRACT §2.1/§2.2.
- Lỗi: `MCP_DISABLED`, `MCP_NOT_ADMIN`, `MCP_NOT_FOUND` (đóng session không phải của mình / đã đóng — idempotent: đóng session đã `closed` của chính user vẫn trả `{ok:true}`). Không phát sinh mã mới ra FE.
- Sự kiện: reaper/đóng ⇒ outbox `orca.mcp.session.closed` → BE-013 đẩy `McpEvent{type:'session.closed', sessionId:<row id>}` (cùng loại id mà `mcp.session.list` trả ⇒ FE-MCP-SOL-002 có thể loại hàng khỏi bảng theo `sessionId`).
- `Mcp-Session-Id` **không bao giờ** ra FE.
- **Đề nghị đổi CONTRACT (không tự sửa):** §2 chưa nói rõ hình dạng tham số. Mã thật (`session_dialect.go`) chỉ chuyển `params` thành **một** `args[0]` object cho web client ⇒ mọi kênh `mcp.*` phải nhận *một object* (`{sessionId}`, `{requestId, decision, scopes}`, `{approvalId, decision, paramsHash, note?}`…), không phải tham số vị trí. Cần ghi quy tắc này (C10) vào CONTRACT; BE và FE solution đã áp dụng quy tắc này.

## Sửa TDD kèm theo

- **T5** — sửa văn bản: "core-NATS ephemeral `orca.ephemeral.mcp.>` cho tín hiệu điều khiển + stream JetStream giới hạn `MCPSSE` (`orca.sse.mcp.>`) cho vòng đệm resume; **không** dùng `SubscribeEphemeral` (là consumer JetStream)". Vào `arch/08` mục "Event conventions" (thêm "ephemeral/bounded subjects") và `api-gateway.md` §5.
- **T6** — hạn mức stream chính xác theo cluster qua `session_streams` (`api-gateway.md` §9).
- **T7** — `mcp-service` sở hữu bảng `sessions` (đã nêu ở BE-001).
- **T8** — metric `orca_mcp_sessions_active`, `orca_mcp_sse_streams_active`, `orca_mcp_resume_total{result}` (BE-015).

## Kiểm thử

```bash
cd /opt/repos/orca/backend-go/common && go test ./eventbus/...                       # unit (fake)
go test -tags=integration ./eventbus/...                                              # NATS testcontainers: ReplayFrom, bounded stream
cd ../services/mcp-service && go test ./... && go test -tags=integration ./internal/adapter/postgres/...
cd ../api-gateway && go test ./internal/adapter/mcpserver/... ./internal/adapter/wscompat/... -run 'Session|SSE|Resume|Cross' -v
go test -tags=integration -run TestTwoReplicaResume ./internal/adapter/mcpserver/...  # 2 handler + 1 NATS
go test -run TestSSEGoroutineLeak ./internal/adapter/mcpserver/...                    # goleak
```

| Test | Nội dung |
|---|---|
| `TestTwoReplicaResume` | replica A xử lý POST (SSE) phát 100 event, cắt kết nối ở event 40; client `GET`+`Last-Event-ID` vào replica B ⇒ nhận đúng 41..100, không lặp (so id) |
| `TestSessionIdentityBinding` | session user A + token user B ⇒ 404; counter `identity_mismatch` tăng |
| `TestDeleteCancelsTools` | `DELETE /mcp` ⇒ tool đang chạy (giả) nhận `ctx.Done()`; `goleak.VerifyNone` |
| `TestIdleTTL` | đổi đồng hồ giả: quá TTL ⇒ request kế = 404; `initialize` lại OK; outbox có `orca.mcp.session.closed` |
| `TestStreamLimitClusterWide` | 2 replica mở GET cho cùng user vượt `MAX_PER_USER` ⇒ cái vượt nhận 429 dù ở replica khác |
| `TestResumeGap` | `MaxMsgsPerSubject` nhỏ, resume từ seq đã bị xoá ⇒ 404 |
| `TestProgressCoalesce` | 50 progress/s ⇒ ≤ 5/s ra SSE, event cuối luôn được phát |
| RLS (mcp-service) | role không-superuser: user thuộc tenant B không thấy `sessions`/`session_streams` tenant A; `ListSessions` lọc đúng `user_id` |
| Tải (kịch bản, không chặn CI) | 500 session × 1 SSE, bộ nhớ ổn định (`runtime.MemStats` tại phút 1 vs 10 ± 10%) |
| `channels_mcp_session_test.go` | `mcp.session.close` của người khác ⇒ `MCP_NOT_FOUND`; list chỉ có session của mình; admin list có `userId`; golden JSON camelCase |

## Rủi ro & phụ thuộc

- Số subject JetStream = số luồng sống (`orca.sse.mcp.<row>.<stream>`): 500 session ≈ 1000 subject — chấp nhận được; đo ở bài tải. Nếu vượt, gộp theo session (một subject/session, lọc stream trong payload).
- API `common/eventbus` mới ảnh hưởng mọi service import ⇒ chỉ **thêm** hàm; chạy `impact({target:"Consumer", direction:"upstream"})` và `impact({target:"Publisher", direction:"upstream"})` trước khi sửa **(chưa chạy; rủi ro dự kiến HIGH vì mọi service dùng `eventbus.Connect`)** — báo người duyệt, giữ nguyên chữ ký cũ.
- Bộ nhớ vòng đệm: giới hạn cứng ở stream; hết chỗ ⇒ `Discard=old` ⇒ resume trượt ⇒ 404 (không OOM).
- NATS down: `Emit` lỗi ⇒ đóng luồng với JSON-RPC error `-32603`, session vẫn sống (client retry); readiness gateway phản ánh NATS khi `MCP_ENABLED`.
- Giả định cần xác minh khi cài: `PubAck.Sequence` đơn điệu theo subject qua nhiều node JetStream (cluster) — **chưa xác minh**, fallback: sequence do mcp-service cấp bằng `bigserial` + lưu vòng đệm Postgres.

## Không thuộc phạm vi

Primitive `tasks` bất đồng bộ (BE-009), verifier token (BE-005/006), `mcp.events.subscribe` (BE-013), metric implement (BE-015), kill switch theo session (BE-013 dùng `CloseSession(reason='kill_switch')`).

## Liên quan

[CR-004](../../../../../../docs/crs/v5/mcp-protocol-server/CR-MCP-004-sessions-sse-resumability.md) · [BE-MCP-SOL-003](./BE-MCP-SOL-003-streamable-http-and-lifecycle.md) · `backend-go/common/eventbus/eventbus.go` · [FE-MCP-SOL-002](../../../../../frontend/crs/v5/mcp-protocol-server/solutions/FE-MCP-SOL-002-connect-panel-and-active-sessions.md)


## Ghi chú triển khai (2026-10-02) - lệch so với thiết kế

- Migration là `0006_sessions` (0005 đã dùng cho custom prompts). `session_streams` chỉ ghi luồng **GET/resume** (kind `get`); POST-stream không đăng ký, nên hạn mức `MCP_MAX_SSE_STREAMS_*` và `activeStreams` chỉ đếm luồng GET.
- Gateway **không còn dùng** `mcp.NewStreamableHTTPHandler`: `adapter/mcpserver/session_host.go` giữ kết nối SDK theo replica, nhận lại ("adopt") session từ store bằng `ServerSessionOptions.State`, ràng buộc danh tính (tenant+user) và trả 404 khi lệch. POST luôn trả SSE (transport SDK không cho đặt `JSONResponse` từ ngoài).
- Id sự kiện giữ định dạng của SDK `"<streamID>_<ordinal>"` (không phải `<stream>.<jetstream seq>`); ordinal ghi trong header `Orca-Idx` nên vẫn đúng khi vòng đệm đã xoá bản cũ. Resume (`GET`+`Last-Event-ID`) do gateway tự phục vụ bằng consumer có thứ tự JetStream (replay + live tail tới khi có response), nên POST ở A / resume ở B nhận đúng phần còn lại. Luồng GET độc lập (`""`) chỉ resume được best-effort khi nhiều replica.
- Chủ đề điều khiển core-NATS: `orca.ephemeral.mcp.session.<rowID>` (`cancel`, `closed`) và `orca.ephemeral.mcp.tenant.<id>` (`tools_list_changed`, chỉ chuyển tiếp khi `Config.ToolsListChanged`). Vòng đệm: stream `MCPSSE` (memory storage), `MaxMsgsPerSubject=256`, `MaxAge=10m`, `MaxBytes=MCP_SSE_BUFFER_MAX_BYTES`, subject dùng khoá dẫn xuất từ secret (không lộ `Mcp-Session-Id`).
- SDK `ServerSession.Close` chỉ chờ handler, không huỷ: gateway tự huỷ context request đang chạy khi DELETE / signal `closed`.
- Elicitation: phản hồi của client là một POST riêng, phải tới đúng replica đã gửi câu hỏi (SDK giữ outgoing call theo kết nối); không có sticky thì quá hạn và rơi về duyệt ngoài kênh.

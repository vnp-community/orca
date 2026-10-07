# code-intel-service

Dịch vụ backend tổng hợp mô hình đồ thị ngữ nghĩa mã nguồn (Code Intelligence Graph Pipeline - v7).

## Kiến trúc và Thành phần

1. **Canonical Graph Model (`BE-CV-SOL-020`)**:
   - Biểu diễn đồ thị chuẩn hoá đa tầng: Symbol Graph, Impact Graph, Architecture Graph, Flow Graph, ERD Catalog.
   - Vector khoá biểu tượng xác định (`SymbolRef`, `NormalizeSymbolKey`, `NormalizeRepoPath`).
   - Cắt tỉa xác định theo ngân sách (`TruncationPolicy`, `ViewLimits`).

2. **Agent Collector (`BE-CV-SOL-021`)**:
   - Bộ thu gom dữ liệu từ DevServer Agent qua gRPC/AgentRPC.
   - Quản lý timeout, hồi phục kết nối (`ReconnectWait`), retry an toàn cho các tác vụ đọc.

3. **Snapshot Cache (`BE-CV-SOL-022`)**:
   - Bộ đệm snapshot đa tầng (in-memory + database).
   - `HeadProbe` xác định tính tươi mới của snapshot (TTL 15s, singleflight).
   - ETag SHA-256 xác thực `NotModified`.
   - Giới hạn tải payload (3 MiB default, 64 MiB per binding, 512 MiB per tenant).
   - Invalidator và Janitor dọn dẹp snapshot định kỳ.

4. **Infra-Fleet Transport (`BE-CV-SOL-023`)**:
   - Giao tiếp hai chiều với infra-fleet-service.
   - Streaming sự kiện thời gian thực và handshake năng lực agent.

5. **Event Distribution (`BE-CV-SOL-024`)**:
   - NATS JetStream stream: `CODEINTEL` (subject wildcard: `orca.codeintel.>`).
   - Ephemeral subscription fan-out trên từng replica.
   - Bounded dedup LRU (1024 sự kiện).
   - Server-sent push streaming qua `StreamCodeIntelEvents` gRPC API với xác thực quyền truy cập từng worktree selector (tối đa 50 selectors).

## Biến môi trường

| Biến môi trường | Mặc định | Ý nghĩa |
|-----------------|----------|---------|
| `CODEINTEL_ENABLED` | `true` | Cờ bật/tắt dịch vụ code-intel |
| `CODEINTEL_AGENT_CALL_TIMEOUT` | `95s` | Timeout gọi agent |
| `CODEINTEL_RECONNECT_WAIT` | `20s` | Thời gian chờ reconnect |
| `CODEINTEL_MAX_INFLIGHT_PER_DEVSERVER` | `4` | Giới hạn gọi đồng thời mỗi devserver |
| `CODEINTEL_AGENT_MAX_RESULT_BYTES` | `12582912` (12 MiB) | Dung lượng tối đa kết quả raw |
| `CODEINTEL_HEAD_PROBE_TTL` | `15s` | TTL của bộ đệm thăm dò Git HEAD |
| `CODEINTEL_COLLECT_TIMEOUT` | `20s` | Ngân sách gọi đồng bộ từ client trước khi trả timeout |
| `CODEINTEL_SNAPSHOT_TTL` | `720h` (30d) | TTL lưu trữ snapshot trong DB |
| `CODEINTEL_SNAPSHOT_KEEP_COMMITS` | `5` | Số commit giữ lại cho mỗi worktree |
| `CODEINTEL_SNAPSHOT_MAX_BYTES` | `3145728` (3 MiB) | Giới hạn kích thước payload snapshot |
| `CODEINTEL_CACHE_MAX_BYTES_PER_BINDING` | `67108864` (64 MiB) | Hạn mức cache cho mỗi repo binding |
| `CODEINTEL_CACHE_MAX_BYTES_PER_TENANT` | `536870912` (512 MiB) | Hạn mức cache cho mỗi tenant |
| `CODEINTEL_SYMBOL_LRU_ENTRIES` | `64` | Số mục LRU cho Symbol View |
| `CODEINTEL_SYMBOL_LRU_TTL` | `5m` | TTL cho Symbol View LRU |
| `CODEINTEL_MAINTENANCE_INTERVAL` | `1h` | Chu kỳ chạy dọn dẹp snapshot cũ |

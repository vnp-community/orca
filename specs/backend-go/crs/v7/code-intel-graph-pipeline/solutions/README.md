# backend-go Solutions: Code Intel Graph Pipeline (v7)

**CRs:** [docs/crs/v7/code-intel-graph-pipeline](../../../../../../docs/crs/v7/code-intel-graph-pipeline/README.md)
**Hợp đồng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md) (nguồn sự thật; mục 1 PQ-xx, §7, §8), [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md), [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md)

> ✅ Implemented. Đã triển khai và vượt qua toàn bộ test (unit, race, contract, integration).

## Bảng CR, Solution, Task

| CR | Solution | Service / Area | Effort | Task |
|----|----------|----------------|--------|------|
| [CR-CV-020](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-020-canonical-graph-model.md) | [BE-CV-SOL-020](./BE-CV-SOL-020-canonical-graph-model.md) | `proto` (`codeintel_common`, `codeintel_graph`), `code-intel-service/internal/domain` | Large | `BE-CV-TASK-020-01` đến `-08` |
| [CR-CV-021](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-021-agent-collector.md) | [BE-CV-SOL-021](./BE-CV-SOL-021-agent-collector.md) | `code-intel-service` (collector, reindex), `proto` (`codeintel_reindex`, request/response đồ thị) | Large | `BE-CV-TASK-021-01` đến `-09` |
| [CR-CV-022](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-022-snapshot-cache.md) | [BE-CV-SOL-022](./BE-CV-SOL-022-snapshot-cache.md) | `code-intel-service` (cache, `graph_snapshots`) | Medium | `BE-CV-TASK-022-01` đến `-08` |
| [CR-CV-023](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-023-infra-fleet-codeintel-transport.md) | [BE-CV-SOL-023](./BE-CV-SOL-023-infra-fleet-codeintel-transport.md) | `infra-fleet-service`, `proto/infrafleet` | Medium | `BE-CV-TASK-023-01` đến `-09` |
| [CR-CV-024](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-024-event-distribution.md) | [BE-CV-SOL-024](./BE-CV-SOL-024-event-distribution.md) | `code-intel-service` (ingest, outbox, NATS, gRPC stream), `proto` (`codeintel_events`) | Medium | `BE-CV-TASK-024-01` đến `-09` |

## Re-verify trước khi thiết kế (đối chiếu CR với mã thật, 2026-10-06)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| `RelayByDevServer` làm mất `error.data.code` | Đúng (`relay_by_dev_server.go:59–62`, `apperrors.ToGRPCStatus` chỉ `Code: Message`) | Không |
| Go không có hàng đợi chờ kết nối lại | Đúng (`relay_by_dev_server.go:55–57` trả lỗi ngay) | Không |
| `tools[]` không tới Go | Đúng (`inboundHandshakeParams`, `HandshakeInfo`, `usecase.HandshakeInfo` đều không có) | Không |
| `StreamFileChanges` thiếu gắn tenant | Đọc code: handler không gọi `withTenantFromStreamMetadata` (chưa chạy) | Chưa kiểm chứng bằng chạy |
| CR-023 Q3: `StreamGuard` có dùng? | `cmd/server/main.go` **không** dùng `internalcaller` | Đã giải |
| Gateway tự mở lại luồng push | Không: `Recv` lỗi → đóng kênh | CR đúng; việc của SOL-040/FE-050 |
| `ChangeOverlay` base + dải số ở CR-020 | PQ-09: CR-036 sở hữu | **Lệch** → SOL-020 bỏ |
| `view` `ViewKind` 10 giá trị; `ARCHITECTURE`←`overview` | PQ-10 + §4 T3: 19+ chuỗi; `ARCHITECTURE` là C4 | **Lệch** → SOL-020/021 (`CLUSTERS`) |
| `StreamCodeIntelEvents` ≤ 4 luồng; `worktree_ids` | PQ-18: 16; PQ-11: `selectors` | **Lệch** → SOL-023/024 |

## Thứ tự thực thi và phụ thuộc

```
BE-CV-SOL-010 (khung service, buf, KindUnavailable) ──┐
SOL-020 (mô hình, proto common/graph) ────────────────┼─▶ SOL-021 (collector, reindex) ─▶ SOL-022 (cache) ─▶ SOL-024 (sự kiện)
SOL-023 (infra-fleet, G2) ────────────────────────────┘                         ▲                                   ▲
   └──────────────────────────────────────────────────────────────────────────┴──── (SOL-023 cũng là nền của 024)
Ngoài feature: SOL-011/012/013 (bảng, binding, quyền) ─▶ 021, 022, 024;  SOL-022/024 ─▶ SOL-040-* (gateway)
```

SOL-020 và SOL-023 độc lập, làm song song (đợt 1, hợp đồng §7.3). SOL-021 cần cả hai và SOL-012/013. SOL-022 bọc SOL-021; SOL-024 cần SOL-023 (stream), SOL-022 (`InvalidateBinding`), SOL-021 (`Watch`, `reindex_jobs`).

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | Agent chuẩn hoá `SymbolRef`, backend kiểm lại và hợp nhất (đổi so với CR-020 F1) | PQ-20; một nơi chịu trách nhiệm, backend phòng thủ |
| F2 | Khoá `SymbolRef` bỏ `#arity`, dòng 1-based, `::`→`.` | Đã đo lệch GitNexus/CodeGraph (CR-020) |
| F3 | Collector tự chờ dev server kết nối lại 20 s, không sửa `RelayByDevServer` | `RelayByDevServer` dùng chung nhiều service |
| F4 | Mã lỗi `CODEINTEL_*` đi bằng tiền tố message + trailer `x-orca-agent-error-data-bin` | PQ-02; `ToGRPCStatus` chỉ gửi `Code: Message` |
| F5 | Cache khoá theo HEAD; `stale` không kích hoạt thu thập lại; huỷ bằng xoá | O3 |
| F6 | `StreamCodeIntelEvents` (gRPC) thay vì NATS để lấy thông báo từ agent; gateway nhận push bằng gRPC stream | Tra tenant trong đường nóng; khớp mẫu hiện có |
| F7 | Giữa replica: outbox + NATS tạm; `processed_events` chỉ ở ingest | Mỗi replica phát cho luồng gateway của mình |
| F8 | Cổng/port của collector và cache đặt file riêng, không dồn vào `ports.go` | Giảm xung đột merge với SOL-010/011/012 |
| F9 | Mọi nhánh hai dialect có suite hợp đồng chung; mọi truy vấn `tenant_id` + test cô lập | Hợp đồng §8.3 |
| F10 | `AgentRPCCaller` là lõi dùng chung cho `quality.*` (SOL-082) | Một nơi xử lý lỗi/timeout/trailer |

## Điều còn mở

- **Số field và enum đề xuất** (`ToolIndexStatus` 11–18, `ViewKind` 11–21, `ReindexJob`, request/response đồ thị): chủ sở hữu CR gán trước khi merge (không đổi được sau `buf breaking`).
- **Thiếu trong hợp đồng** (báo chủ hợp đồng): enum `ViewKind` ↔ chuỗi §4; `SymbolDetail` ở §2.1; `ReindexJob`/`RequestReindex`; field `state` của `CodeIntelEvent`; ánh xạ `reason` agent → push; cổng đọc binding xuyên tenant; ánh xạ `STRUCTURE` (agent không có `center` thư mục); env `INFRA_CODEINTEL_MAX_STREAMS`, `CODEINTEL_REINDEX_POLL_AFTER`.
- **Chưa kiểm chứng:** trailer qua `otelgrpc`; quy tắc ghép `workspaceRoot` ↔ binding (O-14/CR-024 Q1); mọi con số hiệu năng (O-15); hình `data` agent (mẫu Cypher chưa chạy).
- **Gateway không tự mở lại luồng** khi service khởi động lại: xử lý ở SOL-040 + FE-050 (`resync`).

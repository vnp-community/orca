# backend-go Tasks: Code Intel Graph Pipeline (v7)

Task thực thi của năm solution trong [`../solutions/`](../solutions/README.md). Tất cả 43 task có `Status: [x] DONE`, đã chạy và vượt qua toàn bộ test (unit, race, contract, integration). NN tăng liên tục trong cùng CR; mỗi task ghi solution mẹ.

## Bảng Solution, Task

| Solution | Task | Nội dung | Priority |
|----------|------|----------|----------|
| BE-CV-SOL-020 | [020-01](./BE-CV-TASK-020-01-symbol-vectors-and-reverify.md) | Re-verify + vector khoá dùng chung agent | P0 |
| | [020-02](./BE-CV-TASK-020-02-proto-codeintel-common.md) | Proto `codeintel_common` | P0 |
| | [020-03](./BE-CV-TASK-020-03-proto-codeintel-graph-types.md) | Proto `codeintel_graph` (kiểu) | P0 |
| | [020-04](./BE-CV-TASK-020-04-symbol-ref-key-normalization.md) | `SymbolRef`, khoá, ánh xạ kind/cạnh | P0 |
| | [020-05](./BE-CV-TASK-020-05-repo-path-normalization.md) | `NormalizeRepoPath` | P0 |
| | [020-06](./BE-CV-TASK-020-06-merge-two-sources.md) | Hợp nhất hai nguồn | P0 |
| | [020-07](./BE-CV-TASK-020-07-graph-limits-and-deterministic-truncation.md) | Giới hạn, cắt xác định, `ViewKind` | P0 |
| | [020-08](./BE-CV-TASK-020-08-graph-proto-mapping-and-ci.md) | Ánh xạ proto + CI `buf` | P1 |
| BE-CV-SOL-021 | [021-01](./BE-CV-TASK-021-01-reverify-collector-foundations.md) | Re-verify nền collector | P0 |
| | [021-02](./BE-CV-TASK-021-02-gateway-port-and-typed-params.md) | Cổng gateway, tham số kiểu hẹp | P0 |
| | [021-03](./BE-CV-TASK-021-03-infrafleet-client-adapter.md) | Adapter infrafleetclient | P0 |
| | [021-04](./BE-CV-TASK-021-04-agent-error-mapping.md) | Ánh xạ lỗi | P0 |
| | [021-05](./BE-CV-TASK-021-05-reconnect-wait-and-retry.md) | Chờ kết nối lại, retry | P1 |
| | [021-06](./BE-CV-TASK-021-06-collect-view-orchestration.md) | `CollectView` | P0 |
| | [021-07](./BE-CV-TASK-021-07-proto-graph-rpc-messages.md) | Proto request/response đồ thị | P0 |
| | [021-08](./BE-CV-TASK-021-08-reindex-proto-and-usecases.md) | `codeintel_reindex.proto`, reindex | P0 |
| | [021-09](./BE-CV-TASK-021-09-grpc-handlers-and-collector-integration.md) | Handler, metrics, tích hợp | P1 |
| BE-CV-SOL-022 | [022-01](./BE-CV-TASK-022-01-reverify-snapshot-table-and-singleflight.md) | Re-verify bảng, `x/sync` | P0 |
| | [022-02](./BE-CV-TASK-022-02-snapshot-key-hash-and-etag.md) | Khoá, hash, ETag | P0 |
| | [022-03](./BE-CV-TASK-022-03-snapshot-store-port-and-contract-suite.md) | Cổng + suite hợp đồng | P0 |
| | [022-04](./BE-CV-TASK-022-04-head-probe-and-staleness.md) | `HeadProbe`, stale | P0 |
| | [022-05](./BE-CV-TASK-022-05-cached-view-reader-and-singleflight.md) | `CachedViewReader` | P0 |
| | [022-06](./BE-CV-TASK-022-06-not-modified-and-payload-limits.md) | `not_modified`, hạn mức | P1 |
| | [022-07](./BE-CV-TASK-022-07-invalidator-and-janitor.md) | Invalidator, janitor | P1 |
| | [022-08](./BE-CV-TASK-022-08-config-metrics-and-cache-integration.md) | Cấu hình, tích hợp hai dialect | P1 |
| BE-CV-SOL-023 | [023-01](./BE-CV-TASK-023-01-reverify-exec-error-consumers.md) | Re-verify người tiêu thụ lỗi | P0 |
| | [023-02](./BE-CV-TASK-023-02-agent-rpc-error-type-and-exec.md) | `AgentRPCError`, `Exec` | P0 |
| | [023-03](./BE-CV-TASK-023-03-agent-error-mapping-and-trailer.md) | Ánh xạ lỗi + trailer | P0 |
| | [023-04](./BE-CV-TASK-023-04-exec-timeout-table.md) | Bảng timeout | P0 |
| | [023-05](./BE-CV-TASK-023-05-handshake-tools-and-capabilities.md) | `tools[]`/capabilities | P1 |
| | [023-06](./BE-CV-TASK-023-06-get-agent-capabilities-rpc.md) | `GetAgentCapabilities` | P1 |
| | [023-07](./BE-CV-TASK-023-07-codeintel-notification-routing-and-registry.md) | Định tuyến thông báo, registry | P0 |
| | [023-08](./BE-CV-TASK-023-08-stream-code-intel-events-rpc.md) | `StreamCodeIntelEvents` | P0 |
| | [023-09](./BE-CV-TASK-023-09-transport-integration-and-contract-checks.md) | Tích hợp, hồi quy | P1 |
| BE-CV-SOL-024 | [024-01](./BE-CV-TASK-024-01-reverify-event-foundations.md) | Re-verify nền sự kiện | P0 |
| | [024-02](./BE-CV-TASK-024-02-proto-codeintel-events.md) | Proto `codeintel_events` | P0 |
| | [024-03](./BE-CV-TASK-024-03-infra-fleet-stream-client-and-supervisor.md) | Client luồng + supervisor | P0 |
| | [024-04](./BE-CV-TASK-024-04-coalescer-and-rate-limits.md) | Gộp, giới hạn | P0 |
| | [024-05](./BE-CV-TASK-024-05-handle-index-changed-with-outbox.md) | `index_changed` + outbox | P0 |
| | [024-06](./BE-CV-TASK-024-06-reindex-progress-and-quality-events.md) | Tiến trình, quality | P1 |
| | [024-07](./BE-CV-TASK-024-07-nats-consumer-and-push-broadcaster.md) | NATS tạm, broadcaster | P0 |
| | [024-08](./BE-CV-TASK-024-08-stream-handler-and-push-authorization.md) | Handler + quyền | P0 |
| | [024-09](./BE-CV-TASK-024-09-two-replica-integration-and-storm.md) | Tích hợp hai replica | P1 |

## Thứ tự phụ thuộc

```
020-01 ─▶ 020-02 ─▶ 020-03 ─┐        020-01 ─▶ 020-04 ─▶ 020-06 ─▶ 020-07 ─▶ 020-08
                            │        020-01 ─▶ 020-05 ─────────────▲
023-01 ─▶ 023-02 ─▶ 023-03   │        023-01 ─▶ 023-04 ; 023-05 ─▶ 023-06 ; 023-07 ─▶ 023-08 ─▶ 023-09
021-01 ─▶ 021-02 ─▶ 021-03 ─▶ 021-04 ─▶ 021-05 ; (020-06,020-07) ─▶ 021-06 ; (020-03) ─▶ 021-07 ; 021-04 ─▶ 021-08 ; 021-06,07,08 ─▶ 021-09
022-01 ─▶ 022-02 ─▶ 022-03 ─▶ 022-05 ─▶ 022-06 ; 022-04 ─▶ 022-05 ; 022-07 ; (022-05,06,07,021-09) ─▶ 022-08
024-01 ─▶ 024-02, 024-03, 024-04 ─▶ 024-05 ─▶ 024-06 ; 024-05 ─▶ 024-07 ─▶ 024-08 ─▶ 024-09
Liên solution: SOL-020 ─▶ SOL-021 ─▶ SOL-022 ─▶ SOL-024 ; SOL-023 ─▶ SOL-021 (021-03) và SOL-024 (024-03)
```

Làm song song được: toàn bộ SOL-020 với SOL-023; 020-04 với 020-05; 023-04 với 023-05; 022-02 với 022-04.

## Ghi chú

- **Số migration/bảng:** không task nào ở đây tạo bảng; `graph_snapshots`, `reindex_jobs`, `outbox_events`, `processed_events` do BE-CV-SOL-011. Mỗi task re-verify đọc `ls migrations/postgres` trước.
- **Proto:** gọi `buf lint` và `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` **trực tiếp** (Makefile `proto-lint` có `|| true`). Task proto của SOL-023 (06, 08) cùng sửa `infrafleet.proto`: sinh lại stub khi rebase.
- **Hai dialect + tenant:** mọi task có DB chạy `-tags=integration` PG và MySQL, kiểm cô lập tenant; Postgres cần role `NOSUPERUSER NOBYPASSRLS` (dev compose dùng superuser nên RLS không có hiệu lực).
- **Agent:** task phụ thuộc hình `data` của agent đối chiếu tệp vàng G1 (`AG-CV-SOL-070`); trước khi có, dùng mẫu hợp đồng và ghi "chưa kiểm chứng".
- **Sau feature này:** `code-intel-gateway` (BE-CV-SOL-040-*) tiêu thụ `StreamCodeIntelEvents`, `CachedViewReader`; `quality-signals` (SOL-082, 080) tiêu thụ `AgentRPCCaller`, `QualityRunEventSink`.

# BE-CV-TASK-071-05: Metrics `orca_fleet_codeintel_*` và span `devserveragent.call` ở infra-fleet-service

**From Solution:** BE-CV-SOL-071
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/metrics/codeintel_collector.go` (mới), `.../internal/adapter/metrics/codeintel_collector_test.go` (mới), `.../internal/adapter/devserveragent/call_span.go` (mới), `.../cmd/server/main.go`
**Depends on:** BE-CV-SOL-023-infra-fleet-codeintel-transport (`StreamCodeIntelEvents`, bảng timeout theo method, `AgentRPCError`)
**Status:** `[ ] TODO`

---

## Context

- infra-fleet có registry `fleet` riêng, mount `/health/metrics` (`cmd/server/main.go:784`), `inframetrics.NewFleetCollector()` (dòng 490), tiền tố `orca_fleet_*`. Đây là ngoại lệ so với `/metrics`; **giữ nguyên** (không nhân rộng sang code-intel).
- PQ-17: thông báo chuyển tiếp gồm `index_changed`, `reindex_progress`, `quality_progress`, `quality_finished`, `resync`, `overflow`; PQ-18: tối đa 16 luồng mỗi `(tenant, dev_server)`.
- Bảng timeout theo method thuộc BE-CV-SOL-023; solution này **chỉ đo**, không sửa `execTimeoutForMethod`.
- infra-fleet có sẵn `otelgrpc` server (`grpcmw.StatsHandler`) và client auth (`main.go:401`).

## Việc cần làm

1. `codeintel_collector.go`: `orca_fleet_codeintel_notifications_total{kind}`, `orca_fleet_codeintel_notifications_dropped_total{reason}` (`reason ∈ no_subscriber|buffer_full|stream_closed`), `orca_fleet_codeintel_relay_seconds{class}` (`class ∈ read|detect|quality_list|quality_other`; map từ method theo agent-rpc §2.5, method lạ → `other`), `orca_fleet_codeintel_streams_active`. Đăng ký vào registry `fleet` hiện có; không thêm nhãn tenant/dev server.
2. `call_span.go`: bọc `Client.Exec` cho method `codeintel.*`/`quality.*` bằng span `devserveragent.call` (thuộc tính `method` từ tập đóng, `class`); lỗi gán `span.RecordError` **không** chứa `data` agent thô.
3. Gắn vào luồng `StreamCodeIntelEvents` (đếm theo `kind`, `streams_active` ± khi mở/đóng).

## Kiểm thử

- Test collector: nhãn là tập đóng; `kind` lạ → `other`; không nhãn tenant/dev server.
- Test span (tracer trong bộ nhớ): span cha là span gRPC server của `RelayByDevServer`, thuộc tính an toàn.
- `go test ./services/infra-fleet-service/... -run 'CodeIntel|CallSpan'`.

## Tiêu chí hoàn thành

- [ ] `/health/metrics` có bốn họ metric mới.
- [ ] Không đổi hành vi timeout.
- [ ] Không rò `error.data` vào span/log.

## Rủi ro và lưu ý

- Sửa `main.go` của infra-fleet (service trung tâm): chạy `gitnexus_impact` trước và báo blast radius.
- `class` gom theo bảng timeout; nếu §2.5 đổi nhóm, đổi tập nhãn cùng PR hợp đồng.

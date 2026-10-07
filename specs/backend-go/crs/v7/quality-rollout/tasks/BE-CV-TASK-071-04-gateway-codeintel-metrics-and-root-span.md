# BE-CV-TASK-071-04: Metrics và span gốc cho kênh `codeIntel.*` ở api-gateway, ghép `/metrics`

**From Solution:** BE-CV-SOL-071
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/codeintelmetrics/metrics.go` (mới), `.../codeintelmetrics/instrument.go` (mới), `.../codeintelmetrics/metrics_test.go` (mới), `backend-go/services/api-gateway/cmd/server/mcp_metrics_wiring.go`, `backend-go/services/api-gateway/cmd/server/main.go`
**Depends on:** BE-CV-SOL-040-codeintel-channel-foundation (đăng ký kênh, `codeIntelChannelError`, `SetReadLimit`)
**Status:** `[x] DONE`

---

## Context

- `healthAndMetricsMux(health http.Handler, m *mcpmetrics.Metrics)` ở `cmd/server/mcp_metrics_wiring.go:39`; `main.go:587` gọi với `mcpMetrics`; `mcpmetrics.Metrics` có `Registry()` (`metrics.go:89`) và `Handler()` (dòng 92) ⇒ ghép bằng `prometheus.Gatherers` khả thi (SOL-071 C1).
- `wscompat/registry.go`: `ChannelHandler`, `StreamChannelHandler`, `Register`, `RegisterStreamChannel`; `handler.go:247` `invokeTimeout = 25 s`.
- Không có span theo kênh (SOL-071 C2). Mẫu span: `mcpmetrics/instrument.go` (`tracer()`, `mcp.dispatch`).
- Tập `channel` = 46 kênh (ui-api §3), lấy từ danh sách đăng ký, không viết tay (L2).
- **Trước khi sửa `healthAndMetricsMux`/`main.go` chạy `gitnexus_impact` và báo blast radius** (CLAUDE.md của repo).

## Việc cần làm

1. Gói `codeintelmetrics`: registry riêng, `orca_gateway_codeintel_channel_total{channel,result}`, `..._channel_duration_seconds{channel}`, `..._response_bytes{channel}`, `orca_gateway_codeintel_streams_active`, `orca_gateway_codeintel_streams_limit`, `orca_gateway_codeintel_events_dropped_total{reason}`; `result` theo bảng mã (L3) lấy từ chuỗi lỗi `CODEINTEL_*` đã qua `codeIntelChannelError`.
2. `instrument.go`: `WrapChannel(name, h ChannelHandler) ChannelHandler` và `WrapStream(...)`: đo thời lượng, bọc span `codeintel.channel <kênh>` (tracer `orca/api-gateway/codeintel`, thuộc tính `codeintel.channel`, `codeintel.result`), ghi `proto.Size` của phản hồi nếu có. `registerCodeIntelChannels` (CR-040) gọi wrapper cho mọi kênh.
3. `mcp_metrics_wiring.go`: đổi chữ ký để nhận `prometheus.Gatherer` hoặc hai registry; dùng `promhttp.HandlerFor(prometheus.Gatherers{...}, ...)`. Giữ `/healthz`, `/readyz` như cũ; vẫn **không** mount trên cổng công khai.
4. `streams_limit` đặt từ `CODE_INTEL_MAX_STREAMS` (500 mặc định, PQ-23).

## Kiểm thử

- Test gói: tập `channel` bằng tập kênh đăng ký (đỏ khi thêm kênh mà thiếu wrapper); `result` lạ → `other`; không nhãn tenant/worktree.
- Test wiring: `/metrics` chứa cả `orca_mcp_requests_total` và `orca_gateway_codeintel_channel_total`.
- Test span (tracer trong bộ nhớ): một lần gọi kênh tạo span có tên đúng, thuộc tính an toàn.
- `go test ./internal/adapter/codeintelmetrics/... ./cmd/server/...`; chạy lại bộ `MCP|Mcp` ở `run-go-conformance.sh` để chắc không phá `/metrics` MCP.

## Tiêu chí hoàn thành

- [x] `/metrics` gateway phục vụ cả hai họ metric.
- [x] Mọi kênh `codeIntel.*` có số đo và span mà không sửa từng handler.
- [x] Test conformance MCP vẫn xanh.

## Rủi ro và lưu ý

- Ctx của `handleInvoke` có kế thừa span `otelhttp` của nâng cấp WS hay không: chưa kiểm chứng; span kênh có thể thành gốc mới hoặc con của span kết nối dài (cả hai chấp nhận được, ghi lại kết quả quan sát vào PR).
- SSH/remote: độ trễ 50–200 ms/hop không nằm trong số đo gateway.

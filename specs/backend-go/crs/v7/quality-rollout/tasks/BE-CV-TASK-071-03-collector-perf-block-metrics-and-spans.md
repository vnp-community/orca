# BE-CV-TASK-071-03: Đọc khối `perf` của agent: histogram, span con, không lưu không trả

**From Solution:** BE-CV-SOL-071
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/agent_perf_observer.go` (mới), `.../internal/usecase/agent_perf_observer_test.go` (mới), `.../internal/adapter/otel/agent_perf_spans.go` (mới), `.../internal/adapter/otel/agent_perf_spans_test.go` (mới); sửa nhẹ collector của BE-CV-SOL-021
**Depends on:** BE-CV-TASK-071-02, BE-CV-SOL-021 (collector), `AG-CV-SOL-071-perf-block-and-bench` (agent phát `perf`), BE-CV-TASK-070-02 (tệp vàng có `perf`)
**Status:** `[x] DONE`

---

## Context

- agent-rpc §2.2: `perf = {totalMs, queueWaitMs, cliCalls, cli:[{tool,command,ms,stdoutBytes,rssPeakKb}], parseMs, truncated}`; `command` chỉ là hằng whitelist; `perf` "chỉ cho backend ghi metric/span; KHÔNG ra UI, KHÔNG vào cache snapshot".
- Agent không có OTel (`agent/package.json` không có `@opentelemetry/*`), nên span agent là **tổng hợp hậu kỳ** (CR-071 D4): thời điểm bắt đầu suy ra từ `end - ms`.
- Lệnh ngoài tập `perf` (L11) → nhãn `other`.

## Việc cần làm

1. Cổng `AgentPerfObserver` ở `usecase`: `Observe(ctx, method string, perf AgentPerf)`; `AgentPerf` giải mã từ `map[string]any`, mọi trường tuỳ chọn, giá trị âm/NaN bị bỏ.
2. Hiện thực metric: `agent_queue_wait_seconds{tool}`, `agent_cli_seconds{tool,command}`, `agent_rss_peak_bytes{tool}` (`rssPeakKb`×1024; thiếu thì bỏ), `agent_payload_bytes{method}` (kích thước JSON `data`), `response_truncated_total{method}` khi `perf.truncated` hoặc phong bì `truncated`.
3. Hiện thực span: dưới span `codeintel.collect`, tạo `codeintel.agent.queue`, `codeintel.agent.cli` (thuộc tính `tool`, `command`, `bytes`) và `codeintel.agent.parse` bằng `trace.WithTimestamp`; thuộc tính chỉ từ tập đóng/số.
4. Collector gọi `Observe` rồi **xoá** `perf` khỏi dữ liệu đi tiếp (lưu snapshot, trả gateway).
5. Thiếu `perf` hoặc sai kiểu: không lỗi, tăng bộ đếm `orca_codeintel_agent_perf_invalid_total` (một metric nhỏ, không nhãn).

## Kiểm thử

- `collector_perf_test.go` (đặt tên theo CR): tệp vàng có `perf` → histogram đúng, span con đúng cha; thiếu `perf` không lỗi; `perf` không có trong đầu ra lưu.
- Bảng `command` lạ (`analyze`, `check`) → `other`.
- Test thuộc tính span: không chứa đường dẫn, symbol.
- `go test ./internal/usecase/... ./internal/adapter/otel/...`; không DB.

## Tiêu chí hoàn thành

- [x] `perf` không lọt vào snapshot hay phản hồi gateway (test).
- [x] Mọi metric nhóm agent có dữ liệu từ tệp vàng.
- [x] Không panic với `perf` sai hình.

## Rủi ro và lưu ý

- Span hậu kỳ mất thứ tự thật giữa lệnh CLI song song; ghi chú trong mã (một dòng).
- Phụ thuộc agent phát `perf`; trước đó tệp vàng tự tạo.

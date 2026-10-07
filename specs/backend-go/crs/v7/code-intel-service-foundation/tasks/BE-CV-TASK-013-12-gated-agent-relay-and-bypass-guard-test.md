# BE-CV-TASK-013-12: `GatedAgentRelay` (đường duy nhất tới `RelayByDevServer`) và test chặn bỏ qua cổng

**From Solution:** BE-CV-SOL-013-agent-call-gate-and-quotas
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpcclient/gated_agent_relay.go`, `gated_agent_relay_test.go`, `no_relay_bypass_test.go` (mới); `internal/usecase/ports.go` (cổng `AgentRelay`)
**Depends on:** BE-CV-TASK-013-09, BE-CV-TASK-012-04
**Status:** [x] DONE

---

## Context

SOL-013-gate mục 2.C. `RelayByDevServer{dev_server_id, method, params_json}` → `RelayResponse` (hợp đồng §3.3). SOL-012 (probe trạng thái), SOL-021 (collector), SOL-030 (đọc file) đều phải đi qua cổng này. PQ-13: khe nhả khi lời gọi thật kết thúc (singleflight 100 s ở collector); timeout Go 30/90/45 s theo method do infra-fleet.

## Việc cần làm

1. Cổng `usecase.AgentRelay{Call(ctx, devServerID, method string, params map[string]any) (map[string]any, error)}`.
2. `GatedAgentRelay{inner RawRelay, gate AgentCallGate, redact SecretRedactor}`: `class := ClassOf(method)`; `Acquire` (lấy `tenant`, `user` từ ctx) → mã hoá `params` thành `params_json` (chỉ khoá do caller cung cấp; **không** thêm lệnh/`env`/`cwd`/`timeout`; kiểm không có khoá `command|argv|args|env|cwd|timeout` ở mọi tầng — H3) → `RelayByDevServer` → `release` trong `defer` chạy khi lời gọi thật xong (kể cả khi ctx người gọi đã huỷ: dùng goroutine nền nếu lời gọi tiếp tục).
3. Kết quả: giải mã `result_json` vào `map[string]any`; kích thước ≤ 16 MiB (trần nhận); quá → `CODEINTEL_OUTPUT_TOO_LARGE`.
4. Lỗi agent: `FailedPrecondition`+`INFRA_DEV_SERVER_NOT_CONNECTED` → `CODEINTEL_DEV_SERVER_OFFLINE` (`KindUnavailable`); khác giữ mã thô, chuỗi lỗi qua bộ che và cắt 500 ký tự (trước SOL-023 mất mã agent; sau SOL-023 mã `CODEINTEL_*` đi qua).
5. `no_relay_bypass_test.go`: quét AST cả module (trừ file này) — không nơi nào gọi `.RelayByDevServer(` hoặc nhập `InfraFleetServiceClient` ngoài `gated_agent_relay.go` và file dial; đỏ nếu có.
6. Tiêm `GatedAgentRelay` vào prober trạng thái (012-10) thay cổng tạm.

## Kiểm thử

- Unit: lời gọi mỗi lớp đi qua `Acquire` đúng lớp (gate giả ghi lại); `release` luôn chạy khi lỗi/panic (`defer`); huỷ ctx giữa chừng không rò; khoá cấm trong `params` bị từ chối; lỗi offline ánh xạ đúng; chuỗi lỗi có bí mật bị che.
- `go test -race ./services/code-intel-service/internal/adapter/grpcclient/...`

## Tiêu chí hoàn thành

- [x] Mọi lời gọi agent đi qua cổng; test AST đỏ khi thêm đường tắt.
- [x] Không rò khe khi huỷ/lỗi.
- [x] Không khoá lệnh/`env`/`cwd`/`timeout` xuống agent.

## Rủi ro và lưu ý

- SOL-021 phải dùng cổng này, không dial riêng; ghi vào README service.
- Khe giữ lâu hơn ctx người gọi khi lời gọi nền tiếp tục (PQ-13); giới hạn đồng thời vẫn bảo vệ dev server.

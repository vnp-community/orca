# backend-go Solutions — MCP Quality & Rollout (v5)

**CR:** [docs/crs/v5/mcp-quality-rollout](../../../../../../docs/crs/v5/mcp-quality-rollout/README.md) (CR-MCP-015)
**Hợp đồng FE:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Quy ước chung + T1..T8:** [crs/v5/README.md](../../README.md)
**TDD tham chiếu:** [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) · [`arch/10`](../../../../tdd/architecture/10-deployment-infrastructure.md) (CI/CD) · [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md)

## Re-verify trước khi thiết kế (CR vs mã thật, 2026-10-01)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| "Metrics Prometheus theo chuẩn service khác" | Chỉ **một** service có Prometheus (`infra-fleet-service`: `prometheus.NewDesc("orca_fleet_*")`, phục vụ `/health/metrics`). `common/health.Handler()` **không** có `/metrics` (comment của nó nói "mount alongside /metrics" nhưng không mount). `api-gateway` dùng `otelhttp` + `tracing.Init`, không có endpoint metrics. ⇒ không có "chuẩn" để theo, BE-015 phải dựng đường scrape | **Lệch** |
| Tên metric `mcp_requests_total…` | Tiền tố hiện hữu là `orca_<svc>_*` (`orca_fleet_*`) và T8 chốt `orca_mcp_*` | Đổi tên theo T8 |
| "Chạy bộ kiểm chứng/Inspector chính thức (CLI headless)" | Không có gói/CI nào cho MCP; không thể kiểm trong môi trường này gói `@modelcontextprotocol/inspector` có chế độ `--cli` và gói conformance chính thức hay không ⇒ **(chưa xác minh)** | Chưa xác minh ⇒ Inspector chỉ là tầng 2 không chặn. **Đã chốt (D4, 2026-10-01):** client tham chiếu chính thức là MCP Python SDK (PyPI `mcp`) trong job Python `backend-go/ci/mcp-conformance/` (phiên bản ghim; API chưa xác minh — kiểm khi cài đặt); harness Go giữ cho test in-process |
| "CI hiện có (`backend-go/ci`, Makefile `test-integration`)" | `backend-go/ci/` chỉ có 2 script bash; CI thật ở `.github/workflows/backend-go-<svc>.yml` (16 file, **không có cho `api-gateway`**), `Makefile` `proto-lint` kết thúc `|| true` (không chặn). Không có workflow dựng cả stack | **Lệch**: CI conformance là workflow mới |
| "Cập nhật `docs/hld` … C4-code.md:1798" | Đúng: `docs/hld/v1/C4-code.md:1798` có dòng `tools/list, tools/call, # MCP layer — không tài liệu hoá ở đâu khác`. Ngoài ra có `docs/hld/backend-go-architecture.md`, `docs/hld/v1/C2-containers.md` | Không |
| ADR | `docs/adrs/` có `v1/`, `v2/` (mới nhất `ADR-020-…`), không phẳng | Đặt ADR tại `docs/adrs/v2/ADR-02x-*.md` |
| Suite kiểm tra API có sẵn | `tests/backend/*.py` (Python; `run_all.py` `SUITES`, `orca_route_catalog.py` đối chiếu route với mã Go, commit `9a928b033`) | Dùng làm nơi đặt kiểm tra hợp đồng HTTP/WS của MCP |

## Solutions

| Solution | CR | Service / Area | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-015](./BE-MCP-SOL-015-conformance-e2e-observability-rollout.md) | CR-MCP-015 | `api-gateway`, `mcp-service`, `.github/workflows`, `backend-go/tests/mcpconformance` (mới), `tests/backend`, `docs/*` | Medium (rải theo các CR) | 🟡 Partially implemented — see BE-MCP-SOL-015 status banner |

## Thứ tự thực thi & phụ thuộc

BE-015 chạy **xuyên suốt**, không phải bước cuối:

| Mốc | Làm ngay khi | Việc |
|---|---|---|
| M0 | BE-001/002 xong | dựng `/metrics` + `Recorder` thật; workflow CI khung (lint/test); script nginx |
| M1 | BE-003 xong | harness conformance giao thức (lifecycle, version, lỗi); khoá `protocol_versions.go`; job Python SDK tham chiếu (`initialize`, `tools/list`, `tools/call`) |
| M2 | BE-004 xong | resume/cancel/đa replica trong harness (+ resumability trong job Python) |
| M3 | BE-005/006 xong | luồng OAuth đầy đủ + ca âm tính; PAT (+ OAuth discovery trong job Python) |
| M4 | BE-007/008 (đợt 1–2) xong | parity UI↔MCP, e2e agent giả lập |
| M5 | BE-012/013 xong | red-team, approval, alert, runbook; **mốc an toàn bắt buộc trước tool ghi** |
| M6 | trước GA | review bảo mật độc lập, cập nhật toàn bộ TDD (T1..T8), README "Real/Stub" trung thực |

Phụ thuộc phía FE: [FE-MCP-SOL-012](../../../../../frontend/crs/v5/mcp-quality-rollout/solutions/FE-MCP-SOL-012-testing-telemetry-and-rollout.md) dùng cùng cờ rollout và cùng backend dev; hợp đồng test e2e FE phụ thuộc M3–M5.

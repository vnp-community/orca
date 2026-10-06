# Feature: quality-rollout — Chất lượng, hiệu năng, bảo mật và rollout của "Xem code"

> **Trạng thái:** 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code và chạy thử CLI chỉ-đọc ngày 2026-10-05; chưa chạy hệ thống, chưa viết code.
> **Hợp đồng chung:** [`../README.md`](../README.md). Mẫu cấu trúc: [`../../v6/request-quality-rollout/`](../../v6/request-quality-rollout/README.md).

## 1. Mục tiêu

Biến series v7 từ "chạy được trên máy một người" thành "an toàn để bật cho tenant": (1) hợp đồng với hai công cụ ngoài (GitNexus, CodeGraph) được khoá bằng fixture vàng và phát hiện trôi định dạng; (2) có ngân sách hiệu năng, metrics và trace đo được; (3) có kiểm thử bảo mật cho bề mặt mới (agent chạy CLI, đọc mã nguồn, nhiều repo, nhiều tenant); (4) có e2e, cờ `code_intel_enabled` theo tenant, kế hoạch rollout, quay lui và runbook.

## 2. Danh sách CR

| CR | Tên | Priority | Effort | Phụ thuộc | Mở khoá |
|---|---|---|---|---|---|
| [CR-CV-070](./CR-CV-070-golden-fixtures-and-tool-contract-tests.md) | Fixture vàng và kiểm thử hợp đồng với phiên bản công cụ | 🔴 P0 | Medium | CR-CV-001, 002 (cần parser để kiểm); CR-CV-003 cho phần CodeGraph | CR-CV-071, 072, 073; khuyến nghị làm ngay sau CR-CV-002/003 |
| [CR-CV-071](./CR-CV-071-performance-budgets-metrics-tracing.md) | Ngân sách hiệu năng, metrics, tracing | 🟠 P1 | Medium | CR-CV-010, 021, 023, 040; CR-CV-070 (dữ liệu benchmark) | CR-CV-073 (điều kiện chuyển giai đoạn) |
| [CR-CV-072](./CR-CV-072-security-tests.md) | Kiểm thử bảo mật: whitelist, đường dẫn, che secret, cô lập tenant | 🔴 P0 | Medium | CR-CV-001, 012, 013, 030, 040 (và 035, 041 nếu có) | CR-CV-073 (gate bật cho người dùng) |
| [CR-CV-073](./CR-CV-073-e2e-feature-flag-rollout-runbook.md) | E2E, cờ `code_intel_enabled`, rollout, tài liệu vận hành | 🔴 P0 | Medium | CR-CV-010 đến 013, 040, 050, 051; phần cờ chỉ cần CR-CV-010, 013 | GA của tính năng |

## 3. Thứ tự thực thi

```
CR-CV-002/003 ─▶ CR-CV-070 (ngay) ─▶ CR-CV-071 ─┐
CR-CV-013/040 ─▶ CR-CV-072 ─────────────────────┼─▶ CR-CV-073 (gate + rollout)
CR-CV-010/013 ─▶ phần cờ của CR-CV-073 (làm sớm)┘
```

Phần **cờ** của CR-CV-073 làm sớm (ngay sau CR-CV-010 và 013) để mọi RPC có cổng từ ngày đầu; phần e2e, rollout và runbook làm cuối.

## 4. Quyết định chung của feature

| # | Quyết định | Lý do |
|---|---|---|
| Q1 | Fixture lưu **mẫu nhỏ chụp từ một repo mẫu bé** (kèm bản kê hash), không chụp từ repo Orca | Orca đổi từng commit (số node `codegraph status` đã lệch giữa hai lần chạy cùng ngày); mẫu lớn tốn kho và dễ gây thay đổi vô nghĩa |
| Q2 | Kiểm thử hợp đồng chia hai tầng: **offline chặn PR** (parser so với fixture) và **live định kỳ không chặn** (cài bản công cụ mới nhất, chụp lại, so khớp) | Công cụ ngoài không ổn định; không để bản phát hành của bên thứ ba làm đỏ PR của ta, nhưng phải báo sớm |
| Q3 | Trôi định dạng không được im lặng: parser kiểm hình dạng, sai thì lỗi `CODEINTEL_TOOL_FAILED` kèm chi tiết `format_drift` và tăng metric | Kết quả sai lặng lẽ tệ hơn lỗi rõ ràng |
| Q4 | Metrics theo quy ước repo: tiền tố `orca_`, registry riêng, nhãn từ tập đóng, **không** nhãn tenant/user/worktree/đường dẫn | Theo `mcpmetrics` (quy tắc cardinality) |
| Q5 | Bảo mật kiểm bằng test tự động ở cả ba tầng (agent, service, gateway), ưu tiên "từ chối + không lộ oracle" | AGENTS.md và README v7 mục 6: không có đường chạy lệnh tự do, không lộ secret |
| Q6 | Cờ theo tenant, thi hành ở `code-intel-service`, fail closed, mặc định tắt (O8) | Một điểm thi hành; theo mẫu cờ của mcp-service và v6 CR-REQ-025 |

## 5. Phát hiện chung (cần người duyệt xem)

- **CI chưa chạy test của `agent/`.** Không workflow nào trong `.github/workflows/` nhắc `agent/` hay `orca-agent` (grep 2026-10-05), và `pr.yml` gọi vitest với `config/vitest.config.ts` ở gốc repo mà file đó không tồn tại (có `agent/vitest.config.ts` riêng, `include: src/**/*.test.ts`). Test parser của CR-CV-070 cần một job CI mới hoặc xác nhận cách `agent/` đang được kiểm.
- **Không có OTel phía agent, không có truyền `traceparent` qua JSON-RPC agent** (`agent/package.json` không có `@opentelemetry/*`; grep `traceparent` chỉ ra `common/tracing` và một test của task-service). Chuỗi trace agent → infra-fleet bị đứt; CR-CV-071 đề xuất khối `perf` trong kết quả thay vì thêm OTel vào agent.
- **Kênh WS của gateway không tạo span** (không `tracer.Start`/`otelhttp` ở `wscompat` và `httpgateway`, chỉ có trong test); span gốc hiện là span client gRPC đầu tiên.
- **`/metrics` không nhất quán giữa service**: `mcp-service`, `notification-service`, `api-gateway` mount `/metrics` trên cổng health; `infra-fleet-service` mount `/health/metrics` (`cmd/server/main.go:784`); `issue-status-sync` không có metrics (đã kiểm lại). `code-intel-service` nên theo `/metrics`.
- **Rule cảnh báo chưa được nạp ở đâu**: `backend-go/deploy/alerts/mcp.rules.yaml` ghi rõ "Not loaded by anything in this repository". Cảnh báo của CR-CV-071 sẽ ở cùng tình trạng cho tới khi có hạ tầng nạp rule.
- **`agent/` và `desktop/` đều có `src/relay/agent-tool-registry.ts`** (GitNexus báo hai symbol `runToolCommand`); CR-CV-001 phải chốt bản nào là bản chạy trên dev server; fixture và test của CR-CV-070 đặt theo bản đó.
- CodeGraph được cài vào `~/.codegraph/versions/v1.4.1/` (dạng nhiều phiên bản cạnh nhau, `readlink` của `codegraph`): có thể tự nâng cấp; cần theo dõi phiên bản thực tế trên dev server (CR-CV-070, 073).

- **`gitnexus analyze` ghi vào cây làm việc theo mặc định** (chèn mục vào `AGENTS.md`, `CLAUDE.md`, cài `.claude/skills/gitnexus/`, ghi `~/.gitnexus/registry.json`); chỉ `--index-only` tắt việc chèn tệp. `codeintel.reindex` (CR-CV-004) phải truyền cờ này, nếu không mỗi lần reindex làm bẩn diff đang review. Kiểm ở CR-CV-072 và e2e E05 của CR-CV-073.
- **Tiêm tuỳ chọn CLI đã tái hiện**: `gitnexus impact -r orca handleInvoke "--repo=vnp-workplace"` trả dữ liệu repo khác; cần `--` và từ chối giá trị bắt đầu bằng `-` (CR-CV-072). `gitnexus cypher` không có tham số hoá, và `CALL show_tables()` chạy được.
- **`fs.*` của agent không giới hạn workspace root** (`fs.readFile` dùng nguyên đường dẫn tuyệt đối); `secureFs` mà nghiên cứu 02 nhắc không tồn tại trong code. Ảnh hưởng D6/CR-CV-030.

## 6. Ngoài phạm vi

- Chất lượng kết quả heuristic của C4 và luồng dữ liệu (độ đúng của suy luận): chỉ kiểm khung và nhãn "suy luận", không chấm đúng/sai nghiệp vụ.
- Kiểm thử thị giác (visual regression) cho đồ thị: không có trong v7.
- Kiểm thử trên Windows/WSL dev server thật: ghi nhận là rủi ro (AGENTS.md); fixture có biến thể CRLF để giảm rủi ro, không thay thế chạy thật.
- Pentest bên ngoài: CR-CV-072 chỉ là kiểm thử tự động và threat model ngắn.

## 7. Tài liệu liên quan

- `docs/crs/v6/request-quality-rollout/README.md`, `CR-REQ-025-e2e-tests-feature-flag-rollout.md`
- `docs/crs/v5/mcp-quality-rollout/CR-MCP-015-conformance-e2e-observability-rollout.md`
- `tests/mcp/README.md`, `tests/e2e/AGENTS.md`, `backend-go/ci/mcp-conformance/run-go-conformance.sh`

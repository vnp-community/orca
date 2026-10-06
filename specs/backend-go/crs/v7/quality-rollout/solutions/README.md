# quality-rollout: solutions (backend-go, v7)

> 📋 Proposed. Chưa triển khai, chưa chạy. Viết ngày 2026-10-06 từ việc đọc CR-CV-070..073 (và phần server của CR-CV-095), ba hợp đồng v7 và code hiện có. `code-intel-service` và `proto/orca/codeintel/` chưa tồn tại; mọi tên "(mới)" là đề xuất.
> CR nguồn: [`docs/crs/v7/quality-rollout/`](../../../../../../docs/crs/v7/quality-rollout/README.md). Hợp đồng (thắng CR khi khác): [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md), [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md), [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md).

## CR → Solution

| CR | Solution | Nội dung | Service | Priority | Đối ứng khu vực khác (hợp đồng §8.2) |
|---|---|---|---|---|---|
| CR-CV-070 | [BE-CV-SOL-070](./BE-CV-SOL-070-collector-golden-contract.md) | tệp vàng `testdata/agent-results`, test collector, vector `SymbolRef`, ánh xạ lỗi, workflow `code-intel-contract.yml` | `code-intel-service`, `.github` | P0 | AG `AG-CV-SOL-070-golden-fixtures-and-parsers` |
| CR-CV-071 (+ phần server CR-CV-095) | [BE-CV-SOL-071](./BE-CV-SOL-071-metrics-tracing-and-budgets.md) | ngân sách dữ liệu, metrics 3 service, khối `perf`, tracing, benchmark, cảnh báo, số liệu cổng chất lượng | `code-intel-service`, `api-gateway`, `infra-fleet-service`, `deploy/alerts` | P1 | AG `AG-CV-SOL-071-perf-block-and-bench`; FE `FE-CV-SOL-095-review-telemetry` |
| CR-CV-072 | [BE-CV-SOL-072](./BE-CV-SOL-072-security-tests-service-gateway.md) | vector đường dẫn, fuzz gateway/service, RLS thật, cô lập tenant, ma trận quyền 49 RPC, canary secret, DoS, workflow bảo mật | `code-intel-service`, `api-gateway` | P0 | AG `AG-CV-SOL-072-security-tests-agent` |
| CR-CV-073 | [BE-CV-SOL-073](./BE-CV-SOL-073-settings-flag-and-rollout.md) | `GetSettings`/`SetSettings`, hiệu lực cờ, interceptor cờ, script kiểm đăng ký, e2e T1/T3, rollout, runbook | `code-intel-service`, `backend-go/ci`, `tests/code-intel`, `docs` | P0 | AG `AG-CV-SOL-073-agent-kill-switch`; FE `FE-CV-SOL-073-flag-gating-and-web-e2e` |

## Thứ tự phụ thuộc

```
G0 (CR-010, 020) ─▶ G1: AG-070 sinh tệp vàng ─▶ BE-070 ─┐
                                                       ├─▶ BE-073 e2e (dùng tệp vàng)
CR-010, 011, 013 ─▶ BE-073 phần cờ (task 01-04) ───────┤
lõi ổn định (021, 023, 040) ─▶ BE-071 ─────────────────┤
CR-013, 030, 040 ─▶ BE-072 ────────────────────────────┴─▶ cổng GA (rollout giai đoạn 0 → 4)
```

Phần cờ của SOL-073 (task 01–04) vào sớm (đợt 3); SOL-070 sau CR-021 (đợt 5); SOL-071 và SOL-072 khi lõi ổn định (đợt 5); e2e và tài liệu của SOL-073 làm cuối.

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| Q1 | Tệp vàng agent nằm cạnh service Go; agent sinh, Go đọc, không có `-update` phía Go | một nguồn; thay đổi hình dạng làm đỏ hai phía cùng PR |
| Q2 | Tầng offline chặn PR, tầng live/nightly (công cụ thật, stack thật, benchmark) không chặn | công cụ ngoài và stack đầy đủ không ổn định |
| Q3 | Ngân sách là dữ liệu JSON; ô chưa đo là `null` + cảnh báo, không bịa số | mọi con số là giả định (O-15) |
| Q4 | Nhãn metric/ma trận kiểm thử sinh từ nguồn mã (`ServiceDesc`, registry kênh, bảng mã lỗi), không viết tay | CR đếm tay đã lệch hợp đồng (25 so với 49 RPC; 26 so với 46 kênh) |
| Q5 | Cô lập tenant kiểm bằng vai trò `NOSUPERUSER NOBYPASSRLS` + `FORCE` + quét `tenant_id` hai dialect | RLS không có tác dụng với chủ sở hữu; dev compose dùng superuser |
| Q6 | Cờ thi hành ở `code-intel-service` bằng interceptor có bảng phân loại, fail closed, cache 5 s; không có migration ở SOL-073 (bảng T1 thuộc CR-011) | PQ-01, PQ-24 |
| Q7 | Rollback là tắt cờ/cắt gateway, không `down` migration khi còn dữ liệu | không mất dữ liệu review |

## Lệch giữa CR và hợp đồng (tóm tắt; chi tiết ở từng solution mục 3)

- Mã lỗi: `detail` (CR-070) so với `reason` (hợp đồng); `forbidden`/`NotFound` (CR-071/072) so với `CODEINTEL_NOT_AUTHORIZED`/`CODEINTEL_NOT_FOUND` (PQ-03); `CODE_INTEL_ENABLED`/`migrate-code-intel` (CR-073) so với `CODEINTEL_ENABLED`/`migrate-codeintel` (PQ-23).
- Bảng `tenant_settings` do CR-011 tạo đủ cột (không phải CR-073).
- `GetArchitecture` là C4 (PQ-10), đồ thị cụm là `GetClusterOverview`.
- Trần: snapshot 3 MiB, chờ slot 10 s, stdout 16 MiB, JSON cuối 8 MiB, `OUTPUT_TOO_LARGE` > 12 MiB.

## Khoảng trống hợp đồng cần người quyết định (không tự sửa hợp đồng)

1. `codeintel.status` không có `compatibility`/trạng thái `untested`; `data` của `TOOL_FAILED` không có `command`; tập `perf.command` thiếu `check`, `trace`, `list`, `analyze`, `sync`; tập `warnings[]` ổn định chưa có.
2. Không có enum `false_positive` ở `finding_dismissals.reason`/`quality_waivers` ⇒ tỉ lệ báo nhầm (CR-095 §2.8) chỉ xấp xỉ.
3. Chủ sở hữu `feature_gate.go`/bộ đọc cờ giữa BE-CV-SOL-013 và 073; chủ sở hữu file workflow `code-intel-contract.yml`.
4. Bảng vai trò × action cho 49 RPC không nằm trong ba hợp đồng (nằm ở CR-013/085).

## Phát hiện đáng chú ý đã đối chiếu code

- CI không chạy test `agent/` (không workflow nào nhắc `agent/`); `pr.yml` trỏ `config/vitest.config.ts` không tồn tại; 18 workflow `backend-go-*`; Go CI 1.25 so với `go.work` 1.26.0; gRPC không đặt `MaxCallRecvMsgSize`; gateway không gọi `SetReadLimit`.
- `mcpmetrics.Metrics` có `Registry()` ⇒ ghép `/metrics` bằng `prometheus.Gatherers` khả thi; `/ws` có một span `otelhttp` cho cả kết nối nhưng không có span theo kênh.
- `mcp-service` đã có mẫu test RLS vai trò không phải chủ sở hữu và test AST "mỗi method repository chạy trong tx có tenant" để sao chép.
- `AppendAuditEntryRequest` đã có `actor_type`/`target_type`; `auditclient.Append` chưa dùng và nuốt lỗi.
- Hai tệp `init-databases.sh` khác nhau (backend-go và deploy/dev); script kiểm đăng ký phải kiểm cả hai.

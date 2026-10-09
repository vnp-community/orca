# Tasks — Bug nghiệm thu e2e (v1)

Checklist triển khai cho [solutions](../solutions/README.md). Mỗi task đủ ngữ cảnh để thực hiện độc lập: file, phụ thuộc, tiêu chí hoàn thành và cách kiểm thử.

Ký hiệu: `[x]` đã làm (kể cả **chưa commit**), `[~]` làm một phần, `[ ]` chưa làm, `[-]` không áp dụng hoặc cố ý không làm. Ký hiệu này dùng cả cho từng mục "Việc cần làm" và "Tiêu chí hoàn thành" trong mỗi file task; mục chưa xong có ghi chú lý do.

> **Trạng thái:** mọi thay đổi `[x]`/`[~]` đang nằm trong working tree, **chưa commit**. `gitnexus impact` và `detect_changes` chưa chạy được (không có công cụ trong phiên), nên chưa có blast radius cho PR.

## Kết quả thực thi (2026-10-09)

Chạy `python3 tests/stages/e2e/orca-flows/run_acceptance.py` trên stack Docker local, dev server là máy Mac, Claude thật qua phiên đăng nhập CLI (agent không có khoá API):

| Mốc | PASS | FAIL | BLOCKED | SKIP |
|---|---|---|---|---|
| Trước khi triển khai (báo cáo gốc) | 84 | 3 | 4 | 1 |
| Cuối đợt v1, trước khi sửa Plan | 85 | 2 | 4 | 1 |
| **Cuối vòng (có suite `notfound`, `plan`)** | **103** | **0** | **4** | **1** |

BLOCKED còn lại không phải lỗi sản phẩm: MCP-01/02 (`MCP_ENABLED` tắt), AIS-01/03 (đường "không có AI" không dựng được khi dev server có phiên Claude). SKIP: TSK-08 (tuỳ chọn, tốn lượt AI). Chi tiết lỗi phát hiện thêm và đã sửa ở [vòng v2](../../e2e-acceptance-v2/README.md).

Đã chạy xanh: `go test ./...` ở `api-gateway`, `request-service`, `project-service`, `task-service`, `workflow-service`; agent vitest cho `ai-complete*`; tích hợp Postgres và MySQL của hợp đồng Solution (có run loại `plan`). Lỗi có sẵn, không do thay đổi này: `common/tracing` (`TestTraceEventSpanProcessor_OnEnd_ErrorStatusPublishesFailLevel`) và hai test socket unix của infra-fleet (đường dẫn quá dài trên macOS). Chưa chạy: `buf lint/breaking` (không có `buf`), test tích hợp `project-service` Postgres (thiếu CLI `migrate`).

Chưa làm trong đợt này: TASK-023 (tuỳ chọn), 028, 029, 035; làm một phần: 016, 017, 020, 022, 026, 030. Mỗi file task có mục "Kết quả thực thi" nêu rõ phần đã và chưa làm.

## Thứ tự thực hiện

```
Giai đoạn 0  Chốt bản vá đang chờ:  001, 004, 007, 009, 011, 018, 021, 027   (commit + bổ sung test)
Giai đoạn 1  Lan can:               002, 003, 005, 019, 020, 022            (không đổi hành vi)
Giai đoạn 2  Sửa theo gốc:          006 → (007, 008, 009) → (010, 011) → 012
                                    013 → 014 → 015 → (016, 017)
Giai đoạn 3  Thông luồng AI:        033 → 034 → 024 → 035 → 026   (025 song song; 023 tuỳ chọn, P2)
Giai đoạn 4  Ổn định:               028, 029, 030, 031, 032
```

```
001 ─┬─ 002
     ├─ 003 ──► TASK-FE-E2E-001
     └─ 032 ──► TASK-FE-E2E-003
004 ──► 005
006 ─┬─ 007 ─┐
     ├─ 008 ─┤
     ├─ 009 ─┴─ 010
     ├─ 011 ──► 012
     └─ 023 (tuỳ chọn)
033 ─┬─ 034 ──► 024 ──► (FE-005)
     ├─ 025
     └─ 035 ──► 026
013 ──► 014 ──► 015 ──► 016
          └──► 017
018 ─┬─ 019 ──► 022
     └─ 020
021 ──► 022
026 ──► 028, 029
```

## Checklist

### SOL-E2E-001: Khoá hợp đồng tầng (RC-A)

- [x] [TASK-E2E-001](./TASK-E2E-001-commit-gateway-clarification-channels.md) P0 · Chốt và commit kênh gateway `clarification.*` · phụ thuộc: không
- [x] [TASK-E2E-002](./TASK-E2E-002-rpc-to-channel-parity-test.md) P0 · Test P1: mọi RPC công khai của `RequestService` có kênh gateway hoặc loại trừ có lý do · phụ thuộc: TASK-E2E-001
- [x] [TASK-E2E-003](./TASK-E2E-003-channel-inventory-golden-file.md) P1 · Xuất tệp vàng danh mục kênh để frontend so khớp (P2) · phụ thuộc: TASK-E2E-001
- [x] [TASK-E2E-004](./TASK-E2E-004-agent-fs-checkpath-tests.md) P1 · Hoàn tất `fs.checkPath` ở agent: test, capability, tài liệu · phụ thuộc: không
- [x] [TASK-E2E-005](./TASK-E2E-005-backend-to-agent-method-manifest-check.md) P1 · Test P3: method backend gọi agent phải tồn tại ở agent · phụ thuộc: TASK-E2E-004

### SOL-E2E-002: Truyền danh tính (RC-B)

- [x] [TASK-E2E-006](./TASK-E2E-006-grpcmw-client-identity-interceptor.md) P0 · Thêm `ClientIdentityInterceptor` vào `common/grpcmw` · phụ thuộc: không
- [x] [TASK-E2E-007](./TASK-E2E-007-adopt-interceptor-project-service.md) P0 · Áp dụng interceptor cho `project-service` và test lại hai sửa đổi cục bộ · phụ thuộc: TASK-E2E-006
- [x] [TASK-E2E-008](./TASK-E2E-008-adopt-interceptor-other-services-and-lint.md) P1 · Áp dụng cho workflow, task, request, infra-fleet, automation; thêm lint cấm dial thủ công · phụ thuộc: TASK-E2E-006, 007
- [x] [TASK-E2E-009](./TASK-E2E-009-detach-context-helper.md) P0 · `tenant.Detach` cho tác vụ nền và thay mẫu `context.Background()` · phụ thuộc: không
- [x] [TASK-E2E-010](./TASK-E2E-010-workflow-persist-started-by.md) P1 · Lưu `started_by` của execution và khôi phục khi service khởi động lại · phụ thuộc: TASK-E2E-009
- [x] [TASK-E2E-011](./TASK-E2E-011-task-creator-from-identity.md) P1 · `CreateTask` suy người tạo từ danh tính, không tin `creator_id` của client · phụ thuộc: TASK-E2E-006
- [x] [TASK-E2E-012](./TASK-E2E-012-task-owner-grant-backfill.md) P1 · Migration vá quyền `owner` cho task đã tạo trước bản sửa · phụ thuộc: TASK-E2E-011

### SOL-E2E-003: Truy cập project và quy ước lỗi (RC-C)

- [x] [TASK-E2E-013](./TASK-E2E-013-proto-resolve-project-access.md) P1 · Proto `ProjectService.ResolveProjectAccess` (nội bộ) · phụ thuộc: không
- [x] [TASK-E2E-014](./TASK-E2E-014-project-service-resolve-access.md) P1 · Cài `ResolveProjectAccess` và sửa phân loại lỗi của `GetProject` · phụ thuộc: TASK-E2E-013
- [x] [TASK-E2E-015](./TASK-E2E-015-request-service-project-access.md) P1 · `request-service` dùng `ResolveProjectAccess` cho vai trò và kiểm tra tồn tại khi tạo Request · phụ thuộc: TASK-E2E-014
- [~] [TASK-E2E-016](./TASK-E2E-016-orphan-request-report.md) P2 · Báo cáo và xử lý Request mồ côi · phụ thuộc: TASK-E2E-015
- [~] [TASK-E2E-017](./TASK-E2E-017-not-found-convention-and-contract-test.md) P2 · Quy ước "không có → NotFound" và test hợp đồng xuyên service · phụ thuộc: TASK-E2E-014

### SOL-E2E-004: Triển khai và sẵn sàng (RC-D)

- [x] [TASK-E2E-018](./TASK-E2E-018-compose-required-tokens.md) P0 · Chốt cấu hình compose cho `request-service` và làm token bắt buộc khi bật cờ · phụ thuộc: không
- [x] [TASK-E2E-019](./TASK-E2E-019-ensure-request-env-script.md) P1 · Script `ensure-request-env.sh` sinh token nội bộ · phụ thuộc: TASK-E2E-018
- [~] [TASK-E2E-020](./TASK-E2E-020-request-service-readyz-and-fail-closed-audit.md) P1 · `/readyz` phản ánh cấu hình thật; rà guard nội bộ ở các service · phụ thuộc: TASK-E2E-018
- [x] [TASK-E2E-021](./TASK-E2E-021-redeploy-local-script.md) P1 · Sau build không mất mount; script `redeploy-local.sh` cho vòng lặp phát triển · phụ thuộc: không
- [~] [TASK-E2E-022](./TASK-E2E-022-deploy-docs-sync.md) P2 · Cập nhật tài liệu triển khai · phụ thuộc: TASK-E2E-019, 021

### SOL-E2E-005: AI qua phiên đăng nhập CLI của dev server (RC-E)

- [ ] [TASK-E2E-023](./TASK-E2E-023-optional-server-side-key-resolution.md) P2 · Tuỳ chọn: phân giải khoá AI phía server (`resolvedApiKey`) · phụ thuộc: TASK-E2E-006, 033
- [x] [TASK-E2E-024](./TASK-E2E-024-ai-readiness-and-structured-errors.md) P0 · Kiểm tra sẵn sàng AI bằng khả năng thật và mã lỗi có cấu trúc · phụ thuộc: TASK-E2E-034
- [x] [TASK-E2E-025](./TASK-E2E-025-agent-ai-complete-tests.md) P1 · Test `ai.complete` của agent: thứ tự khoá, không ghi log khoá, URL cơ sở cho kiểm thử · phụ thuộc: không
- [~] [TASK-E2E-026](./TASK-E2E-026-ai-stub-ci-wiring.md) P1 · Nối stub AI vào CI và mở khoá test BLOCKED · phụ thuộc: TASK-E2E-033, 035
- [x] [TASK-E2E-033](./TASK-E2E-033-agent-ai-complete-cli-session.md) P0 · Agent: `ai.complete` qua phiên đăng nhập CLI (nguồn mặc định) · phụ thuộc: không
- [x] [TASK-E2E-034](./TASK-E2E-034-agent-ai-sources-capability-and-killswitch.md) P0 · Agent: công bố `ai.sources`, thăm dò đăng nhập, công tắc tắt · phụ thuộc: TASK-E2E-033
- [ ] [TASK-E2E-035](./TASK-E2E-035-claude-cli-stub-for-ci.md) P1 · Stub `claude` cho CI và kiểm thử · phụ thuộc: TASK-E2E-033

### SOL-E2E-006: Kiểm thử, tài liệu, chỉnh vận hành (RC-F)

- [x] [TASK-E2E-027](./TASK-E2E-027-commit-t1-e2e-fixes.md) P1 · Commit sửa bộ e2e Go (T1) · phụ thuộc: không
- [ ] [TASK-E2E-028](./TASK-E2E-028-write-skipped-t1-stages.md) P2 · Viết các stage e2e Go đang skip (Solution, Plan, CommitPlan, ReportTaskOutcome) · phụ thuộc: TASK-E2E-026
- [ ] [TASK-E2E-029](./TASK-E2E-029-ci-orca-flows.md) P2 · Đưa `orca-flows` vào CI (smoke hằng đêm, `--strict` định kỳ) · phụ thuộc: TASK-E2E-026
- [~] [TASK-E2E-030](./TASK-E2E-030-docs-sync-request-guides.md) P2 · Đồng bộ tài liệu với mã · phụ thuộc: TASK-E2E-001, 008
- [x] [TASK-E2E-031](./TASK-E2E-031-rate-limit-configurable.md) P2 · Giới hạn ghi cấu hình được, có `retryAfterMs` và metric · phụ thuộc: không
- [x] [TASK-E2E-032](./TASK-E2E-032-clarification-value-type.md) P2 · Trường `value_type` cho câu hỏi Clarification · phụ thuộc: TASK-E2E-001

## Định nghĩa "xong" cho cả đợt

- [~] `run_acceptance.py` cho **0 FAIL** (đạt: 103 PASS); còn 4 BLOCKED (MCP tắt, hai đường lỗi AI). Chưa có chế độ `--ai-stub` (TASK-035).
- [~] `go test ./...` xanh cho `api-gateway`, `task-service`, `workflow-service`, `project-service`, `request-service`; `common` còn 1 test lỗi có sẵn (`common/tracing`). Bộ e2e Go `-tags e2e` xanh (Postgres).
- [~] Ba kiểm tra tĩnh P1/P2/P3 và lint dial (TASK 002, 003, 005, 008) đã viết và chạy tay; **chưa nằm trong workflow CI**. P1/P2 là test Go nên chạy cùng `go test`; P3 và lint dial là script.
- [ ] `buf lint` và `buf breaking` xanh (chưa chạy: không có `buf`; proto chỉ thêm trường/RPC).
- [ ] Mọi PR có kết quả `gitnexus impact` và `detect_changes` (G10).
- [x] Không có prompt, khoá hay nội dung xác thực trong log/argv (TASK 033: prompt qua stdin, stderr bỏ, env whitelist; TASK 025: test không log khoá).
- [x] Kiểm với tài khoản Claude thật trên dev server: phân loại (RQL-09), Solution (AIS-02), Plan (AIS-04, PLN-01..08) đều PASS.
- [ ] Phần frontend tương ứng xong ([tasks](../../../../frontend/bugs/e2e-acceptance-v1/tasks/README.md)).
